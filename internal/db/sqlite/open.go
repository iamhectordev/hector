package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/doron-cohen/klee/xdg"
	_ "modernc.org/sqlite"
)

const (
	defaultMaxOpenConns = 4
	defaultMaxIdleConns = 4
	appName             = "hector"
	defaultDBName       = "hector.db"
)

// Open opens the app-owned SQLite database with project defaults.
func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	path := resolvePath(cfg)
	if path == "" {
		return nil, fmt.Errorf("db/sqlite: path is required")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("db/sqlite: resolve path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return nil, fmt.Errorf("db/sqlite: create parent directory: %w", err)
	}

	dsn := buildDSN(absPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db/sqlite: open: %w", err)
	}

	db.SetMaxOpenConns(defaultMaxOpenConns)
	db.SetMaxIdleConns(defaultMaxIdleConns)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("db/sqlite: ping: %w", err)
	}

	return db, nil
}

func resolvePath(cfg Config) string {
	path := strings.TrimSpace(cfg.Path)
	if path != "" {
		return path
	}
	return filepath.Join(xdg.New(appName).DataHome(), defaultDBName)
}

func buildDSN(path string) string {
	u := &url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(path),
	}

	q := u.Query()
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "foreign_keys(ON)")
	q.Add("_pragma", "synchronous(NORMAL)")
	u.RawQuery = q.Encode()

	return u.String()
}
