# Roadmap

**Last updated:** 25.11.2026

**Legend:** `[x]` done · `[ ]` planned · 🚧 currently in progress

Guiding principles for every decision:

- One static Go binary, one SQLite file, no cloud dependency.
- Pages rendered on the server; JavaScript only where it adds real value.
- Data stays where it belongs: on your own server.
- German user interface today, built so that other languages can be added.

## 🚧 In progress

- [ ] 🚧 **Test history as a chart** - score of a test over time in the patient
      profile, with trend indicator and comparison to the previous assessment

## Next

- [ ] **Assessment search** - full text search across assessments (diagnosis,
      notes), not only across the knowledge base
- [ ] **Handout templates** - save sets of exercises as a template and apply them
      to a patient in one click
- [ ] **Report export** - medical report / summary as a clean PDF with practice
      logo and header
- [ ] **Recall list** - patients whose next appointment or follow-up assessment is
      due, with a reminder function
- [ ] **Audit log** - who changed or viewed which assessment and when (GDPR
      accountability, useful for handovers)

## Mid term

- [ ] **Prescription management** - remedy prescriptions with validity period,
      number of sessions, approval status and remaining budget per patient
- [ ] **Billing** - remedy catalogue positions, prices, invoices for self-payers
      and private patients, payment status
- [ ] **Questionnaire by link** - the patient fills in a test in the browser
      before the visit (one-time link, no account)
- [ ] **Patient self-booking** - public calendar link for appointment requests,
      confirmed by the practice
- [ ] **Consents and signatures** - capture forms digitally and store them in the
      patient file
- [ ] **Appointment reminders** - e-mail reminder before the appointment, only
      with documented consent
- [ ] **Attachments for tests and assessments** - e.g. photo of the test setup or
      a measurement report as PDF
- [ ] **ICD-10 / ICF codes** - structured diagnosis and functioning codes on the
      assessment, searchable and usable for statistics
- [ ] **Dashboard** - case numbers, utilisation, open assessments, due follow-ups,
      no-show rate
- [ ] **Backup and restore in the UI** - back up, download and restore without a
      shell, including the status of the last backup

## Long term / ideas

- [ ] **Internationalisation** - the project must be translatable into other
      languages: externalise all UI strings and document templates (German as the
      default, English and Dutch next - the KNGF guidelines are published in
      Dutch), per-patient output language for questionnaires, and a documented
      translation workflow so the community can contribute languages
- [ ] **Other therapy professions** - besides physiotherapy also occupational
      therapy, speech therapy and further remedy professions, with separate
      assessment schemas, test collections and handouts per profession
- [ ] **Telematics infrastructure (TI)** - connector and KIM for the secure
      exchange of medical letters and findings, card terminals (eHBA / SMC-B) and,
      as far as available for remedy providers, electronic prescriptions
- [ ] **Opt-in encryption** - encrypted database and media (passphrase at start),
      encrypted backups, optional field-level encryption for particularly
      sensitive data
- [ ] **Offline-capable UI (PWA)** - record assessments in the treatment room
      without a stable Wi-Fi connection and synchronise later
- [ ] **Analytics** - anonymised statistics over your own caseload
- [ ] **Multi-tenancy** - several practitioners with separate data sets in one
      instance
- [ ] **Import from other systems** - CSV/Excel for patient master data and
      digitisation of paper files
- [ ] **Retention periods and deletion concept** - configurable deadlines,
      reminders for archiving and a deletion log

## Done

- [x] ~~**Faster test search and deleting from lists** - clicking the search field
      lists every test, typing filters letter by letter, arrow keys and Enter pick
      one; entries can be deleted from the knowledge list and appointments from the
      overview, each with the confirmation step (0.12.0)~~
- [x] ~~**Free assessments included** - Berg Balance Scale, Mini-BESTest, Functional
      Ambulation Categories and Trunk Control Test as ready test entries with scale,
      scoring notes and reference; licensed instruments stay with the practice.
      `physiobase seed-assessments` adds them to an existing database (0.12.0)~~
- [x] ~~**Two-factor sign-in** - optional TOTP per user (Google Authenticator, Aegis,
      1Password, Vaultwarden …), QR code for setup, eight one-time recovery codes,
      admin reset when the device is lost (0.10.0)~~
- [x] ~~**Roadmap tab in the forge** - this file is shown as a tab in the project
      view, including the open/done counter~~
- [x] ~~**Extended patient master data** - address, occupation, insurance, family
      doctor, other physicians, self-payer / private / subsidy / employers'
      liability (0.9.3)~~
- [x] ~~**Tests on patients** - search inside the assessment, dedicated block in the
      patient overview, result and repeat run without an assessment, demo data
      (0.9.2)~~
- [x] ~~**Custom stamp symbols** - manage symbols via `/admin/symbole` (0.9.1)~~
- [x] ~~**Body chart with stamps, pain sources (P1, P2 ...), scoring questionnaires**
      (0.9.0)~~

## Deliberately not planned

- [x] ~~Cloud hosting or SaaS operation by third parties~~ - rejected
- [x] ~~Telemetry, external analytics or "phoning home"~~ - rejected
- [x] ~~Forcing an external account or login service~~ - rejected
- [x] ~~Proprietary, licensed test content in the repository~~ - instruments are
      entered by the practice itself

## Versioning

Semantic versioning, pre-1.0 with a fixed rule so the numbers stay informative:

| Change | Version |
|---|---|
| New feature, new field, new page, database migration | `0.<minor>.0` – 0.10.0, 0.11.0, 0.12.0 … |
| Bug fix only, no new function, no migration | `0.<minor>.<patch>` – 0.10.1, 0.10.2 … |
| First stable release | `1.0.0` |

Rules:

- A migration always means a new minor version, so "do I need a backup before
  updating?" depends only on the second number.
- Patch releases never change the database schema and never remove a feature, so
  they are safe on a running practice.
- Breaking changes are allowed before 1.0.0, but they are called out in the
  release notes.
- Dates in the repository, in releases and in the app: `TT.MM.JJJJ`. Units metric.

### What 1.0.0 requires

- Data model settled: no migration that rewrites patient data, only additive ones.
- Backup and restore in the UI, tested on a real copy of a practice database.
- Reports, handouts and summaries printed without manual fixes.
- Every "Next" item on this roadmap done or moved to a later release on purpose.
- One full release cycle (four weeks) on the live practice instance without a
  data-loss incident.

After 1.0.0 the same table applies with the major digit in front: features bump
`1.<minor>.0`, fixes `1.<minor>.<patch>`, breaking changes `2.0.0`.

## Release process

1. `internal/version/version.go` bump according to the table above.
2. Format, lint and test locally the way CI does: `gofumpt -l -extra .`,
   `go vet ./...`, `golangci-lint run ./...`, `govulncheck ./...`,
   `go test -race -count=1 ./...`.
3. Commit and annotated tag `vX.Y.Z`, then `git push origin master --tags`. `origin`
   has three push URLs (own forge, GitHub, GitLab), so one push publishes to all
   of them; check that every remote reports success.
4. Watch the pipeline on the forge (tidy, format, lint, vulncheck, test, build, image).
5. CI publishes `forge.gpl-nas.de/physiobase:vX.Y.Z` and `:latest`.
6. Update both running instances from the published image – never build locally:
   `podman-compose pull && systemctl restart physio` and the same in
   `/nvme/container/physio-demo`. Then check `healthz` and the version on the login
   page of both.
