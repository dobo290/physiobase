package web

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"forge.gpl-nas.de/physiobase/internal/befund"
)

const customSymbolsKey = "custom.symbols"

// defaultSymbols: die eingebauten Symbole ohne die Schmerzquelle (die läuft jetzt
// über „Schmerzquelle“ und ist nummeriert).
func defaultSymbols() []befund.Symbol {
	var out []befund.Symbol
	for _, s := range befund.Symbols {
		if s.Key == "pain" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func (s *Server) loadCustomSymbols(ctx context.Context) ([]befund.Symbol, error) {
	raw, err := s.st.Setting(ctx, customSymbolsKey)
	if err != nil {
		return nil, err
	}
	var out []befund.Symbol
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Server) saveCustomSymbols(ctx context.Context, syms []befund.Symbol) error {
	b, _ := json.Marshal(syms)
	return s.st.SetSetting(ctx, customSymbolsKey, string(b))
}

// effectiveSymbols: eingebaute (ohne Schmerz) + eigene, nach Name sortiert.
func (s *Server) effectiveSymbols(ctx context.Context) ([]befund.Symbol, error) {
	custom, err := s.loadCustomSymbols(ctx)
	if err != nil {
		return nil, err
	}
	out := append([]befund.Symbol{}, defaultSymbols()...)
	out = append(out, custom...)
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

// usedSymbolsFor: effektive Symbole, die im Befund tatsächlich verwendet werden
// (Vergleich über Schlüssel oder gespeichertes Zeichen).
func (s *Server) usedSymbolsFor(ctx context.Context, d befund.Data) ([]befund.Symbol, error) {
	eff, err := s.effectiveSymbols(ctx)
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	for _, st := range d.Stamps {
		if st.S == "pain" {
			continue
		}
		if st.Ch != "" {
			used["ch:"+st.Ch] = true
		} else if st.S != "" {
			used["k:"+st.S] = true
		}
	}
	var out []befund.Symbol
	for _, sym := range eff {
		if used["k:"+sym.Key] || used["ch:"+sym.Char] {
			out = append(out, sym)
		}
	}
	return out, nil
}

// ---------- Admin ----------

func (s *Server) symbolsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eff, err := s.effectiveSymbols(ctx)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	custom, err := s.loadCustomSymbols(ctx)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	q := r.URL.Query()
	s.render(w, r, "admin_symbols", http.StatusOK, map[string]any{
		"Symbols": eff, "Custom": custom, "Builtin": len(defaultSymbols()),
		"Error": q.Get("fehler"), "Saved": q.Get("gespeichert") == "1",
	})
}

func (s *Server) addSymbol(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	ctx := r.Context()
	ch := strings.TrimSpace(r.FormValue("char"))
	name := strings.TrimSpace(r.FormValue("name"))
	if ch == "" || name == "" {
		redirect(w, r, "/admin/symbole?fehler=%s", urlPathEscape("Zeichen und Name sind Pflicht."))
		return
	}
	if len([]rune(ch)) > 4 || len([]rune(name)) > 40 {
		redirect(w, r, "/admin/symbole?fehler=%s", urlPathEscape("Zeichen höchstens 4, Name höchstens 40 Zeichen."))
		return
	}
	custom, err := s.loadCustomSymbols(ctx)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	// Schlüssel eindeutig: custom-1, custom-2, …
	n := 1
	usedKeys := map[string]bool{}
	for _, x := range custom {
		usedKeys[x.Key] = true
	}
	for usedKeys["custom-"+itoa(n)] {
		n++
	}
	custom = append(custom, befund.Symbol{Key: "custom-" + itoa(n), Char: ch, Name: name})
	if err := s.saveCustomSymbols(ctx, custom); err != nil {
		s.fail(w, r, err)
		return
	}
	s.log.Info("symbol hinzugefügt", "name", name)
	redirect(w, r, "/admin/symbole?gespeichert=1")
}

func (s *Server) deleteSymbol(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := r.PathValue("key")
	custom, err := s.loadCustomSymbols(ctx)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	kept := custom[:0]
	for _, x := range custom {
		if x.Key != key {
			kept = append(kept, x)
		}
	}
	if err := s.saveCustomSymbols(ctx, kept); err != nil {
		s.fail(w, r, err)
		return
	}
	s.log.Info("symbol entfernt", "key", key)
	redirect(w, r, "/admin/symbole?gespeichert=1")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
