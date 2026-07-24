package main

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/goccy/go-yaml"
)

const DefaultConfigFileName string = "config.yaml"

var DefaultConfigPath = func() string {
	rootDir, _ := os.Getwd()
	return filepath.Join(rootDir, DefaultConfigFileName)
}

type Config struct {
	Node      NodeConfig      `yaml:"node"`
	Discovery DiscoveryConfig `yaml:"discovery"`
	Wake      WakeConfig      `yaml:"wake"`
	DB        DBConfig        `yaml:"db"`
	Security  SecurityConfig  `yaml:"security"`
}

// SecurityConfig has no safe default on purpose. PSK must be identical
// across every node in the mesh -- it's the HMAC key used to sign and
// verify every UDP message, so a node without it can't join the mesh and
// an attacker without it can't forge state/wake/allow messages. APIToken
// gates the HTTP API the same way, independent of the mesh trust
// boundary. Both are required; the process refuses to start without them.
type SecurityConfig struct {
	PSK      string `yaml:"psk"`
	APIToken string `yaml:"api_token"`
}

// NodeConfig describes this instance's place in the mesh. Every node runs
// the same binary and is a peer, not a fixed role: Peers lists other
// nodes' UDP addresses to gossip with directly (edges are learned
// automatically once the other side hears back, so an edge only needs
// to be configured on one side). ListenHTTPAddr is independent of that:
// any node can expose its current view of the mesh over HTTP for a
// frontend, whether or not it has any peers configured.
type NodeConfig struct {
	ID                string        `yaml:"id"`
	ListenUDPAddr     string        `yaml:"listen_udp_addr"`
	ListenHTTPAddr    string        `yaml:"listen_http_addr"`
	AdvertiseHTTPAddr string        `yaml:"advertise_http_addr"`
	Peers             []string      `yaml:"peers"`
	ReportInterval    time.Duration `yaml:"report_interval"`
}

// DiscoveryConfig.Subnet is optional. Left empty, discovery is passive:
// only devices already in the OS neighbor table are seen. Set it to this
// node's CIDR (e.g. "192.168.1.0/24") to actively ping-sweep first, so
// idle devices show up too.
type DiscoveryConfig struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
	Subnet  string   `yaml:"subnet"`
}

// WakeConfig is only exercised when this node ends up owning the MAC
// being woken, in which case it broadcasts onto its own local subnet.
type WakeConfig struct {
	BroadcastAddr string `yaml:"broadcast_addr"`
	Port          int    `yaml:"port"`
}

// DBConfig is only used when ListenHTTPAddr is set, to persist this
// node's own view of the mesh's inventory and WoL allow-list across restarts.
type DBConfig struct {
	Path string `yaml:"path"`
}

func LoadConfig(path string) (cfg *Config, err error) {
	cfg = &Config{
		Node: NodeConfig{
			ID:             "local",
			ListenUDPAddr:  ":9090",
			ListenHTTPAddr: "",
			ReportInterval: 10 * time.Second,
		},
		Discovery: DiscoveryConfig{
			Command: "ip",
			Args:    []string{"neigh"},
		},
		Wake: WakeConfig{
			BroadcastAddr: "255.255.255.255",
			Port:          9,
		},
		DB: DBConfig{
			Path: "./data/inventory.db",
		},
	}

	configFile, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}

		return
	}

	err = yaml.Unmarshal(configFile, cfg)
	return
}
