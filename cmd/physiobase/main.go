// physiobase – Wissensdatenbank für die Physiotherapie.
//
//	physiobase                   Server starten (Konfiguration über PHYSIOBASE_*)
//	physiobase hash-password     bcrypt-Hash für PHYSIOBASE_PASSWORD_HASH (Passwort über stdin)
//	physiobase healthcheck       /healthz prüfen (für den Container-HEALTHCHECK)
//	physiobase seed-assessments  mitgelieferte Assessments nachtragen (idempotent)
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"forge.gpl-nas.de/physiobase/internal/config"
	"forge.gpl-nas.de/physiobase/internal/database"
	"forge.gpl-nas.de/physiobase/internal/store"
	"forge.gpl-nas.de/physiobase/internal/web"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "hash-password":
			os.Exit(hashPassword())
		case "healthcheck":
			os.Exit(healthcheck())
		case "seed-assessments":
			os.Exit(seedAssessments())
		default:
			fmt.Fprintf(os.Stderr, "unbekannter Befehl %q (hash-password, healthcheck, seed-assessments)\n", os.Args[1])
			os.Exit(2)
		}
	}
	if err := run(); err != nil {
		slog.Error("beendet mit fehler", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	var h slog.Handler = slog.NewTextHandler(os.Stderr, opts)
	if cfg.LogJSON {
		h = slog.NewJSONHandler(os.Stderr, opts)
	}
	log := slog.New(h)
	slog.SetDefault(log)

	for _, dir := range []string{cfg.MediaDir, cfg.DocsDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("ordner %s: %w", dir, err)
		}
	}
	db, err := database.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := database.Close(db); err != nil {
			log.Error("datenbank schließen", "err", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go database.RunBackups(ctx, db, cfg.BackupDir, cfg.BackupKeep, log)

	st := store.New(db)
	if cfg.Seed {
		if seeded, err := st.Seed(ctx); err != nil {
			return fmt.Errorf("beispieldaten: %w", err)
		} else if seeded {
			log.Info("beispieleinträge angelegt")
			if n, err := st.SeedAssessments(ctx); err != nil {
				return fmt.Errorf("assessments: %w", err)
			} else if n > 0 {
				log.Info("assessments angelegt", "anzahl", n)
			}
		}
	}
	srv, err := web.New(ctx, cfg, st, log)
	if err != nil {
		return err
	}

	httpSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("physiobase gestartet", "port", cfg.Port, "db", cfg.DBPath, "medien", cfg.MediaDir)
		errCh <- httpSrv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("fahre herunter")
		shutCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutCtx); err != nil {
			return err
		}
	}
	return nil
}

// seedAssessments trägt die mitgelieferten Assessments in eine bestehende
// Installation nach. Mehrfaches Aufrufen ist harmlos.
func seedAssessments() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	db, err := database.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() {
		if err := database.Close(db); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	n, err := store.New(db).SeedAssessments(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("%d Assessments angelegt\n", n)
	return 0
}

func hashPassword() int {
	fmt.Fprint(os.Stderr, "Passwort: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(os.Stderr, "kein Passwort gelesen")
		return 1
	}
	pw := strings.TrimRight(line, "\r\n")
	if len(pw) < 8 {
		fmt.Fprintln(os.Stderr, "Passwort zu kurz (mind. 8 Zeichen)")
		return 1
	}
	h, err := web.HashPassword(pw)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(h)
	return 0
}

func healthcheck() int {
	port := os.Getenv("PHYSIOBASE_PORT")
	if port == "" {
		port = "8080"
	}
	c := http.Client{Timeout: 4 * time.Second}
	if _, err := strconv.Atoi(port); err != nil {
		return 1
	}
	// Fester Loopback-Aufruf, Port ist numerisch geprüft.
	resp, err := c.Get("http://127.0.0.1:" + port + "/healthz") //nolint:gosec // G704: kein fremdes Ziel möglich
	if err != nil {
		return 1
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
