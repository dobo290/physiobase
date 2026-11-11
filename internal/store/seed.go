package store

import "context"

// Seed legt einige Beispieleinträge an, aber nur in einer leeren Datenbank.
// Sie zeigen, wie Einträge aufgebaut und verknüpft werden, und dürfen gelöscht werden.
func (s *Store) Seed(ctx context.Context) (bool, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM entries`).Scan(&n); err != nil {
		return false, err
	}
	if done, _ := s.Setting(ctx, "seeded"); n > 0 || done != "" {
		return false, nil
	}

	ids := map[string]int64{}
	for _, e := range seedEntries {
		id, err := s.CreateEntry(ctx, e.Entry)
		if err != nil {
			return false, err
		}
		ids[e.key] = id
	}
	for _, l := range seedLinks {
		if err := s.Link(ctx, ids[l[0]], ids[l[1]]); err != nil {
			return false, err
		}
	}
	if err := s.AddSource(ctx, Source{
		EntryID: ids["pt"], Ref: "Eigenes Lehrbuch / Skript (Beispiel)", Page: "S. 123",
		Note: "Hier Buch aus der Calibre-Bibliothek eintragen",
	}); err != nil {
		return false, err
	}
	hid, err := s.CreateHandout(ctx, "Knie – Sehnenprogramm Woche 1–2", "")
	if err != nil {
		return false, err
	}
	for _, k := range []string{"iso", "spanish"} {
		if err := s.AddHandoutItem(ctx, hid, ids[k]); err != nil {
			return false, err
		}
	}
	return true, s.SetSetting(ctx, "seeded", "1")
}

type seedEntry struct {
	key string
	Entry
}

var seedLinks = [][2]string{
	{"pt", "iso"},
	{"pt", "spanish"},
	{"pt", "decline"},
	{"pt", "visa"},
	{"pt", "proto"},
	{"proto", "iso"},
	{"proto", "spanish"},
	{"proto", "decline"},
}

var seedEntries = []seedEntry{
	{"pt", Entry{
		Kind: "krankheitsbild", Title: "Patellatendinopathie (Jumper's Knee)", Region: "Knie",
		Tags:    "Sehne, Überlastung, Sprungsport, Beispiel",
		Summary: "Belastungsabhängiger Schmerz am unteren Patellapol, typisch bei Sprung- und Sprintsportarten.",
		Body: `## Befund
- Schmerz lokal am unteren Patellapol, **belastungsabhängig** (Springen, Treppe, tiefe Hocke)
- Typisch: Anlaufschmerz, wird beim Aufwärmen besser, danach / am Folgetag schlechter
- Druckschmerz allein ist wenig spezifisch

## Red Flags / Differenzialdiagnosen
- Schwellung, Erguss, Blockaden → an intraartikuläre Ursache denken
- Jugendliche: Sinding-Larsen-Johansson / Osgood-Schlatter

## Behandlungsidee
1. **Belastung steuern**, nicht komplett pausieren (Schmerz während Übung bis ca. 3–4/10 meist akzeptabel, am Folgetag nicht schlechter)
2. Isometrie zur Schmerzmodulation
3. Schwere, langsame Kraftübungen (Heavy Slow Resistance)
4. Energiespeicher-Belastung (Sprünge) schrittweise zurück
5. Zurück in den Sport

## Eigene Notizen
_Hier eigene Erfahrungen ergänzen._`,
	}},
	{"visa", Entry{
		Kind: "test", Title: "Punkt-Fragebogen Knie (Beispiel)", Region: "Knie", Tags: "Fragebogen, Verlaufskontrolle, Beispiel",
		Summary: "Selbst gemachtetes Beispiel für einen Punkt-Fragebogen (0–100, höher = besser). Eigene, lizenzierte Instrumente hier eintragen.",
		Body: `## Durchführung
Beispielhafte Fragen zu Schmerz, Funktion und Sportfähigkeit – hier eigene Fragen verwenden.

## Interpretation
- höhere Punktzahl = besser
- Verlauf alle 2–4 Wochen dokumentieren`,
	}},
	{"iso", Entry{
		Kind: "uebung", Title: "Isometrische Kniestreckung (Wandsitz)", Region: "Knie",
		Tags:    "Isometrie, Schmerzlinderung, Sehne, Beispiel",
		Summary: "Statisches Halten zur Schmerzlinderung bei Sehnenbeschwerden, gut als Einstieg.",
		Dosage:  "5 × 45 Sekunden halten, 2 Min. Pause, täglich",
		Body: `## Ziel
Schmerzmodulation, Belastbarkeit aufbauen.

## Progression
Winkel tiefer → einbeinig → mit Zusatzgewicht

## Hinweise für Therapeut
Schmerz während der Übung bis etwa 3/10 ist meist in Ordnung.`,
		PatientText: `Lehnen Sie sich mit dem Rücken an eine Wand und rutschen Sie langsam nach unten, bis die Knie etwa **60 Grad** gebeugt sind.

- Halten Sie die Position ruhig, atmen Sie dabei weiter.
- Die Knie zeigen in Richtung der Fußspitzen.
- Ein leichtes Ziehen an der Sehne ist in Ordnung. Wird der Schmerz stärker, gehen Sie weniger tief.`,
	}},
	{"spanish", Entry{
		Kind: "uebung", Title: "Spanish Squat", Region: "Knie",
		Tags:    "Isometrie, Quadrizeps, Sehne, Beispiel",
		Summary: "Kniebeuge mit Band hinter den Knien. Belastet den Quadrizeps stark bei wenig Kniescherkräften.",
		Dosage:  "5 × 45 Sekunden halten",
		Body: `## Aufbau
Festes Band (z. B. Gurt) an einem stabilen Pfosten, Band in den Kniekehlen.

## Progression
Tiefer → länger halten → dynamisch`,
		PatientText: `Legen Sie das Band in die Kniekehlen und lehnen Sie sich nach hinten hinein. Gehen Sie in die Hocke, die **Schienbeine bleiben senkrecht**.

- Oberkörper aufrecht, Gewicht auf den Fersen.
- Position halten, ruhig weiteratmen.`,
	}},
	{"decline", Entry{
		Kind: "uebung", Title: "Kniebeuge auf Schrägbrett (exzentrisch)", Region: "Knie",
		Tags:    "Exzentrik, Kraft, Sehne, Beispiel",
		Summary: "Langsame Kniebeuge auf einem schrägen Brett, betont die Patellarsehne.",
		Dosage:  "3 × 15 Wiederholungen, 3 Sek. runter",
		Body: `## Hinweise
Erst wenn Isometrie gut vertragen wird. Schmerz am Folgetag beobachten.`,
		PatientText: `Stellen Sie sich auf das Schrägbrett, Fersen oben. Gehen Sie **langsam (3 Sekunden)** in die Kniebeuge bis etwa 60–70 Grad.

- Hochkommen mit beiden Beinen.
- Die Knie zeigen in Richtung der Fußspitzen.`,
	}},
	{"fragebogen-beispiel", Entry{
		Kind: "test", Title: "Beispielfragebogen: Alltagsbelastung Knie", Region: "Knie",
		Tags:    "Fragebogen, Beispiel, Punkte",
		Summary: "Selbst gemachtes Beispiel, um Punkt-Fragebögen auszuprobieren. Hier eigene (lizenzierte) Fragebögen eintragen.",
		Body:    "Jede Frage wird mit 0 bis zur Maximalpunktzahl beantwortet; die Punkte werden aufsummiert. Richtung hier: niedriger ist besser.",
		Schema:  `{"direction":"lower","items":[{"q":"Schmerzen beim Treppensteigen","max":10},{"q":"Schmerzen nach 30 Min. Gehen","max":10},{"q":"Schmerzen beim tiefen Hocken","max":10}]}`,
	}},
	{"proto", Entry{
		Kind: "protokoll", Title: "Sehnen-Rehabilitation – Stufenmodell", Region: "Allgemein",
		Tags:    "Tendinopathie, Belastungsaufbau, Beispiel",
		Summary: "Allgemeiner Stufenplan für Sehnenbeschwerden: von Isometrie bis zur Rückkehr in den Sport.",
		Body: `| Stufe | Inhalt | Weiter wenn |
|---|---|---|
| 1 | Isometrie, mehrmals täglich | Schmerz während Übung ≤ 3/10 |
| 2 | Schwere, langsame Kraftübungen, 3×/Woche | Kraft steigt, Folgetag stabil |
| 3 | Energiespeicher: Hüpfen, Sprünge | Sprünge schmerzarm |
| 4 | Sportspezifisch, Rückkehr | Volle Belastung toleriert |

**Regel:** Beschwerden am Folgetag sind der wichtigste Maßstab.`,
	}},
}
