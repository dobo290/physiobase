// Package config liest die gesamte Konfiguration aus Umgebungsvariablen.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Port         int
	DataDir      string
	DBPath       string
	MediaDir     string
	DocsDir      string
	BackupDir    string
	SMTP         SMTP
	BackupKeep   int
	Username     string
	PasswordHash string
	PracticeName string
	BaseURL      string
	Seed         bool
	LogLevel     slog.Level
	LogJSON      bool
}

// SMTP für den Versand von Zusammenfassungen. Leerer Host = Versand aus.
type SMTP struct {
	Host, User, Password, From string
	Port                       int
	Security                   string // "ssl" (implizites TLS, 465), "starttls" (587) oder "none" (nur Tests/LAN)
}

func (s SMTP) Enabled() bool { return s.Host != "" && s.From != "" }

func (c *Config) IsHTTPS() bool { return strings.HasPrefix(c.BaseURL, "https://") }

func get(k, def string) string {
	if v, ok := os.LookupEnv("PHYSIOBASE_" + k); ok && v != "" {
		return v
	}
	return def
}

// Load liest die Konfiguration. Ohne Benutzername und Passwort-Hash startet die
// App nicht – es gibt keinen Modus ohne Anmeldung.
func Load() (*Config, error) {
	c := &Config{
		DataDir:      get("DATA_DIR", "./data"),
		Username:     get("USERNAME", ""),
		PasswordHash: get("PASSWORD_HASH", ""),
		PracticeName: get("PRACTICE_NAME", "PhysioBase"),
		BaseURL:      strings.TrimRight(get("BASE_URL", ""), "/"),
	}
	c.DBPath = get("DB_PATH", filepath.Join(c.DataDir, "physiobase.db"))
	c.MediaDir = get("MEDIA_DIR", filepath.Join(c.DataDir, "medien"))
	c.DocsDir = get("DOCS_DIR", filepath.Join(c.DataDir, "patienten"))
	c.BackupDir = get("BACKUP_DIR", filepath.Join(c.DataDir, "backup"))

	var err error
	if c.Port, err = strconv.Atoi(get("PORT", "8080")); err != nil || c.Port < 1 || c.Port > 65535 {
		return nil, fmt.Errorf("PHYSIOBASE_PORT ungültig: %q", get("PORT", ""))
	}
	if c.BackupKeep, err = strconv.Atoi(get("BACKUP_KEEP", "14")); err != nil {
		return nil, fmt.Errorf("PHYSIOBASE_BACKUP_KEEP ungültig: %w", err)
	}
	c.SMTP = SMTP{
		Host: get("SMTP_HOST", ""), User: get("SMTP_USER", ""), Password: get("SMTP_PASSWORD", ""),
		From: get("SMTP_FROM", get("SMTP_USER", "")), Security: get("SMTP_SECURITY", "starttls"),
	}
	if c.SMTP.Port, err = strconv.Atoi(get("SMTP_PORT", "587")); err != nil {
		return nil, fmt.Errorf("PHYSIOBASE_SMTP_PORT ungültig: %w", err)
	}
	switch c.SMTP.Security {
	case "ssl", "starttls", "none":
	default:
		return nil, fmt.Errorf("PHYSIOBASE_SMTP_SECURITY muss ssl, starttls oder none sein, nicht %q", c.SMTP.Security)
	}
	if c.Seed, err = strconv.ParseBool(get("SEED", "1")); err != nil {
		return nil, fmt.Errorf("PHYSIOBASE_SEED ungültig: %w", err)
	}
	if err := c.LogLevel.UnmarshalText([]byte(get("LOG_LEVEL", "info"))); err != nil {
		return nil, fmt.Errorf("PHYSIOBASE_LOG_LEVEL ungültig: %w", err)
	}
	switch f := get("LOG_FORMAT", "text"); f {
	case "text":
	case "json":
		c.LogJSON = true
	default:
		return nil, fmt.Errorf("PHYSIOBASE_LOG_FORMAT muss text oder json sein, nicht %q", f)
	}

	if c.Username == "" || c.PasswordHash == "" {
		return nil, errors.New("PHYSIOBASE_USERNAME und PHYSIOBASE_PASSWORD_HASH sind Pflicht (Hash erzeugen: physiobase hash-password)")
	}
	if !strings.HasPrefix(c.PasswordHash, "$2") {
		return nil, errors.New("PHYSIOBASE_PASSWORD_HASH ist kein bcrypt-Hash")
	}
	return c, nil
}
