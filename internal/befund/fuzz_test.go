package befund

import "testing"

// FuzzParse: Befund-JSON kommt aus der Datenbank, aus Importdateien und aus
// Sicherungen. Es darf niemals panisch werden, egal was drinsteht.
func FuzzParse(f *testing.F) {
	seeds := []string{
		`{}`,
		`{"fields":{"diag":"Knie","pain_now":"4"},"q":{}}`,
		`{"tables":{"rom":[{"joint":"Hüfte","move":"Flexion","act":"95"}]},"stamps":[],"q":{}}`,
		`{"q":{"7":{"values":["2","3"],"note":"x"}}}`,
		`{"fields":{"diag":"\u0000"},"bad":[1,2,3]}`,
		`null`,
		`{"fields":{}}trailing`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		d, err := Parse(raw)
		if err != nil {
			return
		}
		// Zusicherungen, die die Anzeige braucht: keine Panik, stabile Formate.
		_ = d.Get("diag")
		_ = d.QList()
		_ = PSFSMean(d)
		_ = PainSummary(d)
		for _, v := range Views {
			_ = d.DrawingsFor(0)
			_ = d.StampsFor(0)
			_ = v
			break
		}
	})
}

// FuzzParseQuestionnaire: Fragebogen-Schemata kommen aus dem Editor (Freitext in
// der Datenbank). Kaputte Schemata dürfen keinen Absturz auslösen.
func FuzzParseQuestionnaire(f *testing.F) {
	for _, s := range []string{
		``, `{}`, `null`,
		`{"direction":"lower","items":[{"q":"Schmerz","max":10}]}`,
		`{"direction":"higher","items":[{"q":"","max":0},{"q":"x","max":4}]}`,
		`{"items":[{"q":"a","max":-1}]}`,
		`{"items":"nope"}`,
		`[1,2,3]`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		q, ok := ParseQuestionnaire(raw)
		if !ok {
			return
		}
		if len(q.Items) == 0 {
			t.Fatalf("ok=true ohne Items: %q", raw)
		}
		for _, it := range q.Items {
			if it.Max < 1 || it.Q == "" {
				t.Errorf("ungültiges Item übernommen: %+v", it)
			}
		}
		if q.MaxTotal() <= 0 {
			t.Errorf("MaxTotal <= 0 trotz gültigem Schema: %q", raw)
		}
		if q.Direction != "" && q.Direction != "lower" && q.Direction != "higher" {
			t.Errorf("unerwartete Richtung %q", q.Direction)
		}
	})
}

// FuzzParseLegacy: Import alter "Befund_*.json"-Dateien von fremden Datenträgern.
func FuzzParseLegacy(f *testing.F) {
	for _, s := range []string{
		`{}`,
		`{"fields":{"diag":"Knie"}}`,
		`{"items":[{"name":"a","value":"1"}]}`,
		`{"version":1,"data":{"x":1}}`,
		`[]`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ParseLegacy(b) // darf nicht panisch werden
	})
}
