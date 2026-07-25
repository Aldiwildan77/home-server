package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync/atomic"

	zlog "github.com/rs/zerolog/log"
)

// Server wraps http.Server to reject new requests with 503 while a
// graceful shutdown is in flight, letting in-flight requests finish.
type Server struct {
	*http.Server
	shuttingDown atomic.Int32
}

// NewServer builds the HTTP API any node in the mesh can opt into: the
// inventory and topology it currently knows about, the WoL allow-list,
// and the entry point for waking a device wherever in the mesh it is.
// apiToken is required -- every request under these routes must carry it
// as a bearer token, since this API is otherwise unauthenticated on the
// LAN. The embedded frontend at "/" is deliberately NOT behind that
// token -- the browser has no way to attach it on first page load, and
// the compiled JS/HTML isn't sensitive; only the API calls it makes are.
func NewServer(node *Node, conn *net.UDPConn, inv Inventory, apiToken string) *Server {
	mux := http.NewServeMux()

	auth := func(h http.HandlerFunc) http.Handler {
		return requireBearerToken(apiToken, h)
	}

	mux.Handle("GET /healthz", auth(healthzHandler()))
	mux.Handle("GET /inventory", auth(inventoryHandler(inv)))
	mux.Handle("GET /topology", auth(topologyHandler(node)))
	mux.Handle("POST /devices/{mac}/allow", auth(allowHandler(node, conn, inv)))
	mux.Handle("POST /devices/{mac}/alias", auth(aliasHandler(inv)))
	mux.Handle("POST /wake", auth(wakeHandler(node, inv)))
	mux.Handle("/", http.FileServerFS(webFS()))

	s := &Server{}
	s.Server = &http.Server{
		Handler: withCORS(s.rejectWhileShuttingDown(mux)),
	}

	return s
}

// requireBearerToken sits after withCORS so CORS preflight (OPTIONS,
// which browsers never attach credentials to) is still handled -- every
// other request must present Authorization: Bearer <apiToken>.
func requireBearerToken(apiToken string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "

		got := r.Header.Get("Authorization")
		if !strings.HasPrefix(got, prefix) || subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(got, prefix)), []byte(apiToken)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) rejectWhileShuttingDown(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.shuttingDown.Load() == 1 {
			http.Error(w, "server is shutting down", http.StatusServiceUnavailable)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.shuttingDown.Store(1)
	return s.Server.Shutdown(ctx)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func healthzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func inventoryHandler(inv Inventory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		devices, err := inv.List(r.Context())
		if err != nil {
			zlog.Err(err).Str("action", "LIST_INVENTORY").Msg("failed to list inventory")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, devices)
	}
}

func topologyHandler(node *Node) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, nodes, edges := node.Snapshot()
		writeJSON(w, http.StatusOK, map[string]any{"nodes": nodes, "edges": edges})
	}
}

// allowHandler sets the allow flag locally, then floods it to the whole
// mesh so every node's own inventory stays in sync without a shared
// database.
func allowHandler(node *Node, conn *net.UDPConn, inv Inventory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mac := r.PathValue("mac")

		var req struct {
			Allow bool `json:"allow"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := inv.SetAllowed(r.Context(), mac, req.Allow); err != nil {
			zlog.Err(err).Str("action", "SET_ALLOWED").Str("mac", mac).Msg("failed to update allow-list")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		broadcastAllow(conn, node, mac, req.Allow)

		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	}
}

// aliasHandler sets a friendly display name for a device. It's local
// to this node's own inventory view -- unlike the allow flag, it isn't
// flooded across the mesh, since it's cosmetic rather than something
// that affects wake authorization or routing.
func aliasHandler(inv Inventory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mac := r.PathValue("mac")

		var req struct {
			Alias string `json:"alias"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := inv.SetAlias(r.Context(), mac, req.Alias); err != nil {
			zlog.Err(err).Str("action", "SET_ALIAS").Str("mac", mac).Msg("failed to update alias")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	}
}

func wakeHandler(node *Node, inv Inventory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req WakeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		allowed, err := inv.IsAllowed(r.Context(), req.MAC)
		if err != nil {
			zlog.Err(err).Str("action", "CHECK_ALLOWED").Str("mac", req.MAC).Msg("failed to check allow-list")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !allowed {
			http.Error(w, "device is not allowed to wake", http.StatusForbidden)
			return
		}

		if err := node.Wake(r.Context(), req.MAC, defaultWakeTTL); err != nil {
			zlog.Err(err).Str("action", "WAKE").Str("mac", req.MAC).Msg("failed to route wake")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		zlog.Info().Str("mac", req.MAC).Msg("wake routed")
		writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
