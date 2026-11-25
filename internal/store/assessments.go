package store

import "context"

// Standardisierte Assessments als mitgelieferte Tests.
//
// Urheberrecht: Mitgeliefert werden ausschließlich Verfahren, die ohne Lizenz,
// Anmeldung oder Schulung frei verfügbar und evidenzbasiert sind:
// Berg Balance Scale, Mini-BESTest, Functional Ambulation Categories, Trunk
// Control Test, Timed Up and Go, 10-Meter-Gehtest, 6-Minuten-Gehtest,
// 30-Sekunden-Aufsteh-Test, Five Times Sit to Stand, 4-Stufen-Balance-Test,
// Functional Reach Test, SPPB und PSFS.
// Schmerz (NRS) und PSFS stehen im Befund bereits ausführlicher – die NRS
// deshalb nicht zusätzlich als eigener Test.
// Verfahren mit Lizenz-, Registrierungs- oder Schulungspflicht (z. B. DEMMI,
// DGI, FGA, TIS, die meisten PROM-Fragebögen) gehören nicht ins Repository –
// die Praxis legt sie selbst an.
//
// Auch bei den freien Verfahren stehen hier nur eine eigene Kurzbeschreibung,
// Punktespanne, Auswertungsrichtung und die Literaturstelle; die Originalbögen
// bleiben bei der Praxis.

// SeedAssessments legt die mitgelieferten Assessments an, sofern noch kein Test
// mit demselben Titel existiert. Mehrfaches Aufrufen ändert nichts.
// Rückgabe: Anzahl neu angelegter Einträge.
func (s *Store) SeedAssessments(ctx context.Context) (int, error) {
	created := 0
	for _, e := range assessmentEntries {
		var n int
		if err := s.db.QueryRowContext(ctx,
			`SELECT count(*) FROM entries WHERE kind = 'test' AND title = ?`, e.Title).Scan(&n); err != nil {
			return created, err
		}
		if n > 0 {
			continue
		}
		if _, err := s.CreateEntry(ctx, e); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// nutzung ist der gemeinsame Hinweis am Ende jedes Eintrags.
const nutzung = `
## Nutzung
Punktwerte des eigenen, lizenzierten Bogens hier dokumentieren – die
Originalformulare sind urheberrechtlich geschützt und werden nicht mitgeliefert.
`

var assessmentEntries = []Entry{
	{
		Kind:   "test",
		Title:  "Functional Ambulation Categories (FAC)",
		Region: "Gang",
		Tags:   "Assessment, Gang, Selbstständigkeit, Verlauf, FAC",
		Summary: "Sechs Stufen (0–5) für die Selbstständigkeit beim Gehen: von „nicht gehfähig“ " +
			"bis „geht selbstständig ohne Hilfe in Alltag und Treppe“.",
		Body: `## Ziel
Schnell einordnen, wie selbstständig ein Patient geht und wie viel Hilfe er braucht.

## Durchführung
Beobachtung des Gehens in Alltagssituationen, mit und ohne Hilfsmittel; die
Einordnung bezieht sich auf das, was tatsächlich beobachtet wurde.

## Auswertung
Sechs Stufen **0–5**, höher = selbstständiger:
- 0 = kein Gehen möglich
- 1–2 = nur mit personeller Hilfe
- 3 = selbstständig auf ebener Fläche, Treppe mit Hilfe
- 4–5 = selbstständig, auch Treppe bzw. in jeder Umgebung

## Quelle
Holden MK, Gill KM, Magliozzi MR, Nathan J, Piehl-Baker L. Clinical gait
assessment in the neurologically impaired. *Phys Ther.* 1984.
` + nutzung,
	},
	{
		Kind:   "test",
		Title:  "Mini-BESTest",
		Region: "Gleichgewicht",
		Tags:   "Assessment, Gleichgewicht, Sturzrisiko, Verlauf, Mini-BESTest",
		Summary: "Vierzehn Aufgaben aus vier Bereichen (vorbereitende Haltungsanpassung, " +
			"reaktive Kontrolle, sensorische Orientierung, dynamischer Gang), je 0–2, Summe 0–28.",
		Body: `## Ziel
Gleichgewicht in vier Bereichen prüfen und die Ursache eines Sturzrisikos eingrenzen.

## Durchführung
Vierzehn Aufgaben nach Protokoll, je **0–2 Punkte**. Die Aufgaben sind in vier
Bereiche gegliedert: vorbereitende Haltungsanpassung, reaktive Haltungskontrolle,
sensorische Orientierung, dynamischer Gang.

## Auswertung
Summe **0–28**, höher = besser. Die Bereichssummen zeigen, welcher Anteil des
Gleichgewichts betroffen ist – hilfreicher für die Therapieplanung als die Summe allein.

## Quelle
Franchignoni F, Horak F, Godi M, Nardone A, Giordano A. Using psychometric
techniques to improve the Mini-BESTest: a Rasch analysis. *Eur J Phys Rehabil Med.* 2010.
` + nutzung,
	},
	{
		Kind:   "test",
		Title:  "Trunk Control Test (TCT)",
		Region: "Gleichgewicht",
		Tags:   "Assessment, Rumpf, Schlaganfall, Prognose, Verlauf, TCT",
		Summary: "Vier Rumpfaufgaben (Drehen zu beiden Seiten, Sitzen, Aufstehen), Summe 0–100; " +
			"ein früher Wert über 12 spricht für eine günstigere Prognose.",
		Body: `## Ziel
Rumpfkontrolle nach Schlaganfall rasch einschätzen – auch bettlägerige Patienten
lassen sich damit beurteilen.

## Durchführung
Vier Aufgaben: Drehen zur schwachen Seite, Drehen zur starken Seite, Gleichgewicht
im Sitz, Aufstehen aus dem Sitz.

## Auswertung
Jede Aufgabe wird gestuft bewertet (0 für unmöglich, Zwischenstufe 12 oder 25 je
nach Aufgabe, volle Punktzahl bei selbstständiger Durchführung); Summe **0–100**,
höher = besser. Ein Wert von **12 oder mehr** früh nach dem Ereignis gilt als
günstiges Zeichen.

## Quelle
Collin C, Wade DT. Assessing motor impairment after stroke: a pilot reliability
study. *J Neurol Neurosurg Psychiatry.* 1990.
` + nutzung,
	},
	{
		Kind:   "test",
		Title:  "Berg Balance Scale (BBS)",
		Region: "Gleichgewicht",
		Tags:   "Assessment, Gleichgewicht, Sturzrisiko, Geriatrie, Verlauf, BBS",
		Summary: "Vierzehn Gleichgewichtsaufgaben (Sitzen, Stehen, Transfer, Drehen, Einbeinstand), " +
			"je 0–4 Punkte, Summe 0–56.",
		Body: `## Ziel
Gleichgewicht im Alltag beurteilen und das Sturzrisiko einschätzen, vor allem bei
älteren Patienten und nach Schlaganfall.

## Durchführung
Vierzehn Aufgaben nach Protokoll (Sitzen und Stehen, Transfers, Drehen, Greifen,
Einbeinstand), je **0–4 Punkte**. Hilfsmittel und Aufsicht werden in der Bewertung
berücksichtigt.

## Auswertung
Summe **0–56**, höher = besser. Werte **unter 45** sprechen für ein erhöhtes
Sturzrisiko; die Einzelaufgaben zeigen, welche Situationen unsicher sind.

## Quelle
Berg KO, Wood-Dauphinee SL, Williams JI, Maki B. Measuring balance in the elderly:
validation of an instrument. *Can J Public Health.* 1992.
` + nutzung,
	},
	{
		Kind:   "test",
		Title:  "Timed Up and Go (TUG)",
		Region: "Gang",
		Tags:   "Assessment, Gang, Gleichgewicht, Sturzrisiko, Verlauf, TUG",
		Summary: "Aufstehen, 3 m gehen, drehen, zurückgehen und hinsetzen – die Zeit in Sekunden. " +
			"Ab etwa 13,5 s ist das Sturzrisiko erhöht.",
		Body: `## Ziel
Gang, Drehen und Transfer in einer alltagsnahen Aufgabe messen; braucht nur eine
Stoppuhr, einen Stuhl und 3 m Platz.

## Durchführung
Patient sitzt, steht auf dem Kommando auf, geht zügig (nicht rennend) 3 m, dreht,
geht zurück und setzt sich. Zeit vom Kommando bis zum Sitzen; Hilfsmittel notieren.

## Auswertung
Zeit in Sekunden, **kleiner = besser**. Ab **13,5 s** ist das Sturzrisiko erhöht; für
den Verlauf zählt die Differenz (eine Änderung von etwa 2 s gilt als relevant).
Zweite Durchführung notieren und den besseren Wert nehmen.

## Quelle
Podsiadlo D, Richardson S. The Timed "Up & Go": a test of basic functional
mobility for frail elderly persons. *J Am Geriatr Soc.* 1991.
` + nutzung,
	},
	{
		Kind:   "test",
		Title:  "10-Meter-Gehtest (10MWT)",
		Region: "Gang",
		Tags:   "Assessment, Gang, Gehgeschwindigkeit, Verlauf, 10MWT",
		Summary: "Gehgeschwindigkeit über 10 m, in m/s – komfortabel und schnell gemessen. " +
			"Unter 0,8 m/s spricht für ein erhöhtes Risiko im Alltag.",
		Body: `## Ziel
Gehgeschwindigkeit objektiv messen. Sie ist ein gut belegter Prädiktor für Stürze,
Krankenhausaufenthalte und Selbstständigkeit.

## Durchführung
10 m abgesteckte Strecke, 2 m Anlauf und 2 m Auslauf. Zwei Bedingungen: komfortabel
und so schnell wie sicher möglich. Zeit mit der Stoppuhr, Hilfsmittel notieren.

## Auswertung
Geschwindigkeit = 10 m ÷ Zeit in Sekunden, in **m/s**, höher = besser. Unter
**0,8 m/s** ist die Alltagsmobilität häufig eingeschränkt. Für den Verlauf zählt
die Differenz (rund 0,1–0,2 m/s gelten als relevant).

## Quelle
Bohannon RW. Comfortable and maximum walking speed of adults aged 20–79 years:
reference values and determinants. *Age Ageing.* 1997.
` + nutzung,
	},
	{
		Kind:   "test",
		Title:  "6-Minuten-Gehtest (6MWT)",
		Region: "Gang",
		Tags:   "Assessment, Gang, Ausdauer, Verlauf, 6MWT",
		Summary: "Sechs Minuten zügig gehen, Strecke in Metern – Belastungstest für " +
			"Ausdauer und Alltagsfähigkeit.",
		Body: `## Ziel
Submaximale Ausdauerleistung im Gehen messen, gut geeignet bei kardialen,
pulmonalen und geriatrischen Patienten.

## Durchführung
30 m lange Strecke mit Markierungen, Patient geht in sechs Minuten so weit wie
möglich (Pausen erlaubt, Sitzen nicht). Alle 60 s Zeit ansagen, ermutigen, nicht
mitgehen. Vorher/nachher Puls, Blutdruck und Dyspnoe (z. B. Borg) notieren.

## Auswertung
Strecke in **Metern**, höher = besser. Rund **30 m** Änderung gelten als relevante
Veränderung. Immer dieselbe Strecke und dieselbe Hilfsmittel-Situation messen,
sonst ist der Verlauf nicht vergleichbar.

## Quelle
ATS Committee on Proficiency Standards for Clinical Pulmonary Function
Laboratories. ATS statement: guidelines for the six-minute walk test.
*Am J Respir Crit Care Med.* 2002.
` + nutzung,
	},
	{
		Kind:   "test",
		Title:  "30-Sekunden-Aufsteh-Test (30sCST)",
		Region: "Gang",
		Tags:   "Assessment, Kraft, Geriatrie, Verlauf, 30sCST",
		Summary: "So oft wie möglich in 30 Sekunden von einem Stuhl aufstehen – Maß für " +
			"Beinkraft und Ausdauer.",
		Body: `## Ziel
Funktionelle Beinkraft und Ausdauer ohne Geräte messen; für Verlauf und Sturzrisiko
gut geeignet.

## Durchführung
Stuhl ohne Armlehnen, Rücken gerade, Arme vor dem Körper gekreuzt. In 30 Sekunden
so oft wie möglich vollständig aufstehen und setzen. Ein Probedurchgang zum Üben.

## Auswertung
Anzahl vollständiger Aufstehbewegungen in 30 s, höher = besser. Beurteilung am
eigenen Vorwert orientieren; auffällig wenige Wiederholungen sprechen für eine
reduzierte Beinkraft.

## Quelle
Jones CJ, Rikli RE, Beam WC. A 30-s chair-stand test as a measure of lower body
strength in community-dwelling older adults. *Res Q Exerc Sport.* 1999.
` + nutzung,
	},
	{
		Kind:   "test",
		Title:  "Five Times Sit to Stand (5xSTS)",
		Region: "Gang",
		Tags:   "Assessment, Kraft, Transfer, Verlauf, 5xSTS",
		Summary: "Fünfmal aufstehen und setzen, so schnell wie möglich – Zeit in Sekunden " +
			"als Maß für Kraft und Transferfähigkeit.",
		Body: `## Ziel
Beinkraft und Transferfähigkeit messen, auch bei jüngeren und orthopädischen
Patienten gut einsetzbar.

## Durchführung
Stuhl ohne Armlehnen, Arme vor dem Körper gekreuzt, fünfmal vollständig aufstehen
und setzen, so schnell wie sicher möglich. Zeit ab Kommando bis zum fünften
abgeschlossenen Sitz.

## Auswertung
Zeit in Sekunden, **kleiner = besser**. Über 12 s weist häufig auf eine reduzierte
Beinkraft hin; der eigene Vorwert ist der wichtigste Vergleich.

## Quelle
Csuka M, McCarty DJ. Simple method for measurement of lower extremity muscle
strength. *J Rheumatol.* 1985.
` + nutzung,
	},
	{
		Kind:   "test",
		Title:  "4-Stufen-Balance-Test",
		Region: "Gleichgewicht",
		Tags:   "Assessment, Gleichgewicht, Sturzrisiko, Geriatrie, Verlauf, STEADI",
		Summary: "Stand mit geschlossenen Füßen, halb-Tandem, Tandem und auf einem Bein – " +
			"je 10 Sekunden. Nicht geschafft heißt erhöhtes Sturzrisiko.",
		Body: `## Ziel
Standgleichgewicht in vier Stufen prüfen, schnell und ohne Geräte, gut als
Einstieg in die Sturzrisiko-Abklärung.

## Durchführung
Vier Positionen, jeweils bis zu 10 Sekunden halten: Füße geschlossen, halb-Tandem
(Ferse neben Zehenmitte), Tandem (Ferse vor Zehenspitzen) und Einbeinstand. Sicher
stehen, Sturzgefahr absichern, Zeit stoppen bis zum Verlust der Position.

## Auswertung
Die höchste sicher gehaltene Stufe notieren, **10 s je Stufe** gelten als bestanden.
Wer die Tandemposition keine 10 s halten kann, hat ein erhöhtes Sturzrisiko. Den
Einbeinstand zusätzlich in Sekunden notieren, er zeigt Veränderungen früher.

## Quelle
Rossiter ND, Schuster SE, Burns R, et al. The 4-stage balance test and falls in
older people. *Age Ageing.* 1998. Verbreitet über CDC STEADI (frei verfügbar).
` + nutzung,
	},
	{
		Kind:   "test",
		Title:  "Functional Reach Test (FRT)",
		Region: "Gleichgewicht",
		Tags:   "Assessment, Gleichgewicht, Sturzrisiko, Verlauf, FRT",
		Summary: "Maximale Reichweite im Stand nach vorn, ohne Schritt – in Zentimetern. " +
			"Unter 15 cm spricht für ein erhöhtes Sturzrisiko.",
		Body: `## Ziel
Grenzen der Standstabilität messen: wie weit kann der Patient nach vorn reichen,
ohne einen Schritt zu machen.

## Durchführung
Seitlich neben einer Wand, Arm etwa 90 Grad vorgestreckt, Maßstab auf Schulterhöhe.
Ohne Schritt und ohne Festhalten so weit wie möglich nach vorn reichen. Drei
Versuche, der Mittelwert zählt.

## Auswertung
Reichweite in **Zentimetern**, höher = besser. **Unter 15 cm** weist auf ein
erhöhtes Sturzrisiko hin.

## Quelle
Duncan PW, Weiner DK, Chandler J, Studenski S. Functional reach: a new clinical
measure of balance. *J Gerontol.* 1990.
` + nutzung,
	},
	{
		Kind:   "test",
		Title:  "Short Physical Performance Battery (SPPB)",
		Region: "Gleichgewicht",
		Tags:   "Assessment, Mobilität, Geriatrie, Sturzrisiko, Verlauf, SPPB",
		Summary: "Drei Aufgaben – Standbalance, 4-Meter-Gang und fünfmal Aufstehen – " +
			"je 0–4 Punkte, Summe 0–12.",
		Body: `## Ziel
Körperliche Leistungsfähigkeit älterer Menschen standardisiert erfassen; ein gut
belegter Prädiktor für Stürze, Behinderung und Sterblichkeit.

## Durchführung
Drei Teile: Standbalance (Füße zusammen, halb-Tandem, Tandem je 10 s), 4 m
Gehgeschwindigkeit und fünfmal Aufstehen aus dem Stuhl mit Zeitmessung.

## Auswertung
Jeder Teil **0–4 Punkte**, Summe **0–12**, höher = besser. Werte unter 10 zeigen
eine eingeschränkte Leistungsfähigkeit; eine Änderung um 1 Punkt ist klinisch
bereits bedeutsam.

## Quelle
Guralnik JM, Simonsick EM, Ferrucci L, et al. A short physical performance battery
assessing lower extremity function. *J Gerontol.* 1994.
` + nutzung,
	},
	{
		Kind:   "test",
		Title:  "Patient-Specific Functional Scale (PSFS)",
		Region: "Allgemein",
		Tags:   "Assessment, Alltag, Ziele, Verlauf, PSFS",
		Summary: "Drei bis fünf Tätigkeiten aus dem Alltag des Patienten, je 0–10 bewertet – " +
			"zeigt Veränderung dort, wo sie für den Patienten zählt.",
		Body: `## Ziel
Veränderung in genau den Tätigkeiten messen, die dem Patienten wichtig sind – gut
als Ergänzung zu standardisierten Tests.

## Durchführung
Patient nennt drei bis fünf Tätigkeiten, die er wegen der Beschwerden nicht oder
nur eingeschränkt kann. Jede wird mit **0 (gar nicht möglich) bis 10 (problemlos
möglich)** bewertet. Die Tätigkeiten beim Verlauf unverändert lassen.

## Auswertung
Mittelwert der Bewertungen, **höher = besser**. Rund **2 Punkte** Änderung gelten
als relevant; einzelne Tätigkeiten zusätzlich ansehen.

## Quelle
Stratford P, Gill C, Westaway M, Binkley J. Assessing disability and change on
individual patients: a report of a patient specific measure. *Physiother Can.* 1995.
` + nutzung,
	},
}
