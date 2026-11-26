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

## 3. Open points (gaps)

| # | Requirement | Today | Missing | Roadmap |
|---|---|---|---|---|
| 1 | **Ten-year retention** (§ 630f BGB) | archive flag, but deletion is possible at any time | retention date per patient, "keep until", **deletion guard**, deletion log | *next* |
| 2 | **Access and portability** (Art. 15, 20) | printouts per patient (assessment, summary, appointments) | one export "all data of this patient" as a file | open |
| 3 | **Erasure and restriction** (Art. 17, 18) | deleting is possible, but without checking retention duties | justified exception for erasure requests, logged | *next* (with 1) |
| 4 | **Audit log** (Art. 5(2), 32) | only a server log with sign-in events, not queryable | revision-safe log: who changed or viewed what | *next* |
| 5 | **Encryption** (Art. 32(1)(a)) | no encryption at rest, backups unencrypted | database and backup encryption with a passphrase | *long term* |
| 6 | **Restore** (Art. 32(1)(b), (c)) | backups exist, restore only via shell | restore in the UI, documented restore test | *mid term* |
| 7 | **Record of processing activities** (Art. 30) | not present | template and filling aid (section 5) | open |
| 8 | **Impact assessment** (Art. 35) | not documented | screening template with the result "required / not required" | open |
| 9 | **Incident process** (Art. 33/34) | not described | procedure, responsibilities, deadlines, notification template | open |
| 10 | **Privacy notice for patients** (Art. 13) | not present | template to hand out or display | open |
| 11 | **Consent** | e-mail consent in the patient record; photos, messengers and call recordings are not covered | consent management with date, purpose and withdrawal | open |
| 12 | **Processing agreements** (Art. 28) | self-hosted: not required | only needed for external hosting/backup/maintenance → sign a DPA | practice task |
| 13 | **Access rights** (Art. 32, § 203 StGB) | one account per user, admin role | rights per patient or case, locum access | open |
| 14 | **Storage limitation / deletion concept** (Art. 5(1)(e)) | manual archiving only | rules per data type (assessments, documents, e-mails, logs) | *long term* (with 1) |

---

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

The technical gaps from section 3 are tracked in `ROADMAP.md`:

- **Ten-year retention for medical records** (*next*) – points 1 and 3
- **Audit log** (*next*) – point 4
- **Backup and restore in the UI** (*mid term*) – point 6
- **Opt-in encryption** (*long term*) – point 5
- **Retention periods and deletion concept** (*long term*) – point 14

Points 2 and 7–11 and 13 are not in the roadmap yet; they will be added when they
are scheduled.
