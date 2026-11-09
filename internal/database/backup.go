package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Backup schreibt eine konsistente Kopie der laufenden Datenbank (VACUUM INTO).
// Ein einfaches cp der .db-Datei reicht im WAL-Modus NICHT – Daten liegen ggf. noch in -wal.
func Backup(ctx context.Context, db *sql.DB, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	name := filepath.Join(dir, "physiobase-"+time.Now().Format("2006-01-02_150405")+".db")
	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, name); err != nil {
		os.Remove(name)
		return "", fmt.Errorf("backup: %w", err)
	}
	if err := os.Chmod(name, 0o600); err != nil {
		return name, fmt.Errorf("backup rechte: %w", err)
	}
	return name, nil
}

// Prune behält die neuesten keep Backups.
func Prune(dir string, keep int) error {
	files, err := filepath.Glob(filepath.Join(dir, "physiobase-*.db"))
	if err != nil {
		return err
	}
	sort.Strings(files) // Zeitstempel im Namen → chronologisch
	for i := 0; i < len(files)-keep; i++ {
		if strings.HasPrefix(filepath.Base(files[i]), "physiobase-") {
			os.Remove(files[i])
		}
	}
	return nil
}

// RunBackups: sofort beim Start, danach alle 24 h, bis ctx endet.
func RunBackups(ctx context.Context, db *sql.DB, dir string, keep int, log *slog.Logger) {
	if keep <= 0 {
		log.Info("automatische backups deaktiviert")
		return
	}
	run := func() {
		name, err := Backup(ctx, db, dir)
		if err != nil {
			log.Error("backup fehlgeschlagen", "err", err)
			return
		}
		if err := Prune(dir, keep); err != nil {
			log.Warn("alte backups nicht gelöscht", "err", err)
		}
		log.Info("backup erstellt", "datei", name)
	}
	run()
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}
