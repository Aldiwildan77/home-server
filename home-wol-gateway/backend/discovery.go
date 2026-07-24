package main

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"

	zlog "github.com/rs/zerolog/log"
)

const (
	minNeighParts      = 4
	maxConcurrentPings = 64
)

type Discoverer interface {
	Scan(ctx context.Context) (Devices, error)
}

type discoverer struct {
	command string
	args    []string
	subnet  string
}

// NewDiscoverer reads whatever's already in the OS neighbor table via
// command/args. If subnet is set, it first ping-sweeps every host in
// that CIDR to populate the table, so idle devices (nothing recently
// talked to them) show up too instead of only ones with live traffic.
func NewDiscoverer(command string, args []string, subnet string) Discoverer {
	return &discoverer{
		command: command,
		args:    args,
		subnet:  subnet,
	}
}

func (d *discoverer) Scan(ctx context.Context) (Devices, error) {
	if d.subnet != "" {
		sweepSubnet(ctx, d.subnet)
	}

	output, err := exec.CommandContext(ctx, d.command, d.args...).CombinedOutput()
	if err != nil {
		zlog.Error().
			Err(err).
			Str("command", d.command).
			Strs("args", d.args).
			Str("output", string(output)).
			Msg("command failed")
		return nil, err
	}

	devices := make(Devices, 0)

	for line := range strings.SplitSeq(string(output), "\n") {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}

		dev, ok := parseNeighLine(l)
		if !ok {
			continue
		}

		devices = append(devices, dev)
	}

	return devices, nil
}

func parseNeighLine(line string) (Device, bool) {
	// 192.168.50.10 dev eth0 lladdr aa:bb:cc:dd:ee:ff REACHABLE
	parts := strings.Fields(line)
	if len(parts) < minNeighParts {
		return Device{}, false
	}

	macIdx := -1
	for i, p := range parts {
		if p == "lladdr" && i+1 < len(parts) {
			macIdx = i + 1
			break
		}
	}
	if macIdx == -1 {
		return Device{}, false
	}

	state := parts[len(parts)-1]

	return Device{
		IP:     parts[0],
		MAC:    parts[macIdx],
		Online: state != "FAILED" && state != "INCOMPLETE",
	}, true
}

// sweepSubnet pings every host address in cidr. Replies aren't read —
// the point is purely the side effect of populating the OS neighbor
// table, which Scan then reads via the normal ip neigh path.
func sweepSubnet(ctx context.Context, cidr string) {
	ips, err := hostIPs(cidr)
	if err != nil {
		zlog.Warn().Err(err).Str("subnet", cidr).Msg("skipping subnet sweep: invalid cidr")
		return
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentPings)

	for _, target := range ips {
		wg.Add(1)
		sem <- struct{}{}

		go func(target string) {
			defer wg.Done()
			defer func() { <-sem }()
			_ = exec.CommandContext(ctx, "ping", "-c", "1", "-W", "1", target).Run()
		}(target)
	}

	wg.Wait()
}

// hostIPs lists every usable host address in cidr (network and
// broadcast addresses excluded when the range has more than two).
func hostIPs(cidr string) ([]string, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	ip4 := ip.Mask(ipNet.Mask).To4()
	if ip4 == nil {
		return nil, fmt.Errorf("only ipv4 subnets are supported: %s", cidr)
	}

	var ips []string
	for cur := cloneIP(ip4); ipNet.Contains(cur); incIP(cur) {
		ips = append(ips, cur.String())
	}

	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}

	return ips, nil
}

func cloneIP(ip net.IP) net.IP {
	c := make(net.IP, len(ip))
	copy(c, ip)
	return c
}

func incIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}
