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
- **Disk:** NVMe/SSD for the database and the media folder. Rotating disks only as
  slow storage for backups, never as the data path.
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
      - "8080:8080"
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
- **Firewall:** only the proxy ports from the LAN, nothing else. No database port,
  no SSH from outside without VPN.
- **One central host per practice.** Access from another location is solved with a
  tunnel or the reverse proxy – that does not need a second server. Two hosts
  sharing one SQLite file over the network is not supported (see section 8).

---

## 5. Backup and disaster recovery

The hardware assumptions in this document: **UPS, redundant power supply, RAID**
for the system disk, and a NAS with btrfs snapshots for long-term copies. That
covers power loss and a single failing disk.

What is still needed:

1. **Database snapshots** – the container writes consistent snapshots
   (`VACUUM INTO`) into `PHYSIOBASE_BACKUP_DIR` at start and every 24 hours.
   Recommendation: **hourly** for the database (it takes seconds), keep a week of
   hourly and a month of daily copies. Tracked in `ROADMAP.md`.
2. **Media and documents** – uploads live next to the database in `/data`. Copy
   them together with the snapshot (the NAS snapshot covers this if `/data` is on
   the NAS; a local copy plus a nightly sync is fine too).
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

## 9. Hand-over when the provider changes

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
