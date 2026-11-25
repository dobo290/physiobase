# Roadmap

What comes next - and what deliberately does not. Order and scope are proposals,
not promises. Wishes and priorities are welcome via
[the feedback form](https://feedback.gpl-nas.de/feedback) or as an issue.

**Guiding principles** for every decision:

- One static Go binary, one SQLite file, no cloud dependency.
- Pages rendered on the server; JavaScript only where it adds real value.
- Data stays where it belongs: on your own server.
- German user interface and German documents today, but built so that other
  languages can be added.

## Done

| Version | Content |
|---|---|
| 0.9.0 | Body chart with stamps, pain sources (P1, P2 ...), custom symbols, scoring questionnaires |
| 0.9.1 | Managing custom stamp symbols via `/admin/symbole` |
| 0.9.2 | Tests on patients: search inside the assessment, dedicated block in the patient overview, result and repeat run without an assessment, demo data |
| 0.9.3 | Extended patient master data (address, occupation, insurance, family doctor / other physicians, self-payer / private / subsidy / employers' liability) |

## Short term (next releases)

- **Test history as a chart**: score of a test over time in the patient profile,
  including a trend indicator and comparison with the previous assessment.
- **Assessment search**: full text search across assessments (diagnosis, notes),
  not only across the knowledge base.
- **Handout templates**: save sets of exercises as a template and apply them to a
  patient in one click.
- **Report export**: medical report / summary as a clean PDF with practice logo
  and header.
- **Recall list**: patients whose next appointment or follow-up assessment is due,
  with a reminder function.
- **Audit log**: who changed or viewed which assessment and when (GDPR
  accountability, useful for handovers).

## Mid term

- **Prescription management**: remedy prescriptions with validity period, number
  of sessions, approval status and remaining budget per patient.
- **Billing**: remedy catalogue positions, prices, invoices for self-payers and
  private patients, payment status.
- **Questionnaire by link**: the patient fills in a test in the browser before the
  visit (one-time link, no account), the result lands directly in the patient file.
- **Patient self-booking**: a public calendar link through which patients request
  appointments (confirmed by the practice).
- **Consents and signatures**: capture forms (photos, e-mail delivery) digitally
  and store them in the patient file.
- **Appointment reminders**: e-mail reminder X days before the appointment, only
  with documented consent.
- **Attachments for tests and assessments**: e.g. a photo of the test setup or a
  measurement report as PDF.
- **ICD-10 / ICF codes**: structured diagnosis and functioning codes on the
  assessment, searchable and usable for statistics.
- **Dashboard**: case numbers, utilisation, open assessments, due follow-ups,
  no-show rate.
- **Backup and restore in the UI**: back up, download and restore without a shell,
  including the status of the last backup.

## Long term / ideas

- **Internationalisation**: the project must be translatable into other languages.
  That means externalising all UI strings and document templates (German as the
  default, English and Dutch next - the KNGF guidelines are published in Dutch),
  per-patient output language for questionnaires, and a documented translation
  workflow so the community can contribute languages.
- **Other therapy professions**: besides physiotherapy also occupational therapy,
  speech therapy and further remedy professions. Separate assessment schemas,
  test collections and handouts per profession, without restructuring existing data.
- **Telematics infrastructure (TI)**: connector and KIM for the secure exchange of
  medical letters and findings, card terminals (eHBA / SMC-B) and - as far as
  available for remedy providers - electronic prescriptions. This is a long-term
  topic with certification requirements and will be planned carefully before any
  code is written.
- **Opt-in encryption**: store database and media encrypted (passphrase at start),
  encrypted backups, optional field-level encryption for particularly sensitive
  data. Default behaviour stays transparent when encryption is not enabled.
- **Offline-capable UI (PWA)**: record assessments in the treatment room without a
  stable Wi-Fi connection and synchronise later.
- **Analytics**: anonymised statistics over your own caseload, e.g. distribution of
  diagnoses or average test improvement.
- **Multi-tenancy**: several practitioners with separate data sets in one instance.
- **Import from other systems**: in addition to the existing JSON import, CSV/Excel
  for patient master data and digitisation of paper files.
- **Retention periods and deletion concept**: configurable deadlines, reminders for
  archiving and a deletion log.

## Deliberately not planned

- Cloud hosting or SaaS operation by third parties.
- Telemetry, external analytics or "phoning home".
- Forcing an external account or login service.
- Proprietary, licensed test content in the repository. Instruments are entered by
  the practice itself; licensing questions stay where they belong.

## Release process

1. Bump `internal/version/version.go` (patch/minor).
2. `go vet ./... && go test ./...`, format with `gofumpt`.
3. Commit and annotated tag `vX.Y.Z`, push `master` and the tag.
4. CI builds and publishes `forge.gpl-nas.de/physiobase:vX.Y.Z` and `:latest`.
5. Live instance: `podman-compose pull && systemctl restart physio`.
