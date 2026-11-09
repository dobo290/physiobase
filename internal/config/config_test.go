package config

import "testing"

func TestLoadRequiresCredentials(t *testing.T) {
	t.Setenv("PHYSIOBASE_USERNAME", "")
	t.Setenv("PHYSIOBASE_PASSWORD_HASH", "")
	if _, err := Load(); err == nil {
		t.Fatal("Start ohne Zugangsdaten darf nicht gehen")
	}
	t.Setenv("PHYSIOBASE_USERNAME", "admin")
	t.Setenv("PHYSIOBASE_PASSWORD_HASH", "klartext")
	if _, err := Load(); err == nil {
		t.Fatal("Klartext-Passwort darf nicht akzeptiert werden")
	}
	t.Setenv("PHYSIOBASE_PASSWORD_HASH", "$2a$12$abcdefghijklmnopqrstuv")
	t.Setenv("PHYSIOBASE_DATA_DIR", "/x")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.DBPath != "/x/physiobase.db" || c.MediaDir != "/x/medien" || c.Port != 8080 {
		t.Errorf("Defaults falsch: %+v", c)
	}
}
