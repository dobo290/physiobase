# Datenschutz und medizinische Vorgaben – Stand und offene Punkte

**Stand:** 25.11.2026 · Version 0.13.4

Dieses Dokument sammelt, welche datenschutzrechtlichen und medizinischen Vorgaben
für eine physiotherapeutische Praxis gelten, **was PhysioBase davon technisch
abdeckt** und **was noch fehlt**. Es ist eine technische Checkliste für Praxis,
IT und Prüfung – **keine Rechtsberatung**. Die rechtliche Bewertung (Rechtsgrundlage,
Verzeichnis, DSFA, Meldeprozess) bleibt Aufgabe der Praxis.

Verantwortlich im Sinne der DSGVO ist die **Praxis**, nicht das Projekt. PhysioBase
ist ein Werkzeug, das die Praxis selbst betreibt; es gibt keinen Herstellerzugriff,
keine Telemetrie und keine Cloud-Anbindung.

---

## 1. Rechtlicher Rahmen

| Norm | Anforderung | Betrifft die Software |
|---|---|---|
| **Art. 5 DSGVO** | Grundsätze: Zweckbindung, Datenminimierung, Richtigkeit, Speicherbegrenzung, Integrität | Datenmodell, Löschkonzept, Export |
| **Art. 6, 9 DSGVO** i. V. m. § 22 BDSG | Rechtsgrundlage, Gesundheitsdaten als besondere Kategorie | Dokumentation der Rechtsgrundlage (Praxis), keine Weitergabe durch die Software |
| **Art. 12–22 DSGVO** | Betroffenenrechte: Auskunft, Berichtigung, Löschung, Einschränkung, Übertragbarkeit | Auskunft/Export, Korrektur, Löschbremse |
| **Art. 25 DSGVO** | Datenschutz durch Technikgestaltung | lokal, keine Cloud, keine Telemetrie |
| **Art. 28 DSGVO** | Auftragsverarbeitung | nur relevant bei externem Hosting/Backup |
| **Art. 30 DSGVO** | Verzeichnis von Verarbeitungstätigkeiten | Vorlage in Abschnitt 5 |
| **Art. 32 DSGVO** | Stand der Technik: Vertraulichkeit, Integrität, Verfügbarkeit, Belastbarkeit | Zugriffsschutz, Backups, Verschlüsselung |
| **Art. 33/34 DSGVO** | Meldung von Verletzungen binnen 72 Stunden | Protokolle, Prozessbeschreibung |
| **Art. 35 DSGVO** | Datenschutz-Folgenabschätzung bei hohem Risiko | Screening dokumentieren |
| **§ 630f BGB** | Dokumentationspflicht, **Aufbewahrung 10 Jahre** | Aufbewahrungsfrist, Löschbremse |
| **§ 630g BGB** | Einsichtnahme durch Patientinnen und Patienten | Ausdrucke, Auskunft/Export |
| **§ 630h BGB** | Beweislast bei Dokumentationslücken | Vollständigkeit, Unveränderbarkeit |
| **§ 203 StGB** | Schweigepflicht | Zugriffsbeschränkung, Rollen |
| **SGB V, Heilmittel-Richtlinie, § 125 SGB V** | Verordnung, Leistungsbeschreibung, Abrechnung | Rezepte, Kataloge, Abrechnung |
| **MDR (EU 2017/745)** | Medizinprodukt-Einstufung | Abgrenzung in Abschnitt 6 |
| **EU AI Act** | Pflichten bei KI-Funktionen | heute keine KI-Funktionen |

---

## 2. Was heute technisch abgedeckt ist

| Bereich | Umsetzung in PhysioBase |
|---|---|
| **Zugriffsschutz** | Anmeldung mit Passwort oder Passkey, optional TOTP als zweiter Faktor, signierte Sitzungs-Cookies, serverseitige Prüfung gegen die Datenbank, Abmelden/Passwortwechsel beendet alle Sitzungen |
| **Rollen** | `admin` und `user`; Verwaltungsseiten unter `/admin` sind für andere Benutzer nicht existent (404), nicht nur ausgeblendet |
| **Brute-Force-Schutz** | begrenzte parallele bcrypt-Prüfungen, Wartezeit nach Fehlversuch |
| **Transport** | läuft hinter HTTPS-Proxy (Caddy), `Secure`-Cookies und HSTS bei `https://`-Basis-URL, strikte Content-Security-Policy ohne Inline-Skripte |
| **Datenhaltung** | eine SQLite-Datei plus Medienordner auf dem eigenen Server, kein Cloud-Dienst, keine Telemetrie, keine externen Aufrufe im Normalbetrieb |
| **Dateizugriff** | Medien und Dokumente werden nur an angemeldete Benutzer ausgeliefert, nie öffentlich gecacht |
| **Integrität** | Fremdschlüssel mit Kaskaden, Migrationen versioniert, Tests inkl. Migrations- und Backup-Test |
| **Verfügbarkeit** | tägliche konsistente Snapshots (`VACUUM INTO`), rotierend; Datenbank im WAL-Modus |
| **Demo** | eigene Instanz ohne echte Daten, Zurücksetzen bei Abmelden; Besucherstatistik nur mit Einwilligung |
| **Betrieb** | CI prüft Tests, Linter, `govulncheck`, CodeQL, Dependabot, Secret Scanning, Image-Scan (Trivy), Image reproduzierbar gebaut |
| **Löschen** | explizite Löschaktion mit Bestätigung; Kaskade entfernt auch Befunde, Sitzungen, Dokumente; sonst nur Archivierung (`archived`), Daten bleiben erhalten |

---

## 3. Offene Punkte (Lücken)

| # | Anforderung | Heute | Fehlt | Roadmap |
|---|---|---|---|---|
| 1 | **10-Jahres-Aufbewahrung** (§ 630f Abs. 3 BGB) | Archiv-Flag, aber Löschen ist jederzeit möglich | Frist je Patient, Anzeige „aufbewahren bis“, **Löschbremse**, Löschprotokoll | *next* |
| 2 | **Auskunft und Datenübertragbarkeit** (Art. 15, 20) | Ausdrucke je Patient (Befund, Zusammenfassung, Termine) | ein Export „alle Daten dieses Patienten“ als Datei | offen |
| 3 | **Löschung und Einschränkung** (Art. 17, 18) | Löschen möglich, aber ohne Prüfung gegen Aufbewahrungspflichten | begründete Ausnahme bei Löschverlangen, Protokoll | *next* (mit 1) |
| 4 | **Audit-Log** (Art. 5 Abs. 2, Art. 32) | nur Server-Log mit Anmelde-Ereignissen, nicht auswertbar | revisionsfähiges Protokoll: wer hat was geändert/gesehen | *next* |
| 5 | **Verschlüsselung** (Art. 32 Abs. 1 lit. a) | keine Verschlüsselung „at rest“, Backups unverschlüsselt | Datenbank-/Backup-Verschlüsselung mit Passphrase | *long term* |
| 6 | **Wiederherstellung** (Art. 32 Abs. 1 lit. b, c) | Backups vorhanden, Restore nur per Shell | Restore in der Oberfläche, dokumentierter Restore-Test | *mid term* |
| 7 | **Verzeichnis von Verarbeitungstätigkeiten** (Art. 30) | nicht vorhanden | Vorlage + Ausfüllhilfe (Abschnitt 5) | offen |
| 8 | **Datenschutz-Folgenabschätzung** (Art. 35) | nicht dokumentiert | Screening-Vorlage mit Ergebnis „erforderlich / nicht erforderlich“ | offen |
| 9 | **Meldeprozess bei Vorfällen** (Art. 33/34) | nicht beschrieben | Ablauf, Zuständigkeit, Fristen, Vorlage der Meldung | offen |
| 10 | **Datenschutzhinweise für Patienten** (Art. 13) | nicht vorhanden | Vorlage zum Aushängen/Ausgeben | offen |
| 11 | **Einwilligungen** | E-Mail-Einwilligung im Patientenstamm; Fotos, Messenger, Telefonaufnahmen nicht abgebildet | Einwilligungsverwaltung mit Datum, Widerruf, Zweck | offen |
| 12 | **Auftragsverarbeitung** (Art. 28) | Selbstbetrieb: nicht erforderlich | nur bei externem Hosting/Backup/Wartung nötig → AVV schließen | Praxisaufgabe |
| 13 | **Zugriffsberechtigungen** (Art. 32, § 203 StGB) | pro Benutzer ein Konto, Admin-Rolle | Rechte je Patient/Fall, Vertretungszugriff | offen |
| 14 | **Speicherbegrenzung / Löschkonzept** (Art. 5 Abs. 1 lit. e) | nur manuelles Archivieren | Regeln je Datenart (Befunde, Dokumente, E-Mails, Logs) | *long term* (mit 1) |

---

## 4. Aufgaben der Praxis (nicht der Software)

Die Software kann diese Punkte nicht ersetzen:

1. **Rechtsgrundlage** je Verarbeitung festhalten (Behandlung, Abrechnung, gesetzliche Pflichten).
2. **Verzeichnis von Verarbeitungstätigkeiten** führen (Abschnitt 5) und aktuell halten.
3. **DSFA-Screening** dokumentieren; bei hohem Risiko eine DSFA durchführen.
4. **Auftragsverarbeitungsverträge** mit allen Dienstleistern, die Daten sehen können (Hosting, Backup, IT-Wartung, Steuerberatung bei Datenzugriff).
5. **Zugriffsverwaltung**: wer hat ein Konto, wer darf administrieren, wer wird beim Ausscheiden deaktiviert.
6. **Geräte- und Netzsicherheit**: Datenträgerverschlüsselung, Bildschirmsperre, kein Zugriff von außen ohne VPN, WLAN-/Routerpflege.
7. **Meldeprozess** bei Datenschutzverletzungen inklusive 72-Stunden-Frist und Erreichbarkeit.
8. **Schulung** der Mitarbeitenden (Vertraulichkeit, Passwörter, Umgang mit Ausdrucken).
9. **Aufbewahrung organisatorisch** sichern: was liegt nach den 10 Jahren noch wo (Papier, E-Mails, Kalender)?
10. **Datenschutzbeauftragte/n** benennen, falls die gesetzlichen Schwellen erreicht sind.

---

## 5. Vorlage: Verzeichnis von Verarbeitungstätigkeiten

| Feld | Beispiel (bitte anpassen) |
|---|---|
| Verarbeitungstätigkeit | Patientenbehandlung und Dokumentation |
| Verantwortlicher | Praxisname, Anschrift, Kontakt |
| Zwecke | Behandlung, Dokumentation, Abrechnung, Nachweis der Aufklärung |
| Datenkategorien | Stammdaten, Gesundheitsdaten (Befunde, Messwerte, Fotos), Verordnungen, Rechnungsdaten |
| Betroffene | Patientinnen und Patienten, ggf. Angehörige |
| Rechtsgrundlage | Art. 9 Abs. 2 lit. h DSGVO i. V. m. § 22 BDSG; § 630f BGB; SGB V |
| Empfänger | Kostenträger, verordnende Ärzte, ggf. Abrechnungsstelle |
| Drittlandtransfer | nein (kein Cloud-Dienst) |
| Löschfristen | 10 Jahre nach der letzten Behandlung, danach Löschung |
| Technische Maßnahmen | siehe Abschnitt 2 dieses Dokuments |

---

## 6. Abgrenzung Medizinprodukt (MDR)

PhysioBase **dokumentiert** und **rechnet ab**; es stellt keine Diagnosen, schlägt
keine Therapie vor und löst keine Alarme aus. Nach der Abgrenzung der EU-Kommission
(MDCG 2019-11) ist reine Dokumentationssoftware damit **kein Medizinprodukt**.

Diese Einschätzung muss neu bewertet werden, sobald Funktionen dazukommen, die

- Befunde automatisch interpretieren oder Therapien vorschlagen,
- Messwerte gegen Grenzwerte überwachen und melden,
- KI-gestützte Auswertungen oder Entscheidungshilfen enthalten.

Der Punkt „KI-Funktionen“ ist in der Roadmap nicht vorgesehen; sollte er kommen,
gehört eine MDR- und AI-Act-Prüfung dazu.

---

## 7. Prüfliste für eine Begehung

- [ ] Verzeichnis der Verarbeitungstätigkeiten liegt vor und passt zur Praxis
- [ ] Rechtsgrundlagen dokumentiert (Behandlung, Abrechnung, gesetzliche Pflicht)
- [ ] Datenschutzhinweise werden den Patienten ausgehändigt
- [ ] Einwilligungen (E-Mail, Fotos) sind je Patient dokumentiert und widerrufbar
- [ ] Aufbewahrungsfristen sind bekannt; Löschen passiert nur mit Prüfung und Protokoll
- [ ] Zugriffskonten vollständig, alte Konten deaktiviert, Adminzugang beschränkt
- [ ] Zweiter Faktor für alle Konten mit Zugriff auf Gesundheitsdaten aktiv
- [ ] HTTPS im Einsatz, Fernzugriff nur über VPN
- [ ] Backups laufen täglich, ein Restore wurde **einmal praktisch geübt**
- [ ] Meldeprozess für Datenschutzverletzungen bekannt und erreichbar
- [ ] AVV mit allen Dienstleistern vorhanden, die Daten sehen können
- [ ] DSFA-Screening dokumentiert (Ergebnis und Begründung)

---

## 8. Verhältnis zur Roadmap

Die technischen Lücken aus Abschnitt 3 stehen in `ROADMAP.md`:

- **Ten-year retention for medical records** (*next*) – Punkte 1 und 3
- **Audit log** (*next*) – Punkt 4
- **Backup and restore in the UI** (*mid term*) – Punkt 6
- **Opt-in encryption** (*long term*) – Punkt 5
- **Retention periods and deletion concept** (*long term*) – Punkt 14

Punkte 2, 7–11 und 13 sind noch nicht in der Roadmap eingetragen; sie werden dort
ergänzt, sobald sie geplant werden.
