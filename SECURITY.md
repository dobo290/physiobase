# Security policy

PhysioBase handles health data. Reports about security problems are welcome and
are taken seriously, whatever their size.

## Reporting a vulnerability

**Please do not open a public issue for a security problem.**

- Preferred: GitHub → **Security** → **Report a vulnerability**
  (<https://github.com/dobo290/physiobase/security/advisories/new>). That opens a
  private advisory only the maintainer can see.
- Alternative: the feedback page <https://feedback.gpl-nas.de/feedback> with the
  note "security" – only if the report contains no details that would help an
  attacker.

**Never include real patient data** in a report: no names, dates of birth,
addresses, findings, photos or database dumps. Use anonymised data, a synthetic
record, or describe the problem in the abstract. If you can only show it with a
real record, say so first and wait for a safe channel.

What helps:

- Version (`PhysioBase x.y.z`, visible on the login page) and how you run it
  (image or self-built, reverse proxy, HTTPS yes/no).
- Steps to reproduce, expected and actual behaviour.
- If relevant: the role you were logged in with (admin or user) and whether
  other users were involved.
- Log excerpts with host names, IPs, tokens, patient names and e-mail addresses
  removed.

## What to expect

- Acknowledgement within **7 days**.
- An assessment and, when the report is valid, a fix in a patch release
  (`0.x.y`) including a note in the release commit.
- Credit in the advisory if you want it. No bug bounty – this is a one-person
  project, not a company.

Please give the fix a reasonable head start (about 30 days) before publishing
details.

## Supported versions

Only the latest release gets fixes. Older `0.x` versions are not maintained.

## Scope

In scope: the application code in this repository (login, sessions, passkeys,
two-factor sign-in, role checks, uploads, file serving, calendar feeds, e-mail
sending, SQL handling, the container image) and the ways it is normally deployed
(container behind an HTTPS reverse proxy).

Out of scope: misconfiguration of the host, missing HTTPS, an exposed admin area
on the open internet, third-party components that already publish their own
advisories (report those upstream), and anything that requires an already
compromised server.

## What the project already does

- Sessions are signed, short-lived cookies checked against the database;
  logging out, changing a password or deactivating a user ends all sessions.
- Two-factor sign-in (TOTP) with single-use recovery codes, optional per user.
- Every push runs `go vet`, golangci-lint (gosec), gofumpt, govulncheck and the
  test suite with the race detector; CodeQL and Dependabot run on GitHub.
- The container image is scanned for HIGH/CRITICAL findings during the release
  pipeline.
