# Data protection and medical requirements – current state and open points

**As of:** 25.11.2026 · Version 0.13.6

This document collects which data protection and medical requirements apply to a
physiotherapy practice, **what PhysioBase covers technically**, and **what is still
missing**. It is a technical checklist for the practice, its IT and an auditor –
**not legal advice**. The legal assessment (legal basis, record of processing,
DPIA, incident process) stays with the practice.

Under the GDPR the **practice** is the controller, not the project. PhysioBase is a
tool the practice runs itself: there is no vendor access, no telemetry and no cloud
connection.

---

## 1. Legal framework

| Law | Requirement | Affects the software |
|---|---|---|
| **Art. 5 GDPR** | Principles: purpose limitation, data minimisation, accuracy, storage limitation, integrity | Data model, deletion concept, export |
| **Art. 6, 9 GDPR** with § 22 BDSG | Legal basis, health data as a special category | Documenting the legal basis (practice), no disclosure by the software |
| **Art. 12–22 GDPR** | Data subject rights: access, rectification, erasure, restriction, portability | Access/export, correction, deletion guard |
| **Art. 25 GDPR** | Data protection by design | local, no cloud, no telemetry |
| **Art. 28 GDPR** | Processor agreements | only relevant with external hosting/backup |
| **Art. 30 GDPR** | Record of processing activities | template in section 5 |
| **Art. 32 GDPR** | State of the art: confidentiality, integrity, availability, resilience | access control, backups, encryption |
| **Art. 33/34 GDPR** | Reporting a breach within 72 hours | logs, documented process |
| **Art. 35 GDPR** | Data protection impact assessment for high risk | documented screening |
| **§ 630f BGB** | Documentation duty, **retention for 10 years** | retention period, deletion guard |
| **§ 630g BGB** | Patient's right to inspect the record | printouts, access/export |
| **§ 630h BGB** | Burden of proof when documentation is missing | completeness, immutability |
| **§ 203 StGB** | Professional secrecy | access restriction, roles |
| **SGB V, Heilmittel-Richtlinie, § 125 SGB V** | Prescription, service description, billing | prescriptions, catalogues, billing |
| **MDR (EU 2017/745)** | Medical device classification | scope in section 6 |
| **EU AI Act** | Obligations for AI features | no AI features today |

---

## 2. What is covered today

| Area | Implementation in PhysioBase |
|---|---|
| **Access control** | sign-in with password or passkey, optional TOTP as a second factor, signed session cookies verified against the database, logging out or changing the password ends all sessions |
| **Roles** | `admin` and `user`; the admin pages under `/admin` do not exist for other users (404), they are not merely hidden |
| **Brute-force protection** | limited concurrent bcrypt checks, delay after a failed attempt |
| **Transport** | runs behind an HTTPS proxy (Caddy), `Secure` cookies and HSTS when the base URL is `https://`, strict content security policy without inline scripts |
| **Data location** | one SQLite file plus a media folder on the practice's own server, no cloud service, no telemetry, no external calls in normal operation |
| **File access** | media and documents are only served to signed-in users and never cached publicly |
| **Integrity** | foreign keys with cascades, versioned migrations, tests including migration and backup tests |
| **Availability** | daily consistent snapshots (`VACUUM INTO`), rotating; database in WAL mode |
| **Demo** | separate instance without real data, reset on logout; visitor statistics only with consent |
| **Build and supply chain** | CI runs tests, linters, `govulncheck`, CodeQL, Dependabot, secret scanning, image scanning (Trivy) and builds the image reproducibly |
| **Deletion** | explicit delete action with confirmation; the cascade also removes assessments, sessions and documents; otherwise archiving (`archived`) keeps the data |

---

## 3. Compliance roadmap

This document **is** the compliance roadmap: the items below are the open points,
each with status, priority and the roadmap marker used in `ROADMAP.md`. Nothing is
deleted automatically – the law sets a minimum retention, not an expiry date.

- [ ] **Ten-year retention for medical records** *(next)* – every patient shows
      "keep until TT.MM.JJJJ" (last assessment, session or document plus
      `PHYSIOBASE_RETENTION_YEARS`, default 10); deleting before that date is
      blocked, an admin can override with a reason (Art. 17 GDPR) and the override
      is logged. **Today:** archive flag only, deletion is possible at any time.
      § 630f BGB.
- [ ] **Deletion log** *(next)* – append-only record of who deleted what, when and
      why; also covers the first part of the audit log. **Today:** nothing.
- [ ] **Audit log** *(next)* – revision-safe record of who changed or viewed which
      assessment. **Today:** only sign-in events in the server log. Art. 5(2), 32.
- [ ] **Patient data export** *(next)* – one export per patient (database record,
      assessments, documents, media) as a single file for access and portability
      requests. **Today:** printouts per document only. Art. 15, 20.
- [ ] **Privacy notice and incident process** *(next)* – templates in the
      repository: information for patients (Art. 13) and a documented 72-hour
      breach procedure (Art. 33/34). **Today:** neither exists. The patient-facing
      privacy notice belongs on the public website as well (see the roadmap item
      "Public website for patients").
- [ ] **Public website duties** *(mid term, part of the website item)* – Impressum
      (§ 5 DDG, § 18(2) MStV), a privacy page for the website itself, no external
      fonts/CDNs, and a processing agreement with the provider that publishes the
      site. Visitor statistics are allowed only with **prior consent** (§ 25 TDDDG,
      Art. 6(1)(a) GDPR): first-party cookie, banner with an equally easy reject,
      counting starts after acceptance, daily-rotating hash instead of an IP,
      30-day retention. The privacy page is generated from the same settings so the
      text cannot diverge from what the software actually does. The contact form
      carries a note that medical details do not belong there; messages are sent by
      e-mail and not stored in the database.
- [ ] **Consent management** *(mid term)* – consents with purpose, date, scope and
      withdrawal (e-mail, photos, messengers, recordings). **Today:** only the
      e-mail consent flag in the patient record. Art. 6, 7, 9.
- [ ] **Backup and restore in the UI** *(mid term)* – back up, download and restore
      without a shell, with the status of the last run and a rehearsed restore
      test. **Today:** daily snapshots, restore only via shell. Art. 32(1)(b), (c).
      Availability has two parts: **on-site** (UPS, redundant PSU, RAID, hourly and
      daily snapshots) and **off-site** (an encrypted copy that leaves the building,
      with a documented restore). Both, plus the rehearsed restore, are tracked in
      `ROADMAP.md` under "Disaster recovery on other hardware", "Data volume and
      archiving" and "Operations for larger practices".
- [ ] **Per-patient access rights** *(mid term)* – rights per patient or case and a
      locum mode instead of one global role. **Today:** admin and user only.
      Art. 32, § 203 StGB.
- [ ] **Opt-in encryption** *(long term)* – encrypted database and media
      (passphrase at start), encrypted backups, optional field-level encryption.
      **Today:** no encryption at rest, backups unencrypted. Art. 32(1)(a).
- [ ] **Mobile clients** *(long term, with the mobile roadmap item)* – a phone is a
      second processing location: tokens in Keychain/Keystore, encrypted offline
      cache excluded from cloud backups, remote logout through the session version,
      no analytics or crash SDKs, a "clear local data" action, and a documented
      procedure for a lost device (Art. 32, Art. 33 if data was accessible).
- [ ] **Retention and deletion concept per data type** *(long term)* – rules for
      assessments, documents, e-mails and logs, not only for patients.
      Art. 5(1)(e).

### Already done

- [x] ~~**Compliance documentation in the repository** – `docs/compliance.md`
      lists the legal framework, what the software covers, the gaps below, the
      practice's own duties, a record-of-processing template, the MDR scope and an
      audit checklist~~
- [x] ~~**Demo without real data** – separate instance, reset on logout, visitor
      statistics only with consent~~
- [x] ~~**No telemetry, no cloud** – all data stays on the practice's server, no
      external calls in normal operation (Art. 25)~~

## 4. Tasks of the practice (not of the software)

The software cannot replace these:

1. **Record the legal basis** for each processing operation (treatment, billing, legal duties).
2. **Keep a record of processing activities** (section 5) up to date.
3. **Document the DPIA screening**; run a full DPIA when the risk is high.
4. **Sign processor agreements** with everyone who can see data (hosting, backup, IT maintenance, tax advisor with data access).
5. **Manage access**: who has an account, who may administer, who gets deactivated.
6. **Device and network security**: disk encryption, screen lock, no access from outside without VPN, router and Wi-Fi maintenance.
7. **Incident process** for data breaches including the 72-hour deadline and who is reachable.
8. **Train staff** (confidentiality, passwords, handling printouts).
9. **Organise retention**: what still exists after ten years, and where (paper, e-mails, calendar)?
10. **Appoint a data protection officer** once the legal thresholds are met.

---

## 5. Template: record of processing activities

| Field | Example (adjust to your practice) |
|---|---|
| Processing activity | Patient treatment and documentation |
| Controller | Practice name, address, contact |
| Purposes | Treatment, documentation, billing, proof of information given |
| Data categories | Master data, health data (assessments, measurements, photos), prescriptions, invoice data |
| Data subjects | Patients, and relatives where relevant |
| Legal basis | Art. 9(2)(h) GDPR with § 22 BDSG; § 630f BGB; SGB V |
| Recipients | Payers, prescribing physicians, billing service if used |
| Third-country transfer | none (no cloud service) |
| Retention | ten years after the last treatment, then deletion |
| Technical measures | see section 2 of this document |

---

## 6. Medical device scope (MDR)

PhysioBase **documents** and **bills**; it does not diagnose, does not propose
treatment and does not raise alarms. Following the EU Commission's demarcation
(MDCG 2019-11), pure documentation software is therefore **not a medical device**.

That assessment has to be repeated as soon as features arrive that

- interpret assessments automatically or suggest treatments,
- monitor measurements against limits and notify,
- contain AI-based analysis or decision support.

The roadmap has no such feature planned; should one come, an MDR and AI Act review
belongs to it.

---

## 7. Checklist for an audit

- [ ] Record of processing activities exists and matches the practice
- [ ] Legal bases documented (treatment, billing, legal duty)
- [ ] Privacy notice is handed to patients
- [ ] Consents (e-mail, photos) are documented per patient and can be withdrawn
- [ ] Retention periods are known; deletion only happens after a check and is logged
- [ ] Access accounts complete, old accounts deactivated, admin access restricted
- [ ] Second factor active for every account with access to health data
- [ ] HTTPS in use, remote access only through a VPN
- [ ] Backups run daily, a restore has been **rehearsed once**
- [ ] Incident process for data breaches known and reachable
- [ ] Processing agreements exist with every service provider that can see data
- [ ] DPIA screening documented (result and reasoning)

---

## 8. Relation to the roadmap

This document is the **compliance roadmap**; `ROADMAP.md` only points here for the
data protection and medical part. Priorities (`next`, `mid term`, `long term`) use
the same scale as the main roadmap, so both stay readable side by side.

Items that are already covered are listed under "Already done" in section 3. New
legal requirements are added here first, then implemented as part of a release.
