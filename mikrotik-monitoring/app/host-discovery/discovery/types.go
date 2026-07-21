package discovery

import (
	"errors"
	"net"
	"strconv"
	"strings"
)

const (
	DefaultHostPrefix = "default "
	ipIndex           = 2
	interfaceIndex    = 4
	maxParts          = 6
)

type Host struct {
	Interface string `json:"interface"`
	Gateway   net.IP `json:"gateway"`
}

type Hosts []Host

func (hs Hosts) ToMapWithInterfaceAsKey() HostMap {
	m := make(HostMap, 0)

	for _, h := range hs {
		_, exists := m[h.Interface]
		if !exists {
			m[h.Interface] = append(m[h.Interface], h)
			continue
		}

		m[h.Interface] = append(m[h.Interface], h)
	}

	return m
}

type HostMap map[string]Hosts

func (hm HostMap) ToTargetMap() TargetMap {
	t := make(TargetMap, 0)

	for hsKey, hsValue := range hm {
		_, exists := t[hsKey]
		if !exists {
			t[hsKey] = make(Targets, 0)
		}
		for _, h := range hsValue {
			t[hsKey] = append(t[hsKey], &Target{
				Targets: []string{h.Gateway.String()},
				Labels:  map[string]string{"interface": hsKey}, // for now only support interface key
			})
		}
	}

	return t
}

type Target struct {
	Targets []string `json:"targets"`
	Labels  any      `json:"labels"`
}

func (t *Target) Parse(line string) (bool, error) {
	// default via 192.168.3.1 dev wlan0 proto dhcp src 192.168.3.156 metric 600
	// default via 192.168.4.1 dev eth0 proto dhcp src 192.168.4.99 metric 600
	l := strings.TrimSpace(line)
	if l == "" {
		return false, nil
	}

	parts := strings.Split(l, " ")
	if len(parts) < maxParts {
		return false, errors.New("invalid parsed line: expected at least 6 parts, got " + strconv.Itoa(len(parts)))
	}

	t.Targets = []string{parts[ipIndex]}
	t.Labels = map[string]string{"interface": parts[interfaceIndex]}

	return true, nil
}

type Targets []*Target

type TargetMap map[string]Targets

func (tm TargetMap) ToTargets() Targets {
	t := make(Targets, 0)
	for _, ts := range tm {
		t = append(t, ts...)
	}
	return t
}
