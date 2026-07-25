package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// staleAfter is how many report intervals a known fact (device, node, or
// edge) can go without being refreshed before it's dropped -- covers a
// peer going away without anyone flooding a retraction for it.
const staleAfter = 3

type neighborEntry struct {
	addr   *net.UDPAddr
	nodeID string // resolved once we've directly heard from this address
	seenAt time.Time
}

type deviceRecord struct {
	device Device
	viaHop string // which direct neighbor this was last heard through
	seenAt time.Time
}

type nodeRecord struct {
	info   NodeInfo
	seenAt time.Time
}

type edgeRecord struct {
	edge   Edge
	seenAt time.Time
}

// Node is one peer in the mesh. Every tier (root, gateway, agent) is the
// same Node type -- there's no hierarchy in the protocol, just whichever
// peers are configured or discovered. It gossips its own devices/identity
// and directly-observed edges to its neighbors, and relays anything new
// it hears onward (flood, deduped by message ID so cycles terminate).
type Node struct {
	id       string
	httpAddr string
	psk      string // the shared "vpc key" -- lets any node derive any other node's signing key from its id alone
	nodeKey  string // this node's own derived signing key: HMAC(psk, id)
	waker    Waker
	ttl      time.Duration
	conn     *net.UDPConn
	msgSeq   atomic.Uint64

	mu          sync.Mutex
	mine        Devices
	staticPeers []*net.UDPAddr
	neighbors   map[string]*neighborEntry
	devices     map[string]*deviceRecord
	nodes       map[string]*nodeRecord
	edges       map[string]*edgeRecord
	seenMsgs    map[string]time.Time
}

func NewNode(id, httpAddr, psk string, waker Waker, reportInterval time.Duration) *Node {
	n := &Node{
		id:        id,
		httpAddr:  httpAddr,
		psk:       psk,
		nodeKey:   deriveNodeKey(psk, id),
		waker:     waker,
		ttl:       reportInterval * staleAfter,
		neighbors: make(map[string]*neighborEntry),
		devices:   make(map[string]*deviceRecord),
		nodes:     make(map[string]*nodeRecord),
		edges:     make(map[string]*edgeRecord),
		seenMsgs:  make(map[string]time.Time),
	}
	n.nodes[id] = &nodeRecord{info: NodeInfo{ID: id, HTTPAddr: httpAddr}, seenAt: time.Now()}
	return n
}

// deriveNodeKey is every node's own signing key: HMAC-SHA256 keyed on the
// mesh-wide psk, over that node's own id. Anyone holding psk can derive
// it for any node id (ids travel in cleartext in every message), so this
// doesn't add protection against an attacker who already has psk -- what
// it buys is a hook for future per-node revocation without rotating psk
// for the whole mesh.
func deriveNodeKey(psk, id string) string {
	mac := hmac.New(sha256.New, []byte(psk))
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil))
}

// replayWindowSeconds bounds how old a signed message can be before it's
// rejected -- keeps a captured, legitimately-signed packet from being
// replayed indefinitely.
const replayWindowSeconds = 60

// Sign stamps msg as having come from this node: Hop is always this
// node's own id (whoever last signed it, not necessarily who originated
// it), then signed with this node's own derived key.
func (n *Node) Sign(msg *udpMessage) {
	msg.Hop = n.id
	msg.Timestamp = time.Now().Unix()
	msg.Sig = ""
	msg.Sig = computeSig(*msg, n.nodeKey)
}

// Verify checks msg's signature and freshness. It re-derives the
// expected signer's key from msg.Hop (the claimed signer's id) and this
// node's own psk, rather than needing the signer's key handed to it
// directly. False on any mismatch -- unknown/wrong psk, tampered
// content, or a timestamp outside the replay window.
func (n *Node) Verify(msg udpMessage) bool {
	if n.psk == "" || msg.Hop == "" {
		return false
	}

	delta := time.Now().Unix() - msg.Timestamp
	if delta < 0 {
		delta = -delta
	}
	if delta > replayWindowSeconds {
		return false
	}

	senderKey := deriveNodeKey(n.psk, msg.Hop)

	sig := msg.Sig
	msg.Sig = ""

	return hmac.Equal([]byte(sig), []byte(computeSig(msg, senderKey)))
}

func computeSig(msg udpMessage, key string) string {
	body, err := json.Marshal(msg)
	if err != nil {
		return ""
	}

	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func (n *Node) AttachConn(conn *net.UDPConn) {
	n.conn = conn
}

func (n *Node) AddPeer(addr *net.UDPAddr) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.staticPeers = append(n.staticPeers, addr)
}

func (n *Node) NextMsgID() string {
	return fmt.Sprintf("%s-%d", n.id, n.msgSeq.Add(1))
}

// NoteHop records that a packet just arrived directly from hopID at addr,
// regardless of what the packet is about -- this is how adjacency is
// learned, independent of how far the message's content has traveled.
func (n *Node) NoteHop(addr *net.UDPAddr, hopID string) {
	if hopID == "" {
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	n.neighbors[addr.String()] = &neighborEntry{addr: addr, nodeID: hopID, seenAt: time.Now()}
	n.putEdgeLocked(n.id, hopID)
}

// Seen reports whether msgID has already been processed, marking it seen
// either way. Used to dedup flooded messages so relaying terminates even
// when the mesh has cycles.
func (n *Node) Seen(msgID string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Now()
	for id, at := range n.seenMsgs {
		if now.Sub(at) > 5*time.Minute {
			delete(n.seenMsgs, id)
		}
	}

	if _, ok := n.seenMsgs[msgID]; ok {
		return true
	}

	n.seenMsgs[msgID] = now
	return false
}

// Neighbors is every peer this node currently gossips with -- configured
// static peers plus anything recently heard from -- except addr, for
// flooding a message onward without needing to know the mesh's shape.
func (n *Node) Neighbors(except *net.UDPAddr) []*net.UDPAddr {
	n.mu.Lock()
	defer n.mu.Unlock()

	seen := make(map[string]bool)
	out := make([]*net.UDPAddr, 0, len(n.staticPeers)+len(n.neighbors))

	add := func(addr *net.UDPAddr) {
		if sameUDPAddr(addr, except) || seen[addr.String()] {
			return
		}
		seen[addr.String()] = true
		out = append(out, addr)
	}

	for _, addr := range n.staticPeers {
		add(addr)
	}

	now := time.Now()
	for key, ne := range n.neighbors {
		if now.Sub(ne.seenAt) > n.ttl {
			delete(n.neighbors, key)
			continue
		}
		add(ne.addr)
	}

	return out
}

func sameUDPAddr(a, b *net.UDPAddr) bool {
	if a == nil || b == nil {
		return false
	}
	return a.String() == b.String()
}

// SetMine merges the latest local scan into what this node knows about
// its own LAN. Devices are kept once learned even if a later scan
// doesn't see them -- that's the normal state for a WoL target (it's
// off, so of course ARP won't have it), and losing the record here
// would make it unwakeable exactly when it's needed.
func (n *Node) SetMine(devices Devices) {
	n.mu.Lock()
	defer n.mu.Unlock()

	seen := make(map[string]Device, len(devices))
	for _, d := range devices {
		seen[d.MAC] = d
	}

	merged := make(map[string]Device, len(n.mine)+len(devices))
	for _, d := range n.mine {
		if _, ok := seen[d.MAC]; !ok {
			d.Online = false
		}
		merged[d.MAC] = d
	}
	maps.Copy(merged, seen)

	now := time.Now()
	mine := make(Devices, 0, len(merged))
	for _, d := range merged {
		mine = append(mine, d)
		n.devices[d.MAC] = &deviceRecord{device: d, viaHop: n.id, seenAt: now}
	}
	n.mine = mine
}

// Contribution is what this node gossips about itself each tick: its own
// devices, its own identity, and the edges to whoever it's currently
// adjacent to. Everything else in the mesh's knowledge comes from
// relaying other nodes' contributions the same way.
func (n *Node) Contribution() (Devices, NodeInfo, []Edge) {
	n.mu.Lock()
	defer n.mu.Unlock()

	devices := make(Devices, len(n.mine))
	copy(devices, n.mine)

	info := NodeInfo{ID: n.id, HTTPAddr: n.httpAddr}

	edgeSet := make(map[string]Edge)
	for _, ne := range n.neighbors {
		if ne.nodeID == "" {
			continue
		}
		key, e := canonicalEdge(n.id, ne.nodeID)
		edgeSet[key] = e
	}

	edges := make([]Edge, 0, len(edgeSet))
	for _, e := range edgeSet {
		edges = append(edges, e)
	}

	return devices, info, edges
}

// Ingest merges another node's gossiped contribution into what this node
// knows. Returns false if msgID has already been processed (nothing to
// relay onward).
func (n *Node) Ingest(msgID string, devices Devices, node NodeInfo, edges []Edge, viaHop string) bool {
	if n.Seen(msgID) {
		return false
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Now()

	n.nodes[node.ID] = &nodeRecord{info: node, seenAt: now}

	for _, d := range devices {
		n.devices[d.MAC] = &deviceRecord{device: d, viaHop: viaHop, seenAt: now}
	}

	for _, e := range edges {
		key, canon := canonicalEdge(e.A, e.B)
		n.edges[key] = &edgeRecord{edge: canon, seenAt: now}
	}

	return true
}

func (n *Node) putEdgeLocked(a, b string) {
	key, e := canonicalEdge(a, b)
	n.edges[key] = &edgeRecord{edge: e, seenAt: time.Now()}
}

func canonicalEdge(a, b string) (string, Edge) {
	if a > b {
		a, b = b, a
	}
	return a + "|" + b, Edge{A: a, B: b}
}

// Snapshot is everything this node currently knows about the mesh --
// every device, node, and edge it's heard of, directly or relayed --
// pruned of anything not refreshed within the staleness window.
func (n *Node) Snapshot() (Devices, []NodeInfo, []Edge) {
	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Now()

	devices := make(Devices, 0, len(n.devices))
	for mac, rec := range n.devices {
		if now.Sub(rec.seenAt) > n.ttl {
			delete(n.devices, mac)
			continue
		}
		devices = append(devices, rec.device)
	}

	nodes := make([]NodeInfo, 0, len(n.nodes))
	for id, rec := range n.nodes {
		if id != n.id && now.Sub(rec.seenAt) > n.ttl {
			delete(n.nodes, id)
			continue
		}
		nodes = append(nodes, rec.info)
	}

	edges := make([]Edge, 0, len(n.edges))
	for key, rec := range n.edges {
		if now.Sub(rec.seenAt) > n.ttl {
			delete(n.edges, key)
			continue
		}
		edges = append(edges, rec.edge)
	}

	return devices, nodes, edges
}

// route finds the next hop for a MAC: isMine if it's ours to broadcast
// locally, otherwise the neighbor address last known to lead toward it.
func (n *Node) route(mac string) (hopAddr *net.UDPAddr, isMine bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	rec, ok := n.devices[mac]
	if !ok {
		return nil, false
	}
	if rec.viaHop == n.id {
		return nil, true
	}

	for _, ne := range n.neighbors {
		if ne.nodeID == rec.viaHop {
			return ne.addr, false
		}
	}

	return nil, false
}

func (n *Node) Wake(ctx context.Context, mac string, ttl int) error {
	addr, isMine := n.route(mac)

	if isMine {
		hw, err := net.ParseMAC(mac)
		if err != nil {
			return err
		}
		return n.waker.Send(ctx, hw)
	}

	if addr == nil {
		return fmt.Errorf("unknown mac: %s", mac)
	}

	if ttl <= 0 {
		return fmt.Errorf("wake ttl exceeded for mac: %s", mac)
	}

	return sendWakeUDP(n.conn, n, addr, mac, ttl-1)
}
