package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var (
	ErrDuplicateNamespace = errors.New("migrations: duplicate namespace")
	ErrInvalidNamespace   = errors.New("migrations: invalid namespace")
	ErrInvalidMigration   = errors.New("migrations: invalid migration")
	ErrDirtyMigration     = errors.New("migrations: dirty migration")
)

type Migration struct {
	Version int
	Name    string
	SQL     string
}

type MigrationSet struct {
	namespace string
	items     []Migration
}

func NewSet(namespace string, items ...Migration) MigrationSet {
	cloned := make([]Migration, len(items))
	copy(cloned, items)

	return MigrationSet{
		namespace: namespace,
		items:     cloned,
	}
}

func (s MigrationSet) Namespace() string {
	return s.namespace
}

func (s MigrationSet) Migrations() []Migration {
	cloned := make([]Migration, len(s.items))
	copy(cloned, s.items)
	return cloned
}

type Runner struct {
	db   *sql.DB
	sets map[string]MigrationSet
	seen []string
}

func New(db *sql.DB) *Runner {
	return &Runner{
		db:   db,
		sets: make(map[string]MigrationSet),
	}
}

func (r *Runner) Add(set MigrationSet) error {
	namespace := strings.TrimSpace(set.Namespace())
	if namespace == "" {
		return ErrInvalidNamespace
	}
	if _, exists := r.sets[namespace]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateNamespace, namespace)
	}

	migrations := set.Migrations()
	if err := validateMigrations(migrations); err != nil {
		return err
	}

	r.sets[namespace] = NewSet(namespace, migrations...)
	r.seen = append(r.seen, namespace)
	return nil
}

func (r *Runner) Run(ctx context.Context) error {
	if err := r.ensureSchema(ctx); err != nil {
		return err
	}

	for _, namespace := range r.seen {
		set := r.sets[namespace]
		migrations := set.Migrations()
		sort.Slice(migrations, func(i, j int) bool {
			return migrations[i].Version < migrations[j].Version
		})

		for _, migration := range migrations {
			dirty, exists, err := r.status(ctx, namespace, migration.Version)
			if err != nil {
				return err
			}
			if exists {
				if dirty {
					return fmt.Errorf("%w: namespace=%q version=%d", ErrDirtyMigration, namespace, migration.Version)
				}
				continue
			}

			if err := r.markDirty(ctx, namespace, migration); err != nil {
				return err
			}

			if _, err := r.db.ExecContext(ctx, migration.SQL); err != nil {
				return fmt.Errorf("migrations: apply namespace=%q version=%d name=%q: %w", namespace, migration.Version, migration.Name, err)
			}

			if err := r.markClean(ctx, namespace, migration); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *Runner) ensureSchema(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS migrations (
	namespace TEXT NOT NULL,
	version INTEGER NOT NULL,
	name TEXT NOT NULL,
	dirty BOOLEAN NOT NULL,
	applied_at TEXT NOT NULL,
	PRIMARY KEY (namespace, version)
)
`)
	return err
}

func (r *Runner) status(ctx context.Context, namespace string, version int) (dirty bool, exists bool, err error) {
	err = r.db.QueryRowContext(ctx, `
SELECT dirty
FROM migrations
WHERE namespace = ? AND version = ?
`, namespace, version).Scan(&dirty)
	switch {
	case err == nil:
		return dirty, true, nil
	case errors.Is(err, sql.ErrNoRows):
		return false, false, nil
	default:
		return false, false, err
	}
}

func (r *Runner) markDirty(ctx context.Context, namespace string, migration Migration) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO migrations (namespace, version, name, dirty, applied_at)
VALUES (?, ?, ?, 1, ?)
`, namespace, migration.Version, migration.Name, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("migrations: mark dirty namespace=%q version=%d name=%q: %w", namespace, migration.Version, migration.Name, err)
	}
	return nil
}

func (r *Runner) markClean(ctx context.Context, namespace string, migration Migration) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE migrations
SET dirty = 0, applied_at = ?
WHERE namespace = ? AND version = ?
`, time.Now().UTC().Format(time.RFC3339Nano), namespace, migration.Version)
	if err != nil {
		return fmt.Errorf("migrations: mark clean namespace=%q version=%d name=%q: %w", namespace, migration.Version, migration.Name, err)
	}
	return nil
}

func validateMigrations(items []Migration) error {
	seen := make(map[int]struct{}, len(items))
	for _, migration := range items {
		if migration.Version < 1 || strings.TrimSpace(migration.Name) == "" || strings.TrimSpace(migration.SQL) == "" {
			return fmt.Errorf("%w: version=%d name=%q", ErrInvalidMigration, migration.Version, migration.Name)
		}
		if _, exists := seen[migration.Version]; exists {
			return fmt.Errorf("%w: duplicate version=%d", ErrInvalidMigration, migration.Version)
		}
		seen[migration.Version] = struct{}{}
	}

	return nil
}
