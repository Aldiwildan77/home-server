package main

import (
	"reflect"
	"testing"
)

func TestHostIPs(t *testing.T) {
	tests := []struct {
		name string
		cidr string
		want []string
	}{
		{
			name: "/30 excludes network and broadcast",
			cidr: "10.0.0.0/30",
			want: []string{"10.0.0.1", "10.0.0.2"},
		},
		{
			name: "/31 has no distinct network/broadcast to exclude",
			cidr: "10.0.0.0/31",
			want: []string{"10.0.0.0", "10.0.0.1"},
		},
		{
			name: "/32 is a single host",
			cidr: "10.0.0.5/32",
			want: []string{"10.0.0.5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hostIPs(tt.cidr)
			if err != nil {
				t.Fatalf("hostIPs(%q) error: %v", tt.cidr, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("hostIPs(%q) = %v, want %v", tt.cidr, got, tt.want)
			}
		})
	}
}

func TestHostIPsCount(t *testing.T) {
	ips, err := hostIPs("192.168.1.0/24")
	if err != nil {
		t.Fatalf("hostIPs error: %v", err)
	}
	if len(ips) != 254 {
		t.Fatalf("expected 254 usable hosts in a /24, got %d", len(ips))
	}
	if ips[0] != "192.168.1.1" || ips[len(ips)-1] != "192.168.1.254" {
		t.Fatalf("unexpected range bounds: first=%s last=%s", ips[0], ips[len(ips)-1])
	}
}

func TestHostIPsInvalidCIDR(t *testing.T) {
	if _, err := hostIPs("not-a-cidr"); err == nil {
		t.Fatal("expected error for invalid cidr")
	}
}
