package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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
	// db.path defaults next to the config file itself, not the process's
	// cwd -- same reasoning as -config existing at all: a relative path
	// silently means something different depending on where you happen to
	// launch this from (systemd's WorkingDirectory, a terminal, etc).
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
			Path: filepath.Join(filepath.Dir(path), "data", "inventory.db"),
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

const initConfigTemplate = `node:
  id: %s
  listen_udp_addr: ":9090"
  listen_http_addr: ""   # set to e.g. ":8080" to serve the HTTP API + frontend from this node
  advertise_http_addr: ""
  peers: []               # leave empty on the same subnet as another node; otherwise ["<ip>:9090"]
  report_interval: 10s

discovery:
  command: ip
  args: [neigh]
  subnet: ""              # e.g. "192.168.1.0/24" to actively ping-sweep

wake:
  broadcast_addr: "255.255.255.255"
  port: 9

db:
  path: "%s"   # only read if listen_http_addr is set

security:
  psk: "%s"
  api_token: "%s"
`

// WriteInitConfig writes a starter config to path with freshly generated
// secrets, refusing to clobber an existing file. nodeID lets -init pick a
// sensible default id instead of always writing "local". db.path is
// generated relative to path's own directory (see LoadConfig).
func WriteInitConfig(path, nodeID string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists -- not overwriting", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	psk, err := randomHex(32)
	if err != nil {
		return err
	}
	apiToken, err := randomHex(32)
	if err != nil {
		return err
	}

	dbPath := filepath.Join(filepath.Dir(path), "data", "inventory.db")
	content := fmt.Sprintf(initConfigTemplate, nodeID, dbPath, psk, apiToken)
	return os.WriteFile(path, []byte(content), 0600)
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
