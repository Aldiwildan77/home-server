package main

import (
	"context"
	"encoding/json"
	"net"

	zlog "github.com/rs/zerolog/log"
	"golang.org/x/sys/unix"
)

const (
	udpReadBufferSize = 8192
	defaultWakeTTL    = 16
)

// udpMessage is the only wire format between nodes.
//
//   - "hello": a discovery beacon, broadcast to the local subnet. Carries
//     no payload -- NoteHop (called for every verified message) is what
//     actually adds the sender as a neighbor. Any node sharing the same
//     psk on the same broadcast domain is discovered this way, no peers
//     entry needed for same-subnet nodes.
//   - "state": a node's own gossiped contribution (its devices, its
//     identity, its observed edges). Flooded and deduped by MsgID so it
//     reaches the whole mesh exactly once per node per relay, even with
//     cycles.
//   - "wake": routed hop-by-hop toward whichever node owns MAC. TTL
//     guards against a stale route looping forever.
//   - "allow": an allow-list change, flooded the same way as "state".
//
// Hop is always the immediate signer's own id, stamped by Sign -- so
// adjacency and signature verification both key off whoever actually
// transmitted this specific packet, not the ultimate origin of its
// content. Timestamp+Sig authenticate every other field with a key
// derived from Hop and the mesh's shared psk (see Node.Sign/Verify).
type udpMessage struct {
	Type      string     `json:"type"`
	MsgID     string     `json:"msg_id,omitempty"`
	Hop       string     `json:"hop,omitempty"`
	Devices   Devices    `json:"devices,omitempty"`
	Nodes     []NodeInfo `json:"nodes,omitempty"`
	Edges     []Edge     `json:"edges,omitempty"`
	MAC       string     `json:"mac,omitempty"`
	Allow     bool       `json:"allow,omitempty"`
	TTL       int        `json:"ttl,omitempty"`
	Timestamp int64      `json:"ts,omitempty"`
	Sig       string     `json:"sig,omitempty"`
}

// listenUDP opens this node's one UDP socket, reused for every outgoing
// send too so a peer always sees this node arrive from the same fixed
// address, not a fresh ephemeral port each time. SO_BROADCAST is enabled
// on it so the same socket can also send the LAN discovery beacon.
func listenUDP(addr string, node *Node, inv Inventory) (*net.UDPConn, error) {
	laddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return nil, err
	}

	if err := enableBroadcast(conn); err != nil {
		conn.Close()
		return nil, err
	}

	node.AttachConn(conn)

	go serveUDP(conn, node, inv)

	return conn, nil
}

func enableBroadcast(conn *net.UDPConn) error {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return err
	}

	var sockErr error
	if err := rawConn.Control(func(fd uintptr) {
		sockErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_BROADCAST, 1)
	}); err != nil {
		return err
	}
	return sockErr
}

func serveUDP(conn *net.UDPConn, node *Node, inv Inventory) {
	buf := make([]byte, udpReadBufferSize)

	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}

		var msg udpMessage
		if err := json.Unmarshal(buf[:n], &msg); err != nil {
			zlog.Warn().Err(err).Str("from", from.String()).Msg("dropped malformed udp packet")
			continue
		}

		if !node.Verify(msg) {
			zlog.Warn().Str("from", from.String()).Str("type", msg.Type).Msg("dropped udp packet: bad signature or stale timestamp")
			continue
		}

		if msg.Hop == node.id {
			// our own broadcast reflected back to us (common for broadcast
			// sockets) -- not a real neighbor, ignore entirely
			continue
		}

		node.NoteHop(from, msg.Hop)

		switch msg.Type {
		case "hello":
			// nothing else to do -- NoteHop above already added the sender
		case "state":
			handleState(conn, node, inv, from, msg)
		case "wake":
			if err := node.Wake(context.Background(), msg.MAC, msg.TTL); err != nil {
				zlog.Err(err).Str("mac", msg.MAC).Msg("failed to route wake")
			}
		case "allow":
			handleAllow(conn, node, inv, from, msg)
		default:
			zlog.Warn().Str("type", msg.Type).Str("from", from.String()).Msg("unknown udp message type")
		}
	}
}

func handleState(conn *net.UDPConn, node *Node, inv Inventory, from *net.UDPAddr, msg udpMessage) {
	if len(msg.Nodes) == 0 {
		return
	}

	origin := msg.Nodes[0]

	isNew := node.Ingest(msg.MsgID, msg.Devices, origin, msg.Edges, msg.Hop)
	if !isNew {
		return
	}

	if inv != nil {
		if err := inv.Upsert(context.Background(), msg.Devices); err != nil {
			zlog.Err(err).Str("origin", origin.ID).Msg("failed to update inventory from gossip")
		}
	}

	relay(conn, node, from, msg)
}

// handleAllow persists an allow-list change to this node's own inventory
// (if it has one) and floods it onward. inv may be nil (no local
// inventory to update, just relay).
func handleAllow(conn *net.UDPConn, node *Node, inv Inventory, from *net.UDPAddr, msg udpMessage) {
	alreadySeen := node.Seen(msg.MsgID)
	if alreadySeen {
		return
	}

	applyAllowLocally(context.Background(), inv, msg.MAC, msg.Allow)
	relay(conn, node, from, msg)
}

func applyAllowLocally(ctx context.Context, inv Inventory, mac string, allow bool) {
	if inv == nil {
		return
	}
	if err := inv.SetAllowed(ctx, mac, allow); err != nil {
		zlog.Err(err).Str("mac", mac).Msg("failed to sync allow-list")
	}
}

// relay forwards msg to every neighbor but the one it just arrived from.
// Sign re-stamps Hop to this node's own id and re-signs with this node's
// own key, so downstream peers verify against the last hop, not the
// original sender.
func relay(conn *net.UDPConn, node *Node, from *net.UDPAddr, msg udpMessage) {
	node.Sign(&msg)

	for _, addr := range node.Neighbors(from) {
		if err := sendUDP(conn, addr, msg); err != nil {
			zlog.Err(err).Str("to", addr.String()).Msg("failed to relay message")
		}
	}
}

// broadcastHello sends this node's discovery beacon to the local
// subnet's broadcast address on its own port -- any node sharing psk
// listening there picks it up via NoteHop and becomes a neighbor, no
// peers entry required.
func broadcastHello(conn *net.UDPConn, node *Node) {
	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return
	}

	dst := &net.UDPAddr{IP: net.IPv4bcast, Port: localAddr.Port}

	msg := udpMessage{Type: "hello"}
	node.Sign(&msg)

	if err := sendUDP(conn, dst, msg); err != nil {
		zlog.Err(err).Msg("failed to broadcast discovery beacon")
	}
}

// broadcastAllow starts a fresh flood of an allow-list change from this
// node (as opposed to relaying one that arrived from elsewhere).
func broadcastAllow(conn *net.UDPConn, node *Node, mac string, allow bool) {
	msg := udpMessage{Type: "allow", MsgID: node.NextMsgID(), MAC: mac, Allow: allow}
	node.Sign(&msg)

	for _, addr := range node.Neighbors(nil) {
		if err := sendUDP(conn, addr, msg); err != nil {
			zlog.Err(err).Str("to", addr.String()).Msg("failed to broadcast allow-list change")
		}
	}
}

// broadcastState starts a fresh flood of this node's own contribution.
func broadcastState(conn *net.UDPConn, node *Node) {
	devices, info, edges := node.Contribution()
	msg := udpMessage{
		Type:    "state",
		MsgID:   node.NextMsgID(),
		Devices: devices,
		Nodes:   []NodeInfo{info},
		Edges:   edges,
	}
	node.Sign(&msg)

	for _, addr := range node.Neighbors(nil) {
		if err := sendUDP(conn, addr, msg); err != nil {
			zlog.Err(err).Str("to", addr.String()).Msg("failed to gossip state")
		}
	}
}

func sendWakeUDP(conn *net.UDPConn, node *Node, addr *net.UDPAddr, mac string, ttl int) error {
	msg := udpMessage{Type: "wake", MAC: mac, TTL: ttl}
	node.Sign(&msg)
	return sendUDP(conn, addr, msg)
}

func sendUDP(conn *net.UDPConn, addr *net.UDPAddr, msg udpMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = conn.WriteToUDP(body, addr)
	return err
}
