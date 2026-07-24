package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	zlog "github.com/rs/zerolog/log"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := LoadConfig(DefaultConfigPath())
	if err != nil {
		zlog.Err(err).Str("action", "LOAD_CONFIG").Msg("failed to load config")
		return
	}

	zlog.Debug().Any("config", cfg).Msg("config loaded")

	httpEnabled := cfg.Node.ListenHTTPAddr != ""

	if cfg.Security.PSK == "" {
		zlog.Error().Msg("security.psk is required -- refusing to start unauthenticated; every node in the mesh must share the same value")
		return
	}
	if httpEnabled && cfg.Security.APIToken == "" {
		zlog.Error().Msg("security.api_token is required when listen_http_addr is set -- refusing to expose an unauthenticated HTTP API")
		return
	}

	waker := NewLocalWaker(cfg.Wake.BroadcastAddr, cfg.Wake.Port)
	node := NewNode(cfg.Node.ID, cfg.Node.AdvertiseHTTPAddr, cfg.Security.PSK, waker, cfg.Node.ReportInterval)

	// HTTP + a local inventory are opt-in per node, independent of its
	// place in the mesh. Opened before the UDP listener so incoming
	// gossip can update it as soon as packets start arriving.
	var inv Inventory
	var srv *Server

	if httpEnabled {
		inv, err = NewInventory(cfg.DB.Path)
		if err != nil {
			zlog.Err(err).Str("action", "OPEN_DB").Msg("failed to open inventory db")
			return
		}
	}

	udpConn, err := listenUDP(cfg.Node.ListenUDPAddr, node, inv)
	if err != nil {
		zlog.Err(err).Str("action", "LISTEN_UDP").Msg("failed to start udp listener")
		return
	}
	defer udpConn.Close()

	for _, peer := range cfg.Node.Peers {
		addr, err := net.ResolveUDPAddr("udp4", peer)
		if err != nil {
			zlog.Err(err).Str("action", "RESOLVE_PEER").Str("peer", peer).Msg("failed to resolve peer address")
			continue
		}
		node.AddPeer(addr)
	}

	if httpEnabled {
		srv = NewServer(node, udpConn, inv, cfg.Security.APIToken)
		srv.Addr = cfg.Node.ListenHTTPAddr

		go func() {
			zlog.Info().Str("addr", srv.Addr).Msg("http api starting")
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				zlog.Err(err).Msg("http server error")
			}
		}()
	}

	discoverer := NewDiscoverer(cfg.Discovery.Command, cfg.Discovery.Args, cfg.Discovery.Subnet)

	tick := func() {
		devices, err := discoverer.Scan(ctx)
		if err != nil {
			zlog.Err(err).Str("action", "DISCOVER").Msg("failed to scan local devices")
		} else {
			for i := range devices {
				devices[i].NodeID = cfg.Node.ID
			}
			node.SetMine(devices)
		}

		if httpEnabled {
			snapshotDevices, _, _ := node.Snapshot()
			if err := inv.Upsert(ctx, snapshotDevices); err != nil {
				zlog.Err(err).Str("action", "UPSERT_INVENTORY").Msg("failed to update inventory")
			}
		}

		broadcastState(udpConn, node)
		broadcastHello(udpConn, node)
	}

	tick()

	ticker := time.NewTicker(cfg.Node.ReportInterval)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				tick()
			}
		}
	}()

	<-ctx.Done()
	zlog.Info().Msg("shutdown signal received")

	if srv != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			zlog.Err(err).Msg("http server shutdown error")
		}
	}

	zlog.Info().Msg("gracefully stopped")
}
