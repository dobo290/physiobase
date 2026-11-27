# Hosting PhysioBase – recommendations

**As of:** 25.11.2026 · Version 0.13.6

PhysioBase is a single Go binary with one SQLite database. It runs on one machine
and is reached over the network. This document describes what that machine needs,
how it should be operated, and what a practice has to decide.

**Who this is for:** either an **IT service provider** who hosts and maintains the
instance for the practice, or a **practice owner or technician** who runs it
themselves. Both are normal cases; nothing in the software requires a cloud, an
account at a vendor, or permanent internet access.

Data protection and legal duties are in [`compliance.md`](compliance.md). The
technical roadmap items referenced here are in [`../ROADMAP.md`](../ROADMAP.md).

---

## 1. What the practice is responsible for

PhysioBase is software, not a service. Whoever hosts it is the party that can see
health data, so:

- **Self-hosted by the practice:** the practice is the controller and needs no
  processing agreement with anyone for the software itself.
- **Hosted by an IT provider:** the provider acts as a **processor**; a processing
  agreement (Art. 28 GDPR) is required, including the server, the backups and any
  remote maintenance access. Practice staff must still be able to work if the
  provider is unavailable.
- **Data stays in the practice's control** in both models: no telemetry, no cloud
  API, no vendor access. The database is a file the practice can copy and keep.

Decide and write down: who has root/admin on the host, who holds the backups, who
is called during an outage, and who is allowed to restore.

---

## 2. Server sizing

Baseline: one instance, one database, one machine. Numbers below come from the load
model in `ROADMAP.md` ("Stability for practices with up to 250 therapists").

| Practice size | Appointments/week | CPU | RAM | Database after 10 years | Notes |
|---|---|---|---|---|---|
| 1–5 therapists | ~600 | 2 cores | 4 GB | < 1 GB | mini PC, NUC or a small VM is plenty |
| 10–40 therapists | ~5,000 | 4 cores | 8 GB | 2–5 GB | one SSD, 16 GB RAM if it also does other work |
| up to 250 therapists | ~30,000 | 8–16 cores | 16–32 GB | 10–20 GB | see section 8: archiving keeps the live database small |

- **CPU** matters more than RAM: rendering pages and SQLite queries are CPU bound.
- **Disk:** SSD (NVMe or SATA) for the database and the media folder. **"NAS" in
  this document means HDD-backed network storage**: long sequential reads and slow
  random access, which makes it the right place for backups and long-term snapshots
  and the wrong place for the live database.
- **RAM** helps through the filesystem cache; SQLite profits from it directly.
- The measured ceiling of the current design is two orders of magnitude above the
  largest row in this table (23,000 inserts/s, 29,000 reads/s single-connection),
  so sizing is about comfort, not about reaching a limit.

---

## 3. Operating system and runtime

- **Debian** (or a Debian derivative) with automatic security updates.
- **Podman** or **Docker** plus `podman-compose`/`docker compose`. The published
  image is `forge.gpl-nas.de/physiobase:latest`; it is also available as
  `vX.Y.Z` tags for pinned deployments.
- **Reverse proxy with TLS** in front (Caddy, nginx, Traefik). The container itself
  speaks plain HTTP on port 8080 and expects a proxy for HTTPS; set
  `PHYSIOBASE_BASE_URL=https://…` so cookies become `Secure` and HSTS is sent.
- **Time sync** (`systemd-timesyncd` or chrony). Time drives sessions, TOTP and
  appointment sorting.
- **No other client** on the database file. Exactly one container may open the
  database – see section 8 for why.

### Container contract

| Item | Value |
|---|---|
| Image | `forge.gpl-nas.de/physiobase:latest` (or a pinned `vX.Y.Z`) |
| Port | `8080` |
| Volume | `/data` – database, uploads, patient documents, backups |
| Health check | `physiobase healthcheck` (already in the image) |
| Log | stderr, `PHYSIOBASE_LOG_FORMAT=json` for log collection |

Minimal compose file:

```yaml
services:
  physiobase:
    image: forge.gpl-nas.de/physiobase:latest
    container_name: physiobase
    env_file: .env
    volumes:
      - /srv/physiobase:/data
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "8080:8080"          # application, LAN only
      # - "8081:8081"        # optional: public website (see section 4)
    restart: unless-stopped
    stop_grace_period: 30s
```

### Configuration

Everything is an environment variable; the full list is in the README. The ones a
hosting setup normally sets:

`PHYSIOBASE_BASE_URL`, `PHYSIOBASE_PUBLIC_URL`, `PHYSIOBASE_DATA_DIR`,
`PHYSIOBASE_BACKUP_KEEP`, `PHYSIOBASE_PRACTICE_NAME`, `PHYSIOBASE_PRACTICE_ADDRESS`,
`PHYSIOBASE_SMTP_*`, `PHYSIOBASE_LOG_FORMAT=json`, and the first admin via
`PHYSIOBASE_USERNAME` + `PHYSIOBASE_PASSWORD_HASH` (or the setup wizard, once
available).

---

## 4. Network and access

- **LAN only** for the practice; the admin area is deliberately not exposed to the
  internet.
- **Remote access through a VPN tunnel** (WireGuard, or SSH port forwarding) to the
  practice network. Do not publish the admin area on the internet, not even behind
  a password.
- **Patient-facing pieces** (calendar subscription links) can be published through
  the reverse proxy; they are protected by a secret token in the URL.
- **The public website** runs on its own port (`PHYSIOBASE_SITE_PORT`, for example
  8081) and is the only thing published under the practice's own domain. Keep the
  application port (8080) out of the public proxy configuration.
- **Firewall:** only the proxy ports from the LAN, nothing else. No database port,
  no SSH from outside without VPN.
- **One central host per practice.** Access from another location is solved with a
  tunnel or the reverse proxy – that does not need a second server. Two hosts
  sharing one SQLite file over the network is not supported (see section 8).
- **The application itself has no IP allowlist** for the admin area; restrictions
  belong in the reverse proxy (for example Caddy's `remote_ip` matcher), in the
  cloud firewall or in the tunnel. `X-Forwarded-For` is read only for the demo
  visitor statistics, never for access decisions, so no proxy header can widen
  access. There is also no per-IP rate limit: sign-in is throttled globally (two
  concurrent bcrypt checks plus a delay after a failure), which works unchanged
  behind a proxy.

---

## 5. Backup and disaster recovery

The hardware assumptions in this document: **UPS, redundant power supply, RAID**
for the system disk, and a **NAS on HDDs** for long-term copies and snapshots.
That covers power loss and a single failing disk.

What is still needed:

1. **Database snapshots** – the container writes consistent snapshots
   (`VACUUM INTO`) into `PHYSIOBASE_BACKUP_DIR` at start and every 24 hours.
   Recommendation: **hourly** for the database (it takes seconds), keep a week of
   hourly and a month of daily copies. Tracked in `ROADMAP.md`.
2. **Media and documents** – uploads live next to the database in `/data`. Copy
   them together with the snapshot (a snapshot of the storage covers this if the
   data lives there; keeping the live data on SSD and syncing media to the HDD NAS
   overnight is the faster variant).
3. **Off-site copy** – at least one encrypted copy must leave the building
   (rotating disk, safe, or a second location over the tunnel). Backups in the same
   room as the server do not survive a fire or a flooded cellar.
4. **Rehearsed restore** – once per quarter, restore the latest snapshot into a
   test container and check that patients, documents and calendar links are there.
   Write down how long it took; that number is the recovery time the practice can
   promise.
5. **Targets to agree on:** RPO (accepted data loss) and RTO (time until work can
   continue) – 1 hour and 2 hours are realistic with hourly snapshots and a spare
   machine.

Restore outline:

```sh
systemctl stop physiobase
cp /srv/physiobase/backup/physiobase-<stamp>.db /srv/physiobase/physiobase.db
rm -f /srv/physiobase/physiobase.db-wal /srv/physiobase/physiobase.db-shm
systemctl start physiobase
```

---

## 6. Hardening checklist for the host

- [ ] Disk encryption (LUKS) for the system and data volume
- [ ] Automatic security updates enabled
- [ ] SSH only with keys, no root password login, sshguard/fail2ban active
- [ ] Firewall: only proxy and LAN ports reachable
- [ ] Reverse proxy with TLS, HSTS and a current certificate
- [ ] Admin accounts in PhysioBase limited to the people who need them, **second
      factor enabled** for everyone with access to health data
- [ ] Monitoring: container healthy, snapshot age, free disk space, TLS expiry
- [ ] Backup target reachable and checked (a restore is the only proof)
- [ ] Log retention decided (host logs may contain connection metadata)

---

## 7. Updates

- The image is tagged `latest` and per release `vX.Y.Z`. Pin a version for
  production and update deliberately; `latest` follows the newest release.
- Update procedure: pull image → restart container → check the login page version
  and `/healthz`. Downtime is a few seconds; schedule it outside treatment hours.
- Read the release notes in the repository (`ROADMAP.md` → Done) before updating;
  database migrations run automatically and are additive.
- CI builds and tests every commit, publishes the image, and the repository holds
  the version so a rollback is a tag change plus a restore of the matching
  snapshot if a migration is involved.

---

## 8. What not to do

| Don't | Why |
|---|---|
| Put the SQLite file on NFS/SMB and run two hosts | file locking over network filesystems is unreliable; corruption instead of errors |
| Run two containers on different machines on the same `/data` | same problem; one host at a time, a standby takes over after a failure |
| Expose the admin area to the internet | the app assumes a trusted LAN behind a tunnel |
| Use rotating disks for the database | random access dominates; NVMe/SSD is the baseline |
| Delete the `-wal`/`-shm` files while the container runs | they belong to the live database |
| Rely on the daily snapshot alone as "the archive" | retention is the live database; backups are for recovery (see `compliance.md`) |

**Second host** only as a **standby**: same snapshot, started only after a failure,
one database owner at a time. Running both in parallel requires a client-server
database (PostgreSQL) – that is an option in `ROADMAP.md`, not a requirement for a
practice with up to 250 therapists.

---

## 9. Off-site hosting at a provider (VPS)

**Software-wise nothing changes.** The image runs on any virtual server with a
container runtime, a persistent volume and a reverse proxy. A €5–30/month VPS at a
European provider (Hetzner, netcup, IONOS, Scaleway …) is enough for every size in
the table in section 2. What changes is the surrounding setup:

| Topic | On-site (practice server) | Off-site (VPS at a provider) |
|---|---|---|
| **Processing agreement** | not required for the software itself | **required** with the provider (Art. 28 GDPR); Hetzner and others offer it as a standard document – sign it, keep it |
| **Who can reach the disk** | the practice | the provider's staff, in theory. Mitigation: **encryption at rest** (roadmap: opt-in encryption) and a provider in the EU |
| **Admin access** | LAN only | there is no LAN: put a **WireGuard tunnel** on the VPS and allow the admin area only through it, or restrict it by IP in the proxy/firewall |
| **Internet dependency** | practice keeps working if the WAN fails | **no work when the practice's internet or the provider is down**; plan a fallback (LTE router, or paper forms for the worst case) |
| **Backups** | snapshots on the NAS | snapshots on a **separate volume**, and the off-site copy belongs **at the practice** or at a second provider – not on the same VPS |
| **Provider snapshots** | – | useful for disaster, but **not** a substitute for `VACUUM INTO` snapshots: they are not database-consistent |
| **Firewall** | LAN rules | provider cloud firewall: allow 443 from anywhere (patient links) and 22/51820 **only from the practice's network** |
| **Updates/reboots** | scheduled | scheduled the same way, but a host reboot is done by the provider – announce maintenance windows |
| **Time** | local RTC | NTP is essential (sessions, TOTP, appointments) |

Recommended shape for a VPS deployment:

```
Internet ──443──> Caddy (TLS, Let's Encrypt) ──> physiobase:8080 (container)
Practice LAN ──WireGuard──> VPS (admin area, SSH)
PhysioBase snapshots ──> attached volume ──> nightly encrypted sync to the practice/NAS
```

Minimum for a Hetzner-style setup:

- 2–4 vCPU, 8 GB RAM, 40–80 GB volume for up to 40 therapists (sections 2 and 8:
  archive old records to keep the live database small),
- volume mounted at `/data`, `PHYSIOBASE_BACKUP_KEEP=24` (hourly) if disk allows,
- Caddy with the practice's domain, `PHYSIOBASE_BASE_URL=https://…`,
- WireGuard for admin/SSH, cloud firewall as described,
- one monitoring check from outside (URL + certificate expiry) and one for snapshot
  age.

**One rule stays:** exactly one host owns the database. A VPS plus a second VPS on
the same volume is not supported; a cold standby that starts after a failure is
(section 8).

## 10. Scale, operations and recovery – roadmap

These items belong to whoever operates an instance (practice or IT provider). They are
tracked here instead of in `ROADMAP.md`, because they only matter once the instance
grows or has to be recovered.

- [ ] **Refuse to start on a newer database** *(next)*
      - `migrate()` only applies missing migrations. An older binary on a database
        written by a newer one starts silently and fails later with missing columns.
      - On startup, compare the schema version with the known migrations and stop
        with a clear message ("this data directory belongs to version X, this binary
        understands up to Y") instead of corrupting anything.

- [ ] **Integrity check and restore in the pipeline** *(mid term)*
      - Nothing currently verifies that a snapshot is readable or that the live
        database is intact. Add a scheduled `PRAGMA quick_check`, a checksum of every
        snapshot, and a pipeline job that restores the newest snapshot into a
        throwaway container and queries patient counts.

- [ ] **Stability for practices with up to 250 therapists** *(next)*
      - Target, not wishful thinking: one instance, one database, 250 therapists
        working 40 h/week in 20-minute slots at the same time.
      - Load that this means, derived from the slot model: 250 × 40 h = 10,000
        therapist-hours per week → **30,000 appointments per week**, 6,000 per day,
        ~1.4 million per year. Writes average **0.2 per second** with bursts of a
        few hundred when slot boundaries align; reads reach **100–200 requests per
        second** in reality (theoretical peak 500), each page costing 10–40 queries.
      - **Baseline measured today** (SQLite + WAL, modernc driver, one SSD-backed
        host): 23,000 inserts/s and 29,000 reads/s with a single connection, no
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

- [ ] **Data volume and archiving** *(mid term)*
      - Ten years of records for 250 therapists is roughly **10–20 GB**. Daily
        snapshots (`VACUUM INTO`) take seconds on local NVMe but minutes on the
        **HDD-backed NAS**, and `VACUUM` grows with file size.
      - Move records past their retention period into a **read-only archive database
        file** (opened on demand), keep the live database small (target: below 5 GB),
        and stage backups on the local NVMe before they are copied to the HDD NAS.
      - Admin view: database size, last backup duration, archive state.

- [ ] **Disaster recovery on other hardware** *(mid term)*
      - Assumed and already in place: **UPS, redundant power supply, RAID** on the
        server, a NAS on HDDs for snapshots. Power loss and a single failing disk
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

## 11. Hand-over when the provider changes

Hand over, in writing:

1. `compose.yml`/`podman-compose.yml` and `.env` (with secrets in a password
   manager, not in e-mail).
2. The whole `/data` directory: `physiobase.db`, `medien/`, `patienten/`,
   `backup/`.
3. The backup procedure, the off-site location and the last restore report.
4. The reverse proxy configuration and the DNS records.
5. Which PhysioBase version runs, and where release notes are read.

The practice keeps a copy of points 1–3 at all times; that copy is what allows a
new provider (or the practice itself) to continue without asking anyone.
