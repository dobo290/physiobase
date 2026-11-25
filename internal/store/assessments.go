package store

import "context"

// Standardisierte Assessments als mitgelieferte Tests.
//
// Urheberrecht: Die Einträge enthalten nur eine eigene Kurzbeschreibung,
// Punktespanne, Auswertungsrichtung und die Literaturstelle. Die Originalbögen
// (Aufgabenformulierungen, Tabellen, Auswertungsblätter) sind geschützt und
// werden hier bewusst weder abgedruckt noch als Fragebogen hinterlegt – die
// Praxis dokumentiert die Punktwerte ihres lizenzierten Bogens.
//
// Die Namen der Verfahren selbst sind Fachbegriffe und dürfen genannt werden.

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
		Title:  "de Morton Mobility Index (DEMMI)",
		Region: "Gang",
		Tags:   "Assessment, Mobilität, Geriatrie, Verlauf, DEMMI",
		Summary: "Mobilitätstest für ältere und geriatrische Patienten: 15 alltagsnahe Aufgaben " +
			"vom Liegen bis zum freien Gehen, als Rasch-Skala auf 0–100 Punkte abgebildet.",
		Body: `## Ziel
Mobilität objektiv und verlaufsfähig erfassen – auch bei Patienten, die noch im
Bett oder am Stuhl sitzen und deshalb mit Gehtests nicht erreichbar sind.

## Durchführung
15 Aufgaben in fester Reihenfolge (Liegen, Sitzen, Transfer, Stehen, Gehen) mit
klaren Start- und Abbruchregeln. Hilfsmittel sind erlaubt und werden notiert.

## Auswertung
Rohpunkte werden über die veröffentlichte Umrechnungstabelle auf eine Skala von
**0–100** abgebildet, höher = besser. Für den Verlauf zählt die Differenz, nicht
der Einzelwert.

## Quelle
de Morton NA, Davidson M, Keating JL. The de Morton Mobility Index (DEMMI):
an essential health index for an ageing world. *J Clin Epidemiol.* 2008.
` + nutzung,
	},
	{
		Kind:   "test",
		Title:  "Dynamic Gait Index (DGI)",
		Region: "Gang",
		Tags:   "Assessment, Gang, Gleichgewicht, Sturzrisiko, Verlauf, DGI",
		Summary: "Acht Gangaufgaben (Tempo, Kopfdrehen, Drehen, Hindernis, Treppe), je 0–3 Punkte, " +
			"Summe 0–24; Werte ab 19 und darunter sprechen für ein erhöhtes Sturzrisiko.",
		Body: `## Ziel
Gang auf verschiedenen Anforderungsstufen beurteilen, besonders bei Patienten mit
Gleichgewichts- oder Sturzproblemen.

## Durchführung
Acht Aufgaben nach Protokoll (Gehen auf ebener Strecke, Tempowechsel, Kopf drehen,
Drehen, Hindernis übersteigen, Treppe), Hilfsmittel notieren. Jede Aufgabe wird mit
**0–3** bewertet.

## Auswertung
Summe **0–24**, höher = besser. Ein Wert von **19 oder weniger** weist auf ein
erhöhtes Sturzrisiko hin; die einzelne Aufgabe zeigt, wo es hakt.

## Quelle
Shumway-Cook A, Woollacott M. *Motor Control: Theory and Practical Applications.* 1995.
` + nutzung,
	},
	{
		Kind:   "test",
		Title:  "DGI F (Fassung F)",
		Region: "Gang",
		Tags:   "Assessment, Gang, Gleichgewicht, Sturzrisiko, Verlauf, DGI, DGI-F",
		Summary: "Fassung F des Dynamic Gait Index, wie sie in der Praxis verwendet wird. " +
			"Durchführung und Punktwerte nach dem eigenen Praxisbogen.",
		Body: `## Ziel
Gang und Gleichgewicht unter Alltagsanforderungen beurteilen, in der Fassung F des DGI.

## Durchführung
Aufgaben des Praxisbogens in fester Reihenfolge, Hilfsmittel notieren.

## Auswertung
Punktwerte je Aufgabe und Summe dokumentieren; dieselbe Fassung wie beim Vorwert
verwenden, sonst ist der Verlauf nicht vergleichbar.

## Quelle
Praxisbogen „DGI F“. Grundlage: Shumway-Cook A, Woollacott M. *Motor Control.* 1995.
` + nutzung,
	},
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
		Title:  "Functional Gait Assessment (FGA)",
		Region: "Gang",
		Tags:   "Assessment, Gang, Gleichgewicht, Sturzrisiko, Verlauf, FGA",
		Summary: "Zehn Gangaufgaben mit höherem Anspruch als der DGI (u. a. Gehen mit engem " +
			"Stand und rückwärts), je 0–3 Punkte, Summe 0–30.",
		Body: `## Ziel
Gang unter schwierigen Bedingungen prüfen, gut geeignet bei neurologischen
Patienten und zur Sturzrisiko-Einschätzung.

## Durchführung
Zehn Aufgaben nach Protokoll (u. a. Tempo, Kopfdrehen, enger Stand, rückwärts
gehen, Treppe), je **0–3 Punkte**, Hilfsmittel notieren.

## Auswertung
Summe **0–30**, höher = besser. Ein Wert von **22 oder weniger** spricht für ein
erhöhtes Sturzrisiko.

## Quelle
Wrisley DM, Marchetti GF, Kuharsky DK, Whitney SL. Reliability, internal
consistency, and validity of data obtained with the Functional Gait Assessment.
*Phys Ther.* 2004.
` + nutzung,
	},
	{
		Kind:   "test",
		Title:  "Manual Gait Index (MGI)",
		Region: "Gang",
		Tags:   "Assessment, Gang, Beobachtung, Verlauf, MGI",
		Summary: "Manuelle Beurteilung des Gangbilds anhand von Beobachtungskriterien " +
			"(Schrittlänge, Tempo, Symmetrie, Rumpf, Hilfsmittel).",
		Body: `## Ziel
Das Gangbild strukturiert beschreiben und Veränderungen im Verlauf zeigen.

## Durchführung
Patient geht eine feste Strecke, Beobachtung von beiden Seiten und von vorn; die
Kriterien und ihre Gewichtung folgen dem Praxisbogen.

## Auswertung
Teilwerte und Summe dokumentieren, derselbe Bogen wie beim Vorwert. Zusätzlich
notieren, ob ein Hilfsmittel benutzt wurde.

## Quelle
Praxisbogen „Manual Gait Index“. Für Veröffentlichungen bitte die Originalarbeit
der verwendeten Fassung zitieren.
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
		Title:  "Trunk Impairment Scale (TIS)",
		Region: "Gleichgewicht",
		Tags:   "Assessment, Rumpf, Gleichgewicht, Schlaganfall, Verlauf, TIS",
		Summary: "Rumpfkontrolle im Sitzen in drei Bereichen (statisch, dynamisch, Koordination), " +
			"Summe 0–23.",
		Body: `## Ziel
Rumpfkontrolle als Grundlage von Sitzen, Transfer und Gang beurteilen, vor allem
nach Schlaganfall.

## Durchführung
Aufgaben im Sitz ohne Lehne in drei Bereichen: statisches Sitzen, dynamisches
Sitzen (aktiv und passiv), Koordination der Rumpfbewegung.

## Auswertung
Summe **0–23**, höher = besser. Auch die drei Bereichssummen dokumentieren, weil
sie unterschiedliche Defizite abbilden.

## Quelle
Verheyden G, Nieuwboer A, Mertin J, Preger R, Kiekens C, De Weerdt W. The Trunk
Impairment Scale: a new tool to measure motor impairment of the trunk after stroke.
*Clin Rehabil.* 2004.
` + nutzung,
	},
	{
		Kind:   "test",
		Title:  "TIS F (Fassung F)",
		Region: "Gleichgewicht",
		Tags:   "Assessment, Rumpf, Gleichgewicht, Verlauf, TIS, TIS-F",
		Summary: "Fassung F der Trunk Impairment Scale, wie sie in der Praxis verwendet wird. " +
			"Durchführung und Punktwerte nach dem eigenen Praxisbogen.",
		Body: `## Ziel
Rumpfkontrolle im Sitzen beurteilen, in der Fassung F der TIS.

## Durchführung
Aufgaben des Praxisbogens in fester Reihenfolge, im Sitz ohne Lehne.

## Auswertung
Teilwerte und Summe dokumentieren; dieselbe Fassung wie beim Vorwert verwenden.

## Quelle
Praxisbogen „TIS F“. Grundlage: Verheyden G et al. *Clin Rehabil.* 2004.
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
