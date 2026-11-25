# Roadmap

**Last updated:** 25.11.2026

**Legend:** `[x]` done · `[ ]` planned · 🚧 currently in progress ·
*(next)* = coming in the next releases · *(mid term)* · *(long term)*

The sections group the work by kind: clinical documentation, prescriptions and
billing, reference data, practice workflow, security and operations, platform.

Guiding principles for every decision:

- One static Go binary, one SQLite file, no cloud dependency.
- Pages rendered on the server; JavaScript only where it adds real value.
- Data stays where it belongs: on your own server. Official reference data
  (ICD-10-GM, ICF, remedy catalogue, prices) is imported from official sources and
  kept locally, so no feature needs an internet connection at runtime.
- German user interface today, built so that other languages can be added.

## 🚧 In progress

- [ ] 🚧 **Test history as a chart** - score of a test over time in the patient
      profile, with trend indicator and comparison to the previous assessment

## Prescriptions, billing and invoices

- [ ] **Prescriptions as the folder for a course of treatment** *(next)*
      - Every patient has at least one prescription; a patient without a
        prescription does not exist. The first prescription is created together
        with the patient.
      - A prescription holds: date, diagnosis, treatment method (Behandlungsart),
        prescribed number of appointments/units, frequency, payer, prescribing
        doctor, validity, billed flag with date and invoice number, status and a
        note.
      - Appointments and treatment method are **optional**: a blank prescription
        ("Blankorezept") allows choosing the method and the number of units per
        appointment. The treatment method comes from the treatment types an admin
        maintains; therapists are filtered by their licence (see "Therapists,
        treatment types and licences").
      - Everything that belongs to the treatment hangs off the prescription:
        assessments, sessions, tests, documents, summaries and appointments. The
        patient page opens the newest active prescription and keeps an "all data"
        view across prescriptions.
      - Follow-up prescriptions (Verlängerung / Folgeverordnung) chain under the
        previous one (`parent_id`). The previous prescription is closed when the
        follow-up starts, whether or not appointments are left over; **leftover
        units are never carried over** - nothing may be billed that was not
        provided.
      - Unit counter ("4 of 6"), warning when the prescribed number is exceeded or
        the prescription expires.
      - The demo gets two example prescriptions including a follow-up.
- [ ] **Invoices and billing** - invoice number, amount, date and payer per
      prescription, plus an overview of open and billed prescriptions. Still open
      whether invoices are written in PhysioBase (own numbering, PDF) or handed to
      an external tool such as Firefly III; the billed state and the invoice
      reference stay in PhysioBase either way *(mid term)*
- [ ] **Remedy positions and prices per payer** - the position and price lists are
      not the same for every payer, so each gets its own:
      | Payer | Positions | Prices |
      |---|---|---|
      | GKV | Heilmittelkatalog positions (KG, MT, MLD, KG-ZNS, groups …) | Vergütung per Land (regional differs per Landesverband) |
      | BG / gesetzliche UV | UV positions 8101–8107 ff. (own catalogue) | UV-Gebührenverzeichnis |
      | Bundeswehr | GKV positions (Heilfürsorge) | bundeseinheitliche Vergütungssätze |
      | Private / self-payers | GKV positions as a starting point | own prices (GOÄ/§ 6 or practice rate) |
      Payment status and open invoices stay with the prescription. *(mid term)*
- [ ] **Contract documents and annexes, always current** *(mid term)*
      - The rules behind the catalogue are kept in the app, not just linked: the
        annexes of the § 125 SGB V contract (Anlage 1 Leistungsbeschreibung,
        Anlage 2 Vergütung, Anlage 3 notwendige Angaben auf der Verordnung,
        Anlage 4 Fortbildung, Anlage 5 Zulassungsvoraussetzung, Anlage 7 technische
        Voraussetzungen), the G-BA Heilmittel-Richtlinie with the Heilmittelkatalog,
        and the equivalents for BG/UV and Bundeswehr.
      - Each document is stored with **version, valid-from date, source URL and a
        checksum**, so the app can say what it is quoting and when it was fetched.
      - The Leistungsbeschreibung of a position is shown where the position is
        chosen (prescription, appointment, billing), together with the links to the
        official PDF.
      - **Kept current:** a scheduled check asks the official sources (gkv-heilmittel.de,
        gkv-spitzenverband.de, vdek, DGUV, Bundeswehr) whether a newer version exists
        and reports it on the admin page; the admin confirms the update, nothing is
        replaced silently. A status badge shows "current as of TT.MM.JJJJ" and turns
        red when a check has not run for longer than a month.
      - The official PDF stays the authoritative document; the app keeps the
        version reference and its own short description, no re-publication of
        protected full texts.

## Clinical documentation

- [ ] **Assessment search** - full text search across assessments (diagnosis,
      notes), not only across the knowledge base *(next)*
- [ ] **Report export** - medical report / summary as a clean PDF with practice
      logo and header *(next)*
- [ ] **Handout templates** - save sets of exercises as a template and apply them
      to a patient in one click *(next)*
- [ ] **Attachments for tests and assessments** - e.g. photo of the test setup or
      a measurement report as PDF *(mid term)*
- [ ] **Questionnaire by link** - the patient fills in a test in the browser
      before the visit (one-time link, no account) *(mid term)*

## Reference data and standards

- [ ] **Official code lists via official APIs** - import the ICD-10-GM (WHO, BfArM),
      ICF, OPS, ATC, the German remedy catalogue (Heilmittelkatalog) and remedy
      prices from the official interfaces (WHO APIs, BundesAPI and the German
      health-data endpoints) as versioned lists: searchable pickers in assessment,
      diagnosis, prescription and billing forms, with the catalogue version stored
      on every record so old assessments keep their codes.
      The lists are fetched on demand and stored locally - the practice keeps
      working offline and never depends on a cloud service at runtime. ICD-10-GM
      and OPS start a new catalogue year on 1 January: the admin gets a reminder,
      the new catalogue is imported alongside the old one, and records keep the
      version they were coded with. *(mid term)*
- [ ] **ICD-10 / ICF codes** - structured diagnosis and functioning codes on the
      assessment, searchable and usable for statistics. The diagnosis field of a
      prescription is prepared for this. *(mid term)*

## Practice workflow

- [ ] **Therapists, treatment types and licences** *(next)*
      - Own admin page listing every therapist with the treatment types they are
        licensed for (matrix of therapists × treatment types).
      - Treatment types are maintained by the admin, not hardcoded: name,
        abbreviation (MT, PNF, Bobath, MLD …), whether a licence is required, and
        whether it is a group therapy. Common German GKV qualifications are
        pre-filled (see below).
      - Optional per licence: certificate number and valid-until date, so an
        expiring qualification is visible in time. Expired licences can still be
        recorded but are shown as expired.
      - **The licence requirement is a property of the payer/contract, not of the
        therapist.** Verified against the current rules:
        | Payer | Licence required? | Basis |
        |---|---|---|
        | GKV (statutory) | **yes** | GKV Rahmenvertrag § 125 SGB V; certificate positions for MT, MLD, KG-ZNS … |
        | BG / gesetzliche Unfallversicherung | **yes** | UV-Vereinbarung: own positions 8101–8107, but the same certificate positions (MT 260 h, Bobath/Vojta/PNF 120 h for 8103). Missing qualification = no claim to payment. |
        | Bundeswehr (Heilfürsorge) | **yes** | Truppenarzt prescription, billed at the uniform federal rates (vdek rates) to the Heilfürsorge office; vdek admission and the GKV catalogue apply, so certificate positions count. |
        | Private patients, self-payers, Beihilfe | **no** | no Rahmenvertrag binding |
        The admin can flip the flag per payer, because the contracts change.
      - Booking: choosing the treatment type filters the therapist list to those
        licensed for it (also in the prescription form when the treatment method is
        set per appointment); an expired licence does not count. If nobody is
        licensed, the form says so instead of silently offering everyone. Bookings
        for payers without licence requirement keep the full list, and the
        therapist's licences are shown as information.
      - Existing appointments keep their therapist; nothing is rewritten in
        hindsight.
      - Seed list (admin can edit all of it): Manuelle Therapie (MT), Manuelle
        Lymphdrainage (MLD), Komplexe physikalische Entstauungstherapie (KPE),
        KG-ZNS (Bobath, PNF, Vojta), KG-ZNS Kinder / Kinder-Bobath, Atemtherapie,
        Physio-Akupunktmassage (APM), Funktionelle Bewegungslehre (FBL),
        Rückenschule/KGG (group), plus licence-free positions such as klassische
        Massage, Wärme-/Kältetherapie, Elektrotherapie and Inhalation. Private
        extras (osteopathy, craniosacral, sports physiotherapy) are added by the
        practice itself.
- [ ] **Recall list** - patients whose next appointment or follow-up assessment is
      due, with a reminder function *(next)*
- [ ] **Consents and signatures** - capture forms digitally and store them in the
      prescription *(mid term)*
- [ ] **Appointment reminders** - e-mail reminder before the appointment, only
      with documented consent *(mid term)*
- [ ] **Patient self-booking** - public calendar link for appointment requests,
      confirmed by the practice *(mid term)*
- [ ] **Dashboard** - case numbers, utilisation, open prescriptions, due
      follow-ups, no-show rate *(mid term)*

## Security, data protection and operations

- [ ] **Audit log** - who changed or viewed which assessment and when (GDPR
      accountability, useful for handovers) *(next)*
- [ ] **Backup and restore in the UI** - back up, download and restore without a
      shell, including the status of the last backup *(mid term)*
- [ ] **Opt-in encryption** - encrypted database and media (passphrase at start),
      encrypted backups, optional field-level encryption for particularly
      sensitive data *(long term)*
- [ ] **Retention periods and deletion concept** - configurable deadlines,
      reminders for archiving and a deletion log *(long term)*

## Platform and engineering

- [ ] **Guided first start** *(next)*
      - A setup wizard on the very first start instead of `.env` editing: language,
        admin account (password or passkey), practice data and logo, time zone,
        backup count, appointment defaults; e-mail settings stay in the admin area
        so nothing half-configured is left behind. Every step except language and
        admin can be skipped, and the whole thing is reachable again from the admin
        area afterwards.
      - **Two layers, clear precedence:** the environment (compose/.env) provides
        defaults, the database holds the live configuration. A value set in the
        wizard or the admin area wins over `env`; a restart is not needed to change
        it. The environment is never rewritten by the app - instead the admin page
        shows where a value comes from ("env" or "database"), offers "reset to the
        env value", and can export the current settings as an `.env` snippet for
        documentation or a redeploy.
      - Replaces the `hash-password` step in the installation docs; the
        `PHYSIOBASE_USERNAME`/`PASSWORD_HASH` bootstrap keeps working for automated
        deployments and skips the wizard.
      - **Security:** the setup route is only reachable while no user exists and
        closes with a hard 404 afterwards; protected by a one-time token printed to
        the log, or by `PHYSIOBASE_SETUP_TOKEN` from the compose file; POST-only,
        `no-store`, rate limited; the token is deleted on completion. The README
        tells people to run the first start before exposing the instance.
      - The language step is the first page (German/English buttons, preselected
        from `Accept-Language`), so a new installation never needs an env variable
        just to be readable.
- [ ] **Internationalisation** - the project must be translatable into other
      languages: externalise all UI strings and document templates (German as the
      default, English and Dutch next - the KNGF guidelines are published in
      Dutch), per-patient output language for questionnaires, and a documented
      translation workflow so the community can contribute languages *(long term)*
- [ ] **Other therapy professions** - besides physiotherapy also occupational
      therapy, speech therapy and further remedy professions, with separate
      assessment schemas, test collections and handouts per profession *(long term)*
- [ ] **Multi-tenancy** - several practitioners with separate data sets in one
      instance *(long term)*
- [ ] **Offline-capable UI (PWA)** - record assessments in the treatment room
      without a stable Wi-Fi connection and synchronise later *(long term)*
- [ ] **Analytics** - anonymised statistics over your own caseload *(long term)*
- [ ] **Import from other systems** - CSV/Excel for patient master data and
      digitisation of paper files *(long term)*
- [ ] **Telematics infrastructure (TI)** - connector and KIM for the secure
      exchange of medical letters and findings, card terminals (eHBA / SMC-B) and,
      as far as available for remedy providers, electronic prescriptions *(long term)*

## Done

- [x] ~~**Faster test search and deleting from lists** - clicking the search field
      lists every test, typing filters letter by letter, arrow keys and Enter pick
      one; entries can be deleted from the knowledge list and appointments from the
      overview, each with the confirmation step (0.12.0)~~
- [x] ~~**Free assessments included** - 13 free, evidence-based instruments (Berg
      Balance Scale, Mini-BESTest, FAC, Trunk Control Test, TUG, 10 m walk test,
      6MWT, 30 s chair stand, 5xSTS, 4-stage balance, functional reach, SPPB, PSFS)
      with scale, cut-offs and reference; licensed instruments stay with the
      practice. Pain (NRS) and PSFS already live in the assessment form.
      `physiobase seed-assessments` adds them to an existing database (0.13.0)~~
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
6. `scripts/wiki-sync.sh` mirrors `ROADMAP.md` into the GitHub wiki.
7. Update both running instances from the published image – never build locally:
   `podman-compose pull && systemctl restart physio` and the same in
   `/nvme/container/physio-demo`. Then check `healthz` and the version on the login
   page of both.
