package store

import "context"

// Standardisierte Assessments als mitgelieferte Tests.
//
// Urheberrecht: Mitgeliefert werden ausschließlich Verfahren, die ohne Lizenz
// und ohne Anmeldung frei verfügbar sind (Berg Balance Scale, Mini-BESTest,
// Functional Ambulation Categories, Trunk Control Test). Verfahren mit
// Lizenz-, Registrierungs- oder Schulungspflicht (z. B. DEMMI, DGI, FGA, TIS)
// gehören nicht ins Repository – die Praxis legt sie selbst an.
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
}
