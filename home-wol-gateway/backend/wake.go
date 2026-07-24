package main

import (
	"bytes"
	"context"
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

const magicPacketRepeat = 16

// Waker broadcasts a magic packet onto whatever subnet this node sits in.
// Only the node that actually owns a MAC calls this; every other node in
// the mesh just routes the wake request toward it.
type Waker interface {
	Send(ctx context.Context, mac net.HardwareAddr) error
}

type localWaker struct {
	broadcastAddr string
	port          int
}

func NewLocalWaker(broadcastAddr string, port int) Waker {
	return &localWaker{
		broadcastAddr: broadcastAddr,
		port:          port,
	}
}

func (w *localWaker) Send(ctx context.Context, mac net.HardwareAddr) error {
	packet, err := buildMagicPacket(mac)
	if err != nil {
		return err
	}

	lc := net.ListenConfig{}

	conn, err := lc.ListenPacket(ctx, "udp4", ":0")
	if err != nil {
		return err
	}
	defer conn.Close()

	udpConn, ok := conn.(*net.UDPConn)
	if !ok {
		return fmt.Errorf("unexpected packet conn type: %T", conn)
	}

	rawConn, err := udpConn.SyscallConn()
	if err != nil {
		return err
	}

	var sockErr error
	if err := rawConn.Control(func(fd uintptr) {
		sockErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_BROADCAST, 1)
	}); err != nil {
		return err
	}
	if sockErr != nil {
		return sockErr
	}

	dst := &net.UDPAddr{IP: net.ParseIP(w.broadcastAddr), Port: w.port}

	_, err = udpConn.WriteTo(packet, dst)
	return err
}

func buildMagicPacket(mac net.HardwareAddr) ([]byte, error) {
	if len(mac) != 6 {
		return nil, fmt.Errorf("invalid mac address length: %d", len(mac))
	}

	var buf bytes.Buffer
	buf.Write(bytes.Repeat([]byte{0xFF}, 6))
	for range magicPacketRepeat {
		buf.Write(mac)
	}

	return buf.Bytes(), nil
}
