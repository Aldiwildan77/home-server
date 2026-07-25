package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// InventoryDevice is a Device plus its WoL allow flag and user-set alias.
type InventoryDevice struct {
	Device
	WoLAllowed bool   `json:"wol_allowed"`
	Alias      string `json:"alias,omitempty"`
}

// Inventory is a node's persisted view of the mesh: every device it
// currently knows about, plus which ones are allowed to be woken.
type Inventory interface {
	Upsert(ctx context.Context, devices Devices) error
	List(ctx context.Context) ([]InventoryDevice, error)
	SetAllowed(ctx context.Context, mac string, allowed bool) error
	IsAllowed(ctx context.Context, mac string) (bool, error)
	SetAlias(ctx context.Context, mac string, alias string) error
}

type sqliteInventory struct {
	db *sql.DB
}

func NewInventory(path string) (Inventory, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}

	// This node's report ticker and its UDP gossip handler both write here
	// concurrently from different goroutines. SQLite serializes writers
	// regardless, so capping the pool at one connection lets Go's own
	// database/sql queue requests instead of both hitting SQLITE_BUSY.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS devices (
			mac TEXT PRIMARY KEY,
			ip TEXT NOT NULL,
			hostname TEXT,
			online INTEGER NOT NULL,
			node_id TEXT NOT NULL DEFAULT '',
			wol_allowed INTEGER NOT NULL DEFAULT 0,
			alias TEXT NOT NULL DEFAULT '',
			updated_at INTEGER NOT NULL
		)
	`); err != nil {
		return nil, err
	}

	// Older DBs predate the alias column -- add it if missing. The bundled
	// SQLite doesn't support "ADD COLUMN IF NOT EXISTS", so check first.
	hasAlias, err := hasColumn(db, "devices", "alias")
	if err != nil {
		return nil, err
	}
	if !hasAlias {
		if _, err := db.Exec(`ALTER TABLE devices ADD COLUMN alias TEXT NOT NULL DEFAULT ''`); err != nil {
			return nil, err
		}
	}

	return &sqliteInventory{db: db}, nil
}

func (s *sqliteInventory) Upsert(ctx context.Context, devices Devices) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO devices (mac, ip, hostname, online, node_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(mac) DO UPDATE SET
			ip = excluded.ip,
			hostname = excluded.hostname,
			online = excluded.online,
			node_id = excluded.node_id,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, d := range devices {
		if _, err := stmt.ExecContext(ctx, d.MAC, d.IP, d.Hostname, boolToInt(d.Online), d.NodeID, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *sqliteInventory) List(ctx context.Context) ([]InventoryDevice, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT mac, ip, hostname, online, node_id, wol_allowed, alias FROM devices ORDER BY mac`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]InventoryDevice, 0)

	for rows.Next() {
		var d InventoryDevice
		var online, allowed int
		var hostname sql.NullString

		if err := rows.Scan(&d.MAC, &d.IP, &hostname, &online, &d.NodeID, &allowed, &d.Alias); err != nil {
			return nil, err
		}

		d.Hostname = hostname.String
		d.Online = online == 1
		d.WoLAllowed = allowed == 1
		list = append(list, d)
	}

	return list, rows.Err()
}

func (s *sqliteInventory) SetAllowed(ctx context.Context, mac string, allowed bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE devices SET wol_allowed = ? WHERE mac = ?`, boolToInt(allowed), mac)
	return err
}

func (s *sqliteInventory) SetAlias(ctx context.Context, mac string, alias string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE devices SET alias = ? WHERE mac = ?`, alias, mac)
	return err
}

func (s *sqliteInventory) IsAllowed(ctx context.Context, mac string) (bool, error) {
	var allowed int

	err := s.db.QueryRowContext(ctx, `SELECT wol_allowed FROM devices WHERE mac = ?`, mac).Scan(&allowed)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return allowed == 1, nil
}

func hasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var dflt sql.NullString
		var pk int

		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}

	return false, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
