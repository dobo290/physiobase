package web

import "net/http"

// Hell-/Dunkelmodus pro Gerät. Der Server liest das Cookie beim Rendern, damit
// die Seite nicht erst hell aufblitzt; das Skript schaltet sofort um.
const themeCookie = "physiobase_theme"

// themeName liefert "light", "dark" oder "" (Systemeinstellung).
func themeName(r *http.Request) string {
	c, err := r.Cookie(themeCookie)
	if err != nil {
		return ""
	}
	switch c.Value {
	case "light", "dark":
		return c.Value
	}
	return ""
}

// toggleTheme schaltet zwischen hell und dunkel um (Formular-Fallback ohne JS).
func (s *Server) toggleTheme(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	next := "dark"
	if themeName(r) == "dark" {
		next = "light"
	}
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: reines UI-Cookie, kein Geheimnis
		Name: themeCookie, Value: next, Path: "/", MaxAge: 365 * 24 * 60 * 60,
		SameSite: http.SameSiteLaxMode,
	})
	back := r.FormValue("zurueck")
	if back == "" || back[0] != '/' || len(back) > 512 {
		back = "/"
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}
