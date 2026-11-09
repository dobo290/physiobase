// Package befund beschreibt das Befundformular als Daten. Formular, Druckansicht,
// Vergleich und Import werden alle aus diesem einen Schema erzeugt.
//
// Die Feldschlüssel entsprechen der früheren Befundseite (data-bind), damit alte
// JSON-Exporte 1:1 importiert werden können.
package befund

type Field struct {
	Key, Label, Placeholder string
	Type                    string // "text" (Standard), "area", "select", "check", "nrs" (0–10)
	Rows                    int
	Options                 []string
	Wide                    bool // volle Breite im Raster
}

type Col struct {
	Key, Label, Placeholder string
	Type                    string // "text", "number", "date", "select"
	Options                 []string
	List                    string // id einer <datalist> mit Vorschlägen
}

type Table struct {
	Key   string
	Cols  []Col
	Match []string // Spalten, über die Zeilen zweier Befunde zugeordnet werden
	Diff  []string // Spalten, deren Werte verglichen werden
}

// Part ist ein Abschnitt innerhalb einer Sektion: Feldraster, Tabelle oder Körperschema.
type Part struct {
	Title       string
	Collapsible bool
	Fields      []Field
	Table       *Table
	Body        bool
}

type Section struct {
	ID, Title string
	Specialty bool // Fachbereich: eingeklappt, außer er enthält Daten
	Parts     []Part
}

func area(key, label string) Field  { return Field{Key: key, Label: label, Type: "area"} }
func text(key, label string) Field  { return Field{Key: key, Label: label} }
func wide(f Field) Field            { f.Wide = true; return f }
func ph(f Field, p string) Field    { f.Placeholder = p; return f }
func rows(f Field, n int) Field     { f.Rows = n; return f }
func check(key, label string) Field { return Field{Key: key, Label: label, Type: "check"} }
func nrs(key, label string) Field   { return Field{Key: key, Label: label, Type: "nrs"} }

var (
	TableTimeline = &Table{Key: "timeline", Cols: []Col{
		{Key: "date", Label: "Datum", Type: "date"},
		{Key: "title", Label: "Ereignis"},
		{Key: "note", Label: "Kurzbeschreibung"},
	}}
	TableROM = &Table{Key: "rom", Match: []string{"joint", "move"}, Diff: []string{"act", "pass", "end"}, Cols: []Col{
		{Key: "joint", Label: "Gelenk", Placeholder: "z. B. Knie re"},
		{Key: "move", Label: "Bewegung", Placeholder: "Flex/Ext"},
		{Key: "act", Label: "Aktiv", Placeholder: "0-0-130"},
		{Key: "pass", Label: "Passiv"},
		{Key: "end", Label: "Endgefühl", Placeholder: "z. B. elastisch"},
	}}
	TableMFP = &Table{Key: "mfp", Match: []string{"muscle"}, Diff: []string{"right", "left"}, Cols: []Col{
		{Key: "muscle", Label: "Muskel / Gruppe", Placeholder: "z. B. Quadrizeps"},
		{Key: "right", Label: "rechts (0–5)", Placeholder: "z. B. 4+"},
		{Key: "left", Label: "links (0–5)", Placeholder: "z. B. 4"},
	}}
	TableCirc = &Table{Key: "circ", Match: []string{"point"}, Diff: []string{"right", "left"}, Cols: []Col{
		{Key: "point", Label: "Messpunkt", Placeholder: "z. B. 10 cm unter Patella"},
		{Key: "right", Label: "rechts (cm)"},
		{Key: "left", Label: "links (cm)"},
	}}
	TableTests = &Table{Key: "tests", Match: []string{"test", "side"}, Diff: []string{"result"}, Cols: []Col{
		{Key: "test", Label: "Test", Placeholder: "z. B. Lachman", List: "dl-tests"},
		{Key: "side", Label: "Seite", Type: "select", Options: []string{"", "re", "li", "bds", "–"}},
		{Key: "result", Label: "Ergebnis", Type: "select", Options: []string{"", "positiv", "negativ", "fraglich"}},
		{Key: "note", Label: "Bemerkung", Placeholder: "z. B. Schmerz reproduziert, Endgefühl"},
	}}
	TableMeasures = &Table{Key: "measures", Match: []string{"tool"}, Diff: []string{"value"}, Cols: []Col{
		{Key: "tool", Label: "Instrument", Placeholder: "z. B. ODI, TUG", List: "dl-measures"},
		{Key: "value", Label: "Wert", Placeholder: "z. B. 34"},
		{Key: "unit", Label: "Einheit / Max.", Placeholder: "z. B. % · s · /80"},
		{Key: "note", Label: "Bemerkung"},
	}}
	TablePSFS = &Table{Key: "psfs", Match: []string{"activity"}, Diff: []string{"score"}, Cols: []Col{
		{Key: "activity", Label: "Aktivität (vom Patienten genannt)", Placeholder: "z. B. Treppe steigen 2 Stockwerke"},
		{Key: "score", Label: "0–10", Type: "number"},
	}}
	Tables = []*Table{TableTimeline, TableROM, TableMFP, TableCirc, TableTests, TableMeasures, TablePSFS}

	// Vorschläge für Instrumente (nur Namen; Werte trägt man selbst ein)
	Measures = []string{
		"NRS Schmerz", "PSFS (Mittelwert)", "ODI", "RMDQ", "NDI", "KOOS", "HOOS", "LEFS", "QuickDASH", "SPADI",
		"VISA-A", "VISA-P", "FAAM", "TSK", "PCS", "STarT Back", "Örebro (Kurzform)", "EQ-5D",
		"TUG", "30s Chair Stand", "5x Sit-to-Stand", "6MWT", "10m Gehtest", "BBS", "Tinetti", "Einbeinstand",
		"Handkraft (Dynamometer)", "Y-Balance", "Single Leg Hop", "Borg (RPE)",
	}
)

var Sections = []Section{
	{ID: "patient", Title: "Patientendaten", Parts: []Part{{Fields: []Field{
		text("diag", "Ärztl. Diagnose"),
		text("onset", "Datum der Schädigung/Erkrankung/OP"),
		area("coDiag", "Nebendiagnosen"),
		area("meds", "Medikamente und Dosierung"),
		area("contra", "Kontraindikationen"),
		area("loadLimit", "Belastbarkeit (ärztl. Vorgaben/Grenze)"),
		area("nursing", "Hinweise aus der Pflege"),
		area("tools", "Hilfsmittel"),
		wide(area("firstHyp", "Erste Hypothese/erwartbare Probleme")),
	}}}},
	{ID: "screening", Title: "Screening: Red & Yellow Flags", Parts: []Part{
		{Title: "Red Flags (Hinweise auf ernste Pathologie)", Fields: []Field{
			check("rf_cancer", "Krebserkrankung in der Vorgeschichte"),
			check("rf_weight", "Ungewollter Gewichtsverlust"),
			check("rf_fever", "Fieber, Infektzeichen, Immunsuppression"),
			check("rf_night", "Ruhe-/Nachtschmerz ohne Bewegungsbezug"),
			check("rf_trauma", "Relevantes Trauma / Osteoporose, Kortison (Fraktur?)"),
			check("rf_cauda", "Reithosenanästhesie, Blasen-/Mastdarmstörung"),
			check("rf_neuro", "Progrediente neurologische Ausfälle"),
			check("rf_vasc", "Hinweise auf TVT/LE, kardiovaskuläre Ursache"),
			check("rf_unwell", "Allgemeines Krankheitsgefühl, systemische Symptome"),
			wide(ph(area("rf_action", "Bewertung & Vorgehen"), "z. B. keine Red Flags · Arzt am … informiert · Behandlung nur nach Abklärung")),
		}},
		{Title: "Yellow / Blue Flags (Chronifizierungsrisiko)", Fields: []Field{
			check("yf_fear", "Angst-Vermeidungsverhalten"),
			check("yf_cata", "Katastrophisieren"),
			check("yf_mood", "Niedergeschlagenheit, Angst, Stress"),
			check("yf_expect", "Geringe Erwartung an Besserung, passive Bewältigung"),
			check("yf_work", "Arbeitsplatzprobleme, Konflikte, fehlende Unterstützung (Blue Flags)"),
			wide(ph(text("screen_tool", "Screening-Instrument & Ergebnis"), "z. B. STarT Back: mittleres Risiko · Örebro-Kurzform: 52")),
		}},
	}},
	{ID: "anamnese", Title: "Anamnese", Parts: []Part{
		{Title: "Körperschema", Body: true},
		{Title: "Schmerz", Fields: []Field{
			nrs("pain_now", "NRS aktuell (0–10)"),
			nrs("pain_best", "NRS beste 24 h"),
			nrs("pain_worst", "NRS schlimmste 24 h"),
			{
				Key: "pain_type", Label: "Dominanter Schmerzmechanismus", Type: "select",
				Options: []string{"", "nozizeptiv", "neuropathisch", "noziplastisch", "gemischt", "unklar"},
			},
		}},
		{Fields: []Field{
			wide(rows(ph(area("anaIntro", "Einleitende Frage(n)"), "Was führt Sie zu mir? …"), 5)),
		}},
		{Title: "Zeitlicher Verlauf", Table: TableTimeline},
		{Fields: []Field{
			area("24h", "24h-Verlauf"),
			area("provo", "Provokation"),
			text("lind", "Linderung"),
			text("S/I", "Schwere/Irritierbarkeit"),
			area("nas", "NAS nach Situation, Qualität etc."),
			area("anaLoad", "Alltägliche Belastung (Beruf, Sport, Hobby, Tätigkeit)"),
			area("envPos", "Umweltfaktoren – positiv"),
			area("envNeg", "Umweltfaktoren – negativ"),
		}},
		{Title: "Allgemeiner Eindruck, Orientierung, Psyche, Kognition, Adhärenz", Collapsible: true, Fields: []Field{
			text("psyche", "Psyche"),
			text("cognition", "Kognition"),
			wide(area("adh", "Compliance/Adhärenz/Sonstiges")),
		}},
	}},
	{ID: "inspektion", Title: "Inspektion", Parts: []Part{{Fields: []Field{
		wide(area("inspection", "Inspektionsbefund (Haltung, Haut, Ödeme, Narben, Atmung etc.)")),
	}}}},
	{ID: "bps", Title: "Psychosoziale Anamnese", Parts: []Part{{Fields: []Field{
		area("bps_family", "Familie/soziales Netz"),
		area("bps_home", "Wohn-/Lebenssituation"),
		area("bps_work", "Beruf/Arbeitsbelastung"),
		area("bps_leisure", "Freizeit/Aktivität"),
		ph(area("bps_activity", "Körperliche Aktivität"), "Art, Min./Woche moderat/intensiv, Krafttraining?"),
		area("bps_sleep", "Schlaf"),
		wide(ph(area("bps_expect", "Vorstellungen & Erwartungen des Patienten"), "Eigene Erklärung der Beschwerden, erwarteter Verlauf, Wunsch an die Behandlung")),
	}}}},
	{ID: "hyp-anam", Title: "Hypothese nach Anamnese", Parts: []Part{{Fields: []Field{
		wide(rows(area("hypAfterAnam", ""), 4)),
	}}}},
	{ID: "exam", Title: "Körperliche Untersuchung – Basis", Parts: []Part{
		{Title: "ROM (Neutral-Null)", Table: TableROM},
		{Title: "MFP (Muskelkraftprüfung)", Fields: []Field{wide(ph(text("mfp-allgemein", "Allgemein"), "z. B. Seitenvergleich, Ermüdung"))}, Table: TableMFP},
		{Title: "Umfangmessung", Collapsible: true, Table: TableCirc},
		{Title: "Spezifische Tests", Table: TableTests},
		{Title: "Assessments & PROMs (Messwerte für den Verlauf)", Table: TableMeasures},
		{Title: "Ganganalyse", Collapsible: true, Fields: []Field{wide(ph(area("gait", ""), "Gangphase, Spurbreite, Armpendel, Ausweichmechanismen …"))}},
		{Title: "Assessments – Notizen", Collapsible: true, Fields: []Field{wide(ph(area("assess", ""), "Durchführung, Auffälligkeiten (Werte bitte in die Tabelle oben)"))}},
		{Title: "Sensibilität/Wahrnehmung", Collapsible: true, Fields: []Field{wide(ph(area("sens", ""), "Reflextests, Dermatome …"))}},
		{Title: "Neurologische Untersuchung", Collapsible: true, Fields: []Field{
			area("neuro_derm", "Sensibilität nach Dermatomen"),
			area("neuro_myo", "Kennmuskeln/Myotome"),
			area("neuro_reflex", "Reflexe (MER, Babinski)"),
			area("neuro_dyn", "Neurodynamik (SLR, Slump, ULNT, PKB)"),
		}},
		{Title: "Sicherheitstests", Collapsible: true, Fields: []Field{wide(ph(area("safetyNeuro", ""), "z. B. Instabilitätstests HWS, vaskuläre Tests"))}},
	}},

	// ---- Fachbereiche ----
	{ID: "ortho", Title: "Chirurgie/Orthopädie", Specialty: true, Parts: []Part{{Fields: []Field{
		area("or_jointProv", "Gelenktest/Provokation"),
		area("or_len", "Muskellänge – Details"),
	}}}},
	{ID: "gyn-geb", Title: "Gynäkologie – Geburtshilfe", Specialty: true, Parts: []Part{
		{Fields: []Field{
			text("gg_gravida", "Schwangerschaften (Gravida)"),
			text("gg_para", "Geburten (Para)"),
			area("gg_pregCourse", "Schwangerschaftsverlauf"),
			area("gg_pregFindings", "Auffälligkeiten in der Schwangerschaft"),
			text("gg_mode", "Geburtsmodus"),
			area("gg_birthCourse", "Geburtsverlauf"),
			text("gg_position", "Gebärposition"),
			area("gg_comp", "Komplikationen"),
		}},
		{Title: "Angaben zum Kind", Collapsible: true, Fields: []Field{
			text("gg_apgar", "APGAR"),
			text("gg_child", "Name/Geschlecht"),
			text("gg_stats", "Größe/Gewicht/KU"),
		}},
		{Fields: []Field{
			area("gg_pelvic", "Beckenbodenbefund"),
			area("gg_abdomen", "Bauchbefund"),
		}},
	}},
	{ID: "gyn-uro", Title: "Gynäkologie/Urologie", Specialty: true, Parts: []Part{{Fields: []Field{
		area("gu_pelvic", "Beckenbodenbefund"),
		area("gu_abdomen", "Bauchbefund"),
	}}}},
	{ID: "innere", Title: "Innere Medizin", Specialty: true, Parts: []Part{
		{Fields: []Field{
			text("im_pulse_point", "Puls (Messpunkt)"),
			ph(text("im_pulse_freqs", "Frequenz Ruhe/Belastung/nachher"), "z. B. 68 / 102 / 80"),
			{Key: "im_pulse_rhythm", Label: "Pulsrhythmus", Type: "select", Options: []string{"", "regelmäßig", "unregelmäßig"}},
			text("im_pulse_fill", "Pulsfüllung"),
			ph(text("im_rr", "RR (Ruhe/Belastung/nachher)"), "z. B. 120/70 / …"),
			text("im_meds", "Beeinflussende Medikamente"),
			wide(area("im_breath", "Atemdiagramm (CSV/CSL/CAL/CAV | ARL/MI/ME)")),
		}},
		{Title: "Thrombosedruckpunkte (positiv)", Fields: []Field{
			check("im_payr", "Payr"), check("im_homann", "Homans"), check("im_adduct", "Adduktoren"),
			check("im_meyer", "Meyer"), check("im_bisgard", "Bisgaard"),
		}},
	}},
	{ID: "neuro", Title: "Neurologie", Specialty: true, Parts: []Part{{Fields: []Field{
		wide(area("ne_tone", "Muskeltonus (Spastik, Klonus, Rigor, Tremor, Akinese, FoG, Ataxie)")),
		area("ne_sens", "Sensibilität (Seitenvergleich)"),
		area("ne_prop", "Propriozeption (UEX/OEX)"),
		area("ne_balance", "Romberg/Einbeinstand"),
		area("ne_reflex", "Holding/Placing/Mirroring"),
	}}}},
	{ID: "paedo", Title: "Pädiatrie", Specialty: true, Parts: []Part{{Fields: []Field{
		area("pa_breath", "Atmung (Sekret, Husten, Atemgeräusche, Atelektasen …)"),
		area("pa_vitals", "Vitalfunktionen (Herzfrequenz, Puls, RR)"),
		area("pa_tone", "Muskeltonus"),
		area("pa_fine", "Feinmotorik"),
		text("pa_gmfcs", "GMFCS"),
		text("pa_mac", "MACS"),
		area("pa_adl", "ADL/Selbstständigkeit"),
		area("pa_move", "Bewegungsanalyse"),
		area("pa_play", "Situations-/Spielverhalten"),
		area("pa_assess", "Assessments"),
	}}}},

	// ---- Auswertung ----
	{ID: "hyp-exam", Title: "Hypothese nach körperlicher Untersuchung", Parts: []Part{{Fields: []Field{
		wide(rows(ph(area("hypAfterExam", "Arbeitshypothese"), "Was spricht dafür (Anamnese, Tests, Messwerte)?"), 4)),
		wide(ph(area("ddx", "Differenzialdiagnosen"), "Alternative Erklärungen – und was dagegen spricht")),
	}}}},
	{ID: "icf", Title: "ICF – Hauptproblem und Problemanalyse", Parts: []Part{{Fields: []Field{
		area("icf_part", "Partizipationsebene"),
		area("icf_activity", "Aktivitätsebene"),
		wide(area("icf_body", "Funktions- und Strukturebene")),
		area("goal_patient", "Patienten-/Angehörigenziel"),
		area("goal_pt", "PT-Ziel (SMART)"),
		wide(area("goal_agreement", "Zielvereinbarung")),
	}}, {Title: "PSFS – Patientenspezifische Funktionsskala (0 = nicht möglich, 10 = wie vor dem Problem)", Table: TablePSFS}}},
	{ID: "prognosis", Title: "Prognose mit Begründung", Parts: []Part{{Fields: []Field{
		wide(rows(area("prognosis", ""), 4)),
	}}}},
	{ID: "plan", Title: "Therapieplan", Parts: []Part{{Fields: []Field{
		wide(rows(area("plan", "Plan (Methoden, Frequenz, Edukation, Heimprogramm, Hilfsmittel)"), 5)),
		wide(ph(area("plan_evidence", "Begründung / Evidenz"), "Leitlinie, Studienlage, Protokoll aus der Wissensdatenbank")),
		ph(text("plan_reassess", "Re-Assessment geplant"), "z. B. nach 6 Behandlungen: NRS, PSFS, ODI"),
		check("consent", "Aufklärung über Befund, Prognose und Plan erfolgt; Einwilligung liegt vor"),
	}}}},
	{ID: "zusatz", Title: "Zusätzliche Notizen", Parts: []Part{{Fields: []Field{
		wide(rows(area("zusatz", ""), 4)),
	}}}},
}

// Symbole für das Körperschema (wie auf der alten Befundseite).
type Symbol struct{ Key, Char, Name string }

var Symbols = []Symbol{
	{"pain", "⚡\ufe0e", "Schmerz"},
	{"swelling", "(( ))", "Schwellung"},
	{"fracture", "◆", "Fraktur"},
	{"tender", "×", "Druckschmerz"},
	{"lesion", "●", "Läsion"},
	{"radiating", "→", "Ausstrahlung"},
	{"sensory", "≈", "Sensibilitätsstörung"},
}

// Ansichten des Körperschemas; W/H = Bildgröße, dient als SVG-viewBox.
type View struct {
	Name, Image string
	W, H        int
}

var Views = []View{
	{"Vorne", "/static/body/body-front.png", 78, 180},
	{"Hinten", "/static/body/body-back.png", 84, 176},
	{"Seitlich", "/static/body/body-side.png", 106, 181},
}

var fieldIndex = func() map[string]Field {
	m := map[string]Field{}
	for _, s := range Sections {
		for _, p := range s.Parts {
			for _, f := range p.Fields {
				if _, dup := m[f.Key]; dup {
					panic("doppelter Feldschlüssel: " + f.Key)
				}
				m[f.Key] = f
			}
		}
	}
	return m
}()

func FieldByKey(k string) (Field, bool) { f, ok := fieldIndex[k]; return f, ok }
