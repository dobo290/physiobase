# Roadmap

Was als Nächstes kommt - und was bewusst nicht. Reihenfolge und Umfang sind
Vorschläge, keine Zusagen. Wünsche und Prioritäten gern über
[das Feedback-Formular](https://feedback.gpl-nas.de/feedback) oder als Issue.

**Leitlinien** für alle Entscheidungen:

- Eine statische Go-Binary, eine SQLite-Datei, kein Cloud-Zwang.
- Seiten serverseitig gerendert; JavaScript nur, wo es echten Mehrwert bringt.
- Daten bleiben dort, wo sie hingehören: auf dem eigenen Server.
- Deutschsprachige Oberfläche, deutschsprachige Dokumente.

## Erledigt

| Version | Inhalt |
|---|---|
| 0.9.0 | Körperschema mit Stempeln, Schmerzquellen (P1, P2 ...), eigene Symbole, Punkt-Fragebögen |
| 0.9.1 | Verwaltung eigener Stempel-Symbole über `/admin/symbole` |
| 0.9.2 | Tests am Patienten: Suche im Befund, eigener Block in der Übersicht, Ergebnis und Wiederholung ohne Befund, Demo-Daten |
| 0.9.3 | Erweiterte Stammdaten (Adresse, Beruf, Kasse, Hausarzt/weitere Ärzte, Selbstzahler/Privat/Beihilfe/BG) |

## Kurzfristig (nächste Versionen)

- **Testverlauf als Kurve**: Summe eines Tests über die Zeit im Patientenprofil,
  inklusive Trendpfeil (besser/schlechter) und Vergleich zum Vorbefund.
- **Befund-Suche**: Volltextsuche über Befunde (Diagnose, Notizen), nicht nur
  über das Wissen.
- **Handout-Vorlagen**: Sets aus mehreren Übungen als Vorlage speichern und mit
  einem Klick auf einen Patienten anwenden.
- **Bericht exportieren**: Arztbericht/Zusammenfassung als PDF mit Praxislogo
  und Kopfzeile.

## Mittelfristig

- **Fragebogen per Link**: Patient füllt einen Test vorab im Browser aus
  (Einmal-Link, keine Anmeldung), Ergebnis landet direkt im Patienten.
- **Termin-Erinnerungen**: E-Mail-Erinnerung X Tage vor dem Termin, nur mit
  dokumentierter Einwilligung.
- **Anhänge zu Tests und Befunden**: z. B. Foto des Testaufbaus oder
  Messprotokoll als PDF.
- **ICD-10-/ICF-Codes**: strukturierte Diagnose- und Funktionscodes am Befund,
  durchsuchbar und für Auswertungen nutzbar.
- **Dashboard**: Fallzahlen, Auslastung, offene Befunde, fällige Wiederbefunde.
- **Backup und Wiederherstellung in der Oberfläche**: sichern, herunterladen und
  zurückspielen ohne Shell, inklusive Statusanzeige der letzten Sicherung.

## Langfristig / Ideen

- **Offline-fähige Oberfläche (PWA)**: Befunde im Behandlungsraum auch ohne
  stabiles WLAN erfassen, später synchronisieren.
- **Auswertungen**: anonymisierte Statistik über das eigene Kollektiv, etwa
  Verteilung von Diagnosen oder durchschnittliche Testverbesserung.
- **Mandantenfähigkeit**: mehrere Behandler mit getrennten Datenbeständen in
  einer Instanz.
- **Import aus anderen Systemen**: neben dem bestehenden JSON-Import auch
  CSV/Excel für Patientenstammdaten.

## Bewusst nicht geplant

- Cloud-Hosting oder SaaS-Betrieb durch Dritte.
- Telemetrie, externe Analytics oder "Anrufe nach Hause".
- Zwang zu einem externen Account oder Login-Dienst.
- Proprietäre, lizenzpflichtige Testinhalte im Repository. Instrumente werden
  von der Praxis selbst eingetragen; Lizenzfragen bleiben dort, wo sie hingehören.

## Release-Ablauf

1. `internal/version/version.go` anheben (Patch/Minor).
2. `go vet ./... && go test ./...`, Formatierung mit `gofumpt`.
3. Commit und annotierter Tag `vX.Y.Z`, Push von `master` und Tag.
4. CI baut und veröffentlicht `forge.gpl-nas.de/physiobase:vX.Y.Z` und `:latest`.
5. Live-Instanz: `podman-compose pull && systemctl restart physio`.
