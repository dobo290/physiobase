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

- [ ] **Public website for patients** *(mid term)*
      - One extra listener in the same binary and container
        (`PHYSIOBASE_SITE_PORT`, empty = off) serving a small, read-only site.
        In the compose file it is its own port, published through the reverse proxy
        under the practice's own domain; the application port stays LAN-only.
      - **Content is edited in the admin area**, no HTML and no file editing:
        pages (slug, title, markdown body, position in the menu, published yes/no),
        plus the practice data that already exists (name, address, phone, e-mail,
        logo) and opening hours.
      - **Legal pages are part of it, not an afterthought.** An "Impressum" page with
        the fields required by § 5 DDG (name, address, contact, professional title,
        the authority that granted it, supervisory body, chamber where applicable,
        VAT ID where applicable) and § 18(2) MStV (person responsible for content),
        plus a privacy page for the website itself: what the server logs, which
        cookies (none for visitors), and that no external fonts, CDNs or analytics
        are used. Templates with the required fields are pre-filled so a practice
        only has to complete them.
      - **Visitor statistics for the practice, with consent.** Reuse the pattern the
        demo already uses: one first-party cookie, a small banner that explains it,
        and counting only after the visitor accepts. Nothing is measured before
        that, reject is as easy as accept, and the choice is remembered.
      - What is measured stays coarse and local: page views per day, the most
        visited pages, the referring domain (not the full URL), 404s and the share
        of visitors who accepted. No third-party service, no cross-site ID, no
        profile building; the visitor is counted through a hash that rotates daily,
        so nobody is recognisable across days. Retention 30 days by default, purge
        automated, and the figures are only visible in the admin area.
      - The privacy page is generated from the same settings and spells out exactly
        what is measured - it cannot drift away from the implementation.
      - **No external requests at all**: self-hosted CSS, fonts and images, no CDN,
        no analytics service. The only cookie is the consent one and the optional
        statistics cookie.
      - For practices that do not want a banner, the statistics can be switched off
        completely (`PHYSIOBASE_SITE_STATS=off`, the default); the site then sets no
        cookies at all.
      - Basic search engine hygiene: `<html lang>`, title and description per page,
        canonical URL, `robots.txt`, `sitemap.xml`, Open Graph tags, server-rendered
        HTML without JavaScript.
      - **Separate from the application**: its own router, its own templates and its
        own layout; no session cookie, no admin routes, no patient data on that port
        - enforced by a test that walks both routers and asserts the public one only
        reaches public handlers.
      - 404 for unpublished pages, soft-delete for pages, "preview" in the admin
        while a page is still a draft.
      - Later hooks into the roadmap: the appointment request ("Patient
        self-booking") and the questionnaire link are linked from the site instead
        of being built into it.
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

- [ ] **Compliance roadmap** *(next/mid/long, see the document)* - the data
      protection and medical items (ten-year retention, deletion and audit log,
      patient data export, privacy notice, incident process, consent management,
      backup/restore, per-patient access rights, encryption, retention concept)
      are tracked in [docs/compliance.md](docs/compliance.md). That document is the
      compliance roadmap: status, priority and legal basis per item.

## Platform and engineering

- [ ] **Stability for practices with up to 250 therapists** *(next)*
      - Target, not wishful thinking: one instance, one database, 250 therapists
        working 40 h/week in 20-minute slots at the same time.
      - Load that this means, derived from the slot model: 250 × 40 h = 10,000
        therapist-hours per week → **30,000 appointments per week**, 6,000 per day,
        ~1.4 million per year. Writes average **0.2 per second** with bursts of a
        few hundred when slot boundaries align; reads reach **100–200 requests per
        second** in reality (theoretical peak 500), each page costing 10–40 queries.
      - **Baseline measured today** on the reference host (SQLite + WAL, modernc
        driver): 23,000 inserts/s and 29,000 reads/s with a single connection, no
        errors while writing and reading in parallel. The load above therefore fits
        the engine by a factor of 10–500; the work is in connection handling,
        indexes and UI concurrency, not in replacing the database.
      - Work items: **separate pools** (one writer, 4–8 readers, WAL allows parallel
        readers), **indexes** for the real access patterns
        (`appointments(therapist_id, start)`, `prescriptions(patient_id, status)`,
        `sessions(patient_id, date)`, `befunde(patient_id, date)`), batching instead
        of per-row queries, and pagination everywhere.
      - **Load test in the pipeline**: seed a database with 250 therapists and ten
        years of data, replay appointment booking and patient-page reads, hold
        p95 below 300 ms and keep it as a regression gate.
- [ ] **Concurrent editing (optimistic locking)** *(next)*
      - Today the last save wins. With several therapists per patient (locum,
        handover, shared front desk) that silently loses work.
      - A version counter per patient, assessment, prescription and summary; a save
        on an outdated version is refused with a clear "someone else changed this,
        reload and merge" instead of overwriting.
      - Tested with two sessions editing the same record.
- [ ] **Data volume and archiving** *(mid term)*
      - Ten years of records for 250 therapists is roughly **10–20 GB**. Daily
        snapshots (`VACUUM INTO`) take seconds on NVMe but minutes over the NAS,
        and `VACUUM` grows with file size.
      - Move records past their retention period into a **read-only archive database
        file** (opened on demand), keep the live database small (target: below 5 GB),
        and stage backups on NVMe before they are copied to the NAS.
      - Admin view: database size, last backup duration, archive state.
- [ ] **Disaster recovery on other hardware** *(mid term)*
      - Assumed and already in place: **UPS, redundant power supply, RAID** on the
        server, btrfs with snapshots on the NAS. Power loss and a single failing disk
        are therefore covered; the open risks are OS or container corruption, human
        error, a compromised host, several disks failing at once, and losing the room
        itself (water, fire, theft).
      - The server-side recommendations (sizing, network, backup, hardening, what a
        hosting provider has to know) are in `docs/hosting.md`; this item is the
        practice-side runbook that goes with it.
      - A written runbook: what the practice does in the first hour, who is called,
        how the container comes up on spare hardware from the latest snapshot, how
        DNS or the proxy is pointed there, and a **measured restore time** (RTO) plus
        the accepted data loss (RPO) in plain numbers.
      - Rehearsed restore on a copy, once per quarter, with a short report.
      - **Off-site copy**: today every backup lives in the same house as the server,
        so a fire or a flooded cellar takes both. Add an encrypted copy that leaves
        the building (rotating disk, second location over the tunnel) and document
        how it is restored. When the instance itself runs at a provider (see
        `docs/hosting.md`, section 9), the copy belongs at the practice instead.
      - Shorten the RPO where it is cheap: database snapshots hourly (they take
        seconds), keep hourly for a week, daily for a month.
- [ ] **Operations for larger practices** *(mid term)*
      - Runbook for the practice: what to check, how to restore, who is called.
      - Health and metrics endpoint (database size, backup age, failed logins,
        request latency), alerting when a snapshot fails or the database crosses a
        size threshold, and a rehearsed restore on a copy of a real database.
- [ ] **Optional PostgreSQL backend** *(long term, only on demand)*
      - Decision criteria, so the question stays factual: **several hosts sharing
        the same database** (relevant when one instance serves many practices), high
        availability across machines, real-time reporting across practices, or write
        loads beyond a few hundred per second. Access from another location does
        **not** need it - a tunnel or a reverse proxy is enough, and for 250
        therapists in one practice neither criterion applies.
      - Keep the store behind its current API so the engine stays swappable; the
        migration would touch placeholders, upserts, `unixepoch()`,
        `pragma_table_info`, `VACUUM INTO` and FTS5 (→ `tsvector` + GIN), plus a
        second variant of every migration and a test matrix for both engines.
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

- [x] ~~**Faster test runs and testable logic** - `t.Parallel()` in all web tests
      plus `-p`/`-parallel` in the pipelines: the suite dropped from ~170 s to ~25 s.
      `patientTests` and `questionnaireHistory` were split into pure functions
      (`mergeTestResults`, `buildQuestionnaireHistory`, `applyResult`, `newerThan`)
      with their own unit tests, including the date rule (a newer assessment beats a
      single run, the same date stays with the single run) and corrupt data. A
      `Makefile` wraps the container-based checks (`make test|check|fuzz|coverage`)
      (0.13.3)~~
- [x] ~~**Test suite without artificial waits** - the brute-force pauses after a failed
      attempt and the bcrypt cost are configurable (`PHYSIOBASE_TEST_NO_DELAY`,
      `PHYSIOBASE_BCRYPT_COST`), set by the test suite itself; production keeps the
      real values. `make test` now runs in about 20-25 s (0.13.4)~~
- [x] ~~**Test hardening and reproducible image** - migration test from every
      intermediate schema version, backup/restore round trip with
      `integrity_check`, five fuzz targets for input that comes from outside
      (assessment JSON, questionnaire schemas, legacy import files, TOTP codes),
      SMTP delivery against a fake server (which caught `a@b` being accepted as an
      address), route/permission matrix for 15 routes and a 404 test for unknown
      ids; CI now runs `-shuffle=on`, writes a coverage report as an artifact,
      runs the fuzz targets with a short budget, sets `go mod verify`, and treats
      `govulncheck` as blocking again; base images are pinned by digest and the
      binary builds reproducibly (`-buildvcs=false -buildid=`, verified by two
      identical builds) (0.13.2)~~
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
