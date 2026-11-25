#!/usr/bin/env bash
# Spiegelt ROADMAP.md ins GitHub-Wiki (https://github.com/<owner>/<repo>/wiki).
#
# Voraussetzung: das Wiki wurde auf GitHub einmal angelegt ("Create the first
# page"). Danach ist <repo>.wiki.git ein normales Git-Repository.
#
# Aufruf:  scripts/wiki-sync.sh            (Owner/Repo aus dem origin-Remote)
#          WIKI_REPO=github.com/foo/bar scripts/wiki-sync.sh
set -euo pipefail

cd "$(dirname "$0")/.."

# Owner/Repo bestimmen: bevorzugt aus dem GitHub-Remote, sonst aus dem Parameter.
WIKI_REPO="${WIKI_REPO:-}"
if [ -z "$WIKI_REPO" ]; then
	# origin hat mehrere Push-URLs (Forge, GitHub) – alle durchsuchen.
	url=$( { git remote get-url --all origin 2>/dev/null; git config --get-all remote.origin.pushurl 2>/dev/null; } | grep -m1 'github\.com' || true)
	if [ -z "$url" ]; then
		echo "Kein GitHub-Remote gefunden. WIKI_REPO=github.com/<owner>/<repo> setzen." >&2
		exit 1
	fi
	WIKI_REPO=$(printf '%s' "$url" | sed -E 's#^.*github\.com[:/]##; s#\.git$##; s#/+$##')
fi

# WIKI_URL erlaubt eine abweichende URL (Test mit lokalem Repository).
WIKI_URL="${WIKI_URL:-https://github.com/${WIKI_REPO}.wiki.git}"

version=$(sed -nE 's/^const Version = "([^"]+)".*/\1/p' internal/version/version.go | head -1)
stand=$(sed -nE 's/^\*\*Last updated:\*\* (.*)$/\1/p' ROADMAP.md | head -1)

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

if ! git clone -q "$WIKI_URL" "$work/wiki" 2>/dev/null; then
	echo "Wiki-Repository ${WIKI_URL} existiert noch nicht." >&2
	echo "Einmal https://github.com/${WIKI_REPO}/wiki öffnen, dort \"Create the first page\" klicken," >&2
	echo "speichern und das Skript erneut starten." >&2
	exit 1
fi

# Home: Einstieg mit Kurzüberblick und Links
cat >"$work/wiki/Home.md" <<EOF
# PhysioBase

Self-hosted knowledge base and documentation tool for physiotherapy: one static
Go binary, one SQLite file, patient data stays on your own server.

- **Source code:** https://${WIKI_REPO}
- **Live demo:** https://physiobase.gpl-nas.de (admin / demo1234)
- **Feedback:** https://feedback.gpl-nas.de/feedback
- **Version in the repository:** ${version} · Roadmap stand: ${stand}

## Pages

- [Roadmap](Roadmap) – what is done, what is planned, and what is deliberately
  not planned. The source of truth is \`ROADMAP.md\` in the repository; this page
  is generated from it by \`scripts/wiki-sync.sh\`.
- [Versioning and release process](Versioning) – how version numbers are assigned
  and how a release reaches the forge, GitHub and the running instances.

## Principles

- One static Go binary, one SQLite file, no cloud dependency.
- Pages are rendered on the server; JavaScript only where it adds real value.
- Data stays where it belongs: on your own server. Official reference data
  (ICD-10-GM, ICF, remedy catalogue, prices) is imported from official sources and
  kept locally, so no feature needs an internet connection at runtime.
- German user interface today, built so that other languages can be added.
EOF

# Roadmap: unverändert aus dem Repository
{
	echo "<!-- Generated from ROADMAP.md by scripts/wiki-sync.sh – do not edit here. -->"
	echo
	cat ROADMAP.md
} >"$work/wiki/Roadmap.md"

# Versionierung und Release-Ablauf: die beiden Abschnitte aus der Roadmap
{
	echo "<!-- Generated from ROADMAP.md by scripts/wiki-sync.sh – do not edit here. -->"
	echo
	sed -n '/^## Versioning$/,/^## Release process$/p' ROADMAP.md | sed '$d'
	sed -n '/^## Release process$/,$p' ROADMAP.md
} >"$work/wiki/Versioning.md"

cd "$work/wiki"
git add -A
if git diff --cached --quiet; then
	echo "Wiki ist schon aktuell (${version})."
	exit 0
fi
git -c user.name="${GIT_AUTHOR_NAME:-Admin}" -c user.email="${GIT_AUTHOR_EMAIL:-admin@gpl-nas.de}" \
	commit -q -m "Sync roadmap from repository (${version})"
git push -q origin HEAD
echo "Wiki aktualisiert: https://github.com/${WIKI_REPO}/wiki (${version})"
