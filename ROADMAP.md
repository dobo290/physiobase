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

- [ ] 🚧 **Home page as dashboard, navigation in a collapsible sidebar** *(in progress)*
      - Today the start page is the knowledge list, and the other areas are only
        reachable through the top navigation. The knowledge base moves to `/wissen`
        (old links redirect), and `/` becomes a dashboard.
      - **Persistent left sidebar** with the areas (patients, knowledge, tests,
        handouts, appointments, account), collapsible and remembered per device
        (cookie), on narrow screens a drawer with a single toggle. Server-rendered
        like the rest, no framework.
      - **Main element is the appointment calendar** (own item below): the day plan
        with one column per therapist. Around it: patients seen last week, open
        assessments without a follow-up, knowledge entries changed recently, a direct
        search field and the two or three most common actions (new patient, new
        assessment, new entry).
      - Keeps the existing "Dashboard" item meaningful: that one adds figures
        (case numbers, utilisation, no-show rate); this one is the shell, the
        navigation and the daily overview.
- [ ] 🚧 **Appointment calendar: one column per therapist** *(in progress)*
      - A day (and week) view on the dashboard with one column per therapist - the
        practice's wall planner, not a list. Time axis with a configurable grid
        (5 or 10 minutes), appointments as blocks showing time, patient and treatment
        type, cancelled ones struck through.
      - **Appointments are editable in place**: click opens an editor next to the
        calendar, drag moves, dragging the lower edge changes the duration, snapping
        to the grid. Every change is saved through the same server action the
        appointment page uses, so the two views can never disagree; an alternative
        keyboard/form path stays, because drag and drop must not be the only way.
      - **Linked to prescription and patient**: a block opens the patient page, the
        prescription the appointment belongs to, or the assessment if one exists;
        creating an appointment from a prescription pre-fills treatment type,
        duration, therapist and payer (licence filter for statutory patients), and
        the units counter of the prescription updates on completion.
      - **Conflicts are visible before saving**: overlapping appointments in the same
        column, a therapist who is not licensed for the treatment type (statutory
        patients), a closed or expired prescription. Overriding needs an explicit
        confirmation, so a deliberate exception stays possible.
      - With many therapists the columns cannot all fit: filter by team, location or
        "my column only", scroll horizontally, and keep the calendar usable on a
        tablet. Optimistic locking (own item) prevents two people overwriting the same
        appointment.
      - Print view of the day plan per therapist or for the whole practice, plus the
        existing patient calendar feed staying as it is.
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

- [ ] **Modular assessments: schema as data, templates from the community** *(long term)*
      - Today the assessment form is Go code (`internal/befund/schema.go`): sections,
        fields, tables and symbols are fixed in the binary, and form, print view and
        comparison all derive from it. That makes the form reliable, but a practice
        cannot change it and a second profession cannot use it.
      - **The schema becomes data**: a declarative file (JSON, human-readable, diffable)
        with sections, fields, tables, symbols, validation rules and print layout.
        The built-in form is extracted into that format and shipped as the default
        template, so nothing changes for existing practices; a new **template version
        is pinned to every assessment**, so old findings keep rendering with the
        schema they were filled in (same idea as the catalogue versions for ICD codes).
      - **The assessment becomes runnable on its own**: one static executable per
        platform (the existing Go binary with a `standalone`/`builder` mode, plus a
        WASM build for the browser) that opens a template, records a finding offline
        and exports JSON or PDF. Usable without PhysioBase - for a single assessment,
        for teaching, or by a profession that does not run the full application.
      - **Two ways to author a template**: the admin area in PhysioBase (sections and
        fields arrange, reorder, preview, export) and the standalone builder, which
        produces the identical file. No HTML, no code - templates are data, so a
        template can never execute anything.
      - **Template registry**: a table in the database plus an index repository that
        lists installable templates. A template is a small package with a manifest
        (id, name, version, profession, language, licence, author, minimum app
        version) and the schema file. Install from URL, from a git repository or from
        a file upload; list, pin, update and remove in the admin area; the practice's
        choice of default template is per practice, optionally per patient.
      - **Distribution paths, all three supported**: install it only into one's own
        instance, publish it as its own repository, or publish it into the community
        index so other practices and other professions find it. Licences are declared
        per template (CC0, CC-BY, AGPL …) because clinical content is shared, not
        sold.
      - **Opens the project to other professions**: templates carry a profession and a
        language, so occupational therapy, speech therapy, podiatry, nursing or a
        medical practice can use the same engine with their own form - the reason this
        replaces the "other therapy professions" wish rather than duplicating it.
      - **Data model in SQLite** - three tables, no per-template tables and no
        rewriting of patient data:
        `befund_templates` (identity: id, name, profession, language, licence,
        author), `befund_template_versions` (immutable rows: template_id, version,
        schema JSON, manifest JSON, checksum, source, created_at, retired_at) and a
        new column `befunde.template_version_id` with `ON DELETE RESTRICT`.
        Rules that follow from it: a new or edited template is always a **new
        version row** (nothing is overwritten); every assessment **pins the version**
        it was filled in, so `befunde.data` stays untouched and the version only
        decides labels, print layout and comparison; **deleting means setting
        `retired_at`**, so it disappears from the pickers but stays resolvable for
        old findings; and a hard delete is refused by the database itself as long as
        a finding references that version (`RESTRICT`). A manual admin cleanup may
        remove **unreferenced** versions only - never automatically, because of the
        retention duties.
      - **Default template**: one setting per practice, plus an optional
        `patients.template_version_id` for patients who need a different form for
        good. The community index is a git repository with a JSON catalogue and is
        **not** part of the database: installing copies schema, checksum and source
        URL into `befund_template_versions`, after which the instance works offline.
        Size is negligible (50 templates x 5 versions x 20 KB ~ 5 MB next to a
        multi-gigabyte patient database).
      - **Migration**: schema keys stay stable, old assessments keep their pinned
        version, and the comparison view compares only fields that exist in both
        versions (showing the rest as "not part of that version"). Migration 12 adds
        the two tables and the column, inserts the built-in schema as version 1 of
        `physio-standard` and backfills every existing finding with it, so behaviour
        does not change on upgrade.
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

- [ ] **Patient master data: normalise the date of birth** *(next)*
      - The field accepts a date, a year or an age and stores it as free text, so an
        age of "45" stays 45 forever and quietly corrupts the record over the years.
      - Store a real date (ISO) plus the precision that was entered (day, year or age
        with the date it was recorded), show it with `dobLabel`, and migrate existing
        values with a one-time best-effort pass that flags what it could not parse.
- [ ] **Merge duplicate patients** *(mid term)*
      - Imports and typos create duplicates; merging today means moving assessments,
        sessions, documents, tests and appointments by hand.
      - A merge that moves all child records to the keeper, keeps a note about the
        merged name, and is logged; tested with a patient that has assessments,
        sessions and documents.
- [ ] **Undo for deletions** *(mid term)*
      - Deleting an assessment, test or document is final today. A trash with a
        short retention (for example 30 days) and a restore action makes mistakes
        reversible without weakening the ten-year retention rules.
- [ ] **Knowledge base editor role** *(next)*
      - A user who works on the knowledge base and nothing else: reads and writes
        entries (conditions, tests, assessments, exercises, protocols, notes),
        uploads their media, maintains sources, tags and body regions - and has **no**
        access to patients, assessments, appointments, billing or administration.
      - Completes the role set next to `admin`, therapist (`user`) and reception:
        `admin` -> `therapist` -> `reception` / `editor`. Each role gets a fixed set of
        routes, enforced server-side and listed in the route matrix test; the
        navigation only shows what the role may open.
      - Useful for practices that let students, guests or a colleague maintain the
        knowledge base without exposing health data.
- [ ] **Reception / assistant role** *(next)*
      - Only `admin` and `user` exist. A front desk needs appointments and patient
        master data but must not read assessments: its own role with a narrow set of
        allowed routes, enforced server-side and covered by the route matrix test
        (the same mechanism as the knowledge base editor role).
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
        cookies exist (the consent cookie and, only after consent, the statistics
        cookie), and that no external fonts, CDNs or analytics services are used. Templates with the required fields are pre-filled so a practice
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
      - **Decided scope:** own domain published through the reverse proxy (separate
        port, the application port stays LAN-only); pages Start, Team, Services,
        Contact, Imprint and Privacy, with logo and team photos uploaded in the
        admin area; a contact form that sends by SMTP (honeypot + rate limit, no
        storage in the database, and a clear note that medical details do not belong
        in the form).
      - **Statistics have their own admin page** (`/admin/website/statistics`):
        visitors and page views per day as small charts, top pages, referring
        domains, share of visitors who accepted the cookie, and the addresses that
        ended in a 404. Charts are server-rendered SVG - no JavaScript and no chart
        library, matching the rest of the application.
      - The demo instance runs the website as well and shows the statistics page with
        seeded demo figures; the demo reset clears them together with the pages.
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

## Interoperability

- [ ] **Practice management coexistence** *(mid term)*
      - Import patients and prescriptions from a practice management system
        (CSV/JSON first, then GDT/BDT), export assessments and sessions, and keep the
        calendar in sync (ICS export exists, CalDAV import follows).
      - Aim: data exchange in both directions, so a practice can document in
        PhysioBase whatever else it runs - and leave again without losing anything.
      - Calendar and messaging: CalDAV import, iCal export (exists), later e-mail and
        patient messaging. Every interface is documented in the repo and covered by a
        test with a real exchange file.

- [ ] **Smart home controls inside PhysioBase** *(mid term)*
      - Direction is **one way into PhysioBase**: it reads device states and sends
        control commands. **No practice or patient data leaves PhysioBase** - the
        smart home system only ever sees the entity that is being switched.
      - **Provider-neutral core, not one vendor**: an internal model for switchable
        things (on/off, dimmable, thermostat, read-only sensor) with adapters behind
        it. The practice view renders controls from the model, so a new provider does
        not touch the UI.
      - **Adapters, in this order**:
        `Home Assistant` (REST + long-lived token; covers most systems the practice
        already has), `Homey` (local Web API with a personal access token), and
        **generic MQTT** (topics configured per entity - works with Homey, openHAB,
        ioBroker, Node-RED, Tasmota, Shelly and anything else that speaks MQTT).
        openHAB and ioBroker REST adapters only when somebody actually asks.
      - **Admin**: provider URL/credentials plus a **whitelist/allow-list of entities**
        (light, switch, thermostat, sensor) with a display name and a room. Anything
        not on the list cannot be reached from PhysioBase; a provider can be switched
        off entirely.
      - **Practice view** (dashboard panel or its own page): plain controls - on/off,
        brightness, target temperature - and read-only readings such as room or cellar
        temperature. Server-rendered, small refresh for readings; credentials and all
        HTTP calls stay on the server.
      - **Failure stays local**: provider unreachable means the panel says so; the rest
        of the practice software keeps working untouched.
      - Out of scope on purpose: no events or data going to the smart home system, no
        appointment or patient data, no automatic device control from practice events.
        Whoever wants that later needs a separate decision.
      - **Tests**: fake servers per adapter (state read, command, unreachable host,
        bad token), the allow-list enforced server-side, and one adapter-independent
        test that the UI renders from the model.

## Security, data protection and operations

- [ ] **Compliance roadmap** *(next/mid/long, see the document)* - the data
      protection and medical items (ten-year retention, deletion and audit log,
      patient data export, privacy notice, incident process, consent management,
      backup/restore, per-patient access rights, encryption, retention concept)
      are tracked in [docs/compliance.md](docs/compliance.md). That document is the
      compliance roadmap: status, priority and legal basis per item.

## Platform and engineering

Engineering, scaling and operations (database limits, data volume and archiving,
disaster recovery, the PostgreSQL option, upgrade guards, backup integrity, hosting
and sizing) are tracked in [docs/hosting.md](docs/hosting.md). What stays here are the
product and code-quality items.

- [ ] **Paginate the patient page** *(next)*
      - Assessments and sessions are loaded without a limit; ten years of weekly
        treatment is hundreds of rows per page load, plus documents and tests.
      - Page or lazy-load both lists (for example 20 at a time) and keep the print
        view complete.
- [ ] **Browser end-to-end tests** *(long term)*
      - The body chart, the test search and keyboard handling are only tested at HTTP
        level. A small Playwright (or chrome-devtools) suite for those three flows
        catches JavaScript regressions before a practice does.
- [ ] **Concurrent editing (optimistic locking)** *(next)*
      - Today the last save wins. With several therapists per patient (locum,
        handover, shared front desk) that silently loses work.
      - A version counter per patient, assessment, prescription and summary; a save
        on an outdated version is refused with a clear "someone else changed this,
        reload and merge" instead of overwriting.
      - Tested with two sessions editing the same record.
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
- [ ] **JSON API for companion clients** *(mid term)*
      - Prerequisite for any mobile client: documentation and knowledge base over a
        versioned JSON API instead of scraping the rendered pages.
      - Per-device tokens (named, revocable, scoped to documentation and knowledge
        base), issued in the account page, hashed at rest, listed with last use and
        revocable individually; the existing session version invalidates them too.
      - Endpoints needed by the first client: search knowledge base, read entries,
        list patients, read and write assessments and sessions, upload a document
        photo, list tests and record a test run. Everything else stays web-only.
      - Contract first: OpenAPI file in the repo, generated server bindings checked
        against it, and a test that fails when an endpoint changes behaviour without
        the contract being updated. Rate limits and audit logging apply as on the web.
- [ ] **Mobile client for therapists (Android/iOS)** *(long term)*
      - Scope is deliberately small: **documentation and knowledge base**. No admin
        area, no billing, no user management, no website editing.
      - **First step is the PWA** (see "Offline-capable UI"), not an app store
        release: installable, works offline, one codebase, no review process. It
        already covers "look up a test, write the session, photograph the referral"
        on a phone or tablet.
      - **Native shell only where it earns its keep**: camera for documents,
        biometric unlock, local encrypted cache, later push for reminders. Then
        Kotlin Multiplatform or Flutter over the same JSON API - one codebase for
        both stores, AGPL-3.0 like the server (the network clause is unaffected, the
        app ships source).
      - **Data protection on the device** is part of the feature, not a footnote:
        tokens in Keychain/Keystore instead of local storage, the offline cache
        encrypted and excluded from iCloud/Google backups, remote logout through the
        existing session version bump, no analytics or crash SDKs, and a "clear local
        data" action. A lost phone is a data protection incident, so the app must
        support wiping without the practice's server.
      - **Conflicts**: the same optimistic locking as the web (version counters);
        offline edits are queued and conflicts are shown, never silently overwritten.
      - Costs to accept: Apple Developer Program (yearly fee, App Store review) and
        Google Play (one-time fee); both stores require a privacy declaration and a
        data-safety form - the compliance document gains a section for the mobile
        client.
- [ ] **Internationalisation** - the project must be translatable into other
      languages: externalise all UI strings and document templates (German as the
      default, English and Dutch next - the KNGF guidelines are published in
      Dutch), per-patient output language for questionnaires, and a documented
      translation workflow so the community can contribute languages *(long term)*
- [ ] **Other therapy professions** - besides physiotherapy also occupational
      therapy, speech therapy, podiatry, nursing and medical practices. Solved through
      the modular assessment templates (see "Clinical documentation"): own form,
      test collection and handouts per profession, chosen per template. *(long term)*
- [ ] **Multi-tenancy** - several practitioners with separate data sets in one
      instance *(long term)*
- [ ] **Installable web app (PWA, stage 1)** *(next)*
      - `manifest.webmanifest` with name, short name, start URL, `display: standalone`,
        theme and background colour; icons in 192 and 512 px plus `apple-touch-icon`
        (180 px), generated from the practice logo; `theme-color` and the
        `apple-mobile-web-app-*` meta tags in the layout.
      - A one-line hint on the login page ("add to home screen" on iOS, install button on
        Android) and `worker-src 'self'` in the content security policy so stage 2 can
        register a service worker later.
      - **No offline behaviour yet** - the app becomes installable and starts without
        browser chrome on phone and tablet, which is the visible step. Stage 2
        (service worker, offline capture queue, conflict handling) stays in the
        "Offline-capable UI (PWA)" item below.
- [ ] **Offline-capable UI (PWA)** - record assessments in the treatment room
      without a stable Wi-Fi connection and synchronise later. Stage 1 makes the app
      installable (own item), stage 2 adds the service worker, an offline queue for
      entries and the conflict handling on top. This is also the first mobile client:
      installable, no app store, and the groundwork the native app builds on (see
      "Mobile client for therapists"). *(long term)*
- [ ] **Analytics** - anonymised statistics over your own caseload *(long term)*
- [ ] **Import from other systems** - CSV/Excel for patient master data and
      digitisation of paper files *(long term)*
- [ ] **Telematics infrastructure (TI)** - connector and KIM for the secure
      exchange of medical letters and findings, card terminals (eHBA / SMC-B) and,
      as far as available for remedy providers, electronic prescriptions *(long term)*

## Done

- [x] ~~**Installable web app (PWA, stage 1)** - `/manifest.webmanifest` served with the
      practice name, short name, standalone display and theme colour; icons for 192,
      512 and 180 px generated on the fly from the uploaded logo (PNG/JPEG/WebP) or as a
      neutral mark, plus `theme-color`, `apple-touch-icon` and
      `apple-mobile-web-app-*`; a "add to home screen" hint on the login page;
      `worker-src` and `manifest-src` in the CSP so stage 2 can register a service
      worker; manifest and icons reachable without a login (0.14.0)~~
- [x] ~~**Wording in the inspection section** - the term "Haltung" is scientifically
      contested: static alignment is a weak predictor of pain, inter-rater agreement
      of postural judgements is moderate at best, and "posture faults" as an
      indication are not supported (Slater et al., "Sit Up Straight: Time to
      Re-evaluate", JOSPT 2019; Lederman, "The fall of the postural-structural-
      biomechanical model", 2011). The field now says "Alignment" and the
      "Subjektive Wahrnehmung (Suggestiv)" field asks for the impression from the
      patient's report **and** from the inspection (0.13.8)~~
- [x] ~~**Assessment form polish** - section "Assessments - Notizen" renamed to
      "Tests/Assessments"; the header prints an age entered as "45" as "45 Jahre"
      (date and year keep the "geb." label, one `dobLabel` helper for screen and
      print); the printout drops table columns without values (empty "Subtyp (nur
      nozizeptiv)" disappears, the input form and the comparison keep all columns);
      inspection gained a field "Subjektive Wahrnehmung (Suggestiv)"; the yellow
      flag "Niedergeschlagenheit, Angst, Stress" became "Depressive/pessimistische
      Symptomatik" and the yellow flags gained their own "Bewertung & Vorgehen"
      field like the red flags (0.13.7)~~
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
