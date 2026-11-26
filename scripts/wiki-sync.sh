#!/usr/bin/env bash
# Spiegelt ROADMAP.md in die Wikis der Spiegel-Repos (GitHub und GitLab).
#
# Voraussetzung: das Wiki ist auf der Plattform einmal angelegt. Bei GitHub muss
# dafür die erste Seite über die Weboberfläche erstellt werden, danach ist
# <owner>/<repo>.wiki.git ein normales Git-Repository. Bei GitLab legt das Projekt
# das Wiki-Repository selbst an.
#
# Aufruf:  scripts/wiki-sync.sh                (Owner/Repo aus den Push-Remotes)
#          WIKI_URL=/pfad/zu/repo.git scripts/wiki-sync.sh   (Test, ein Ziel)
set -euo pipefail

cd "$(dirname "$0")/.."

version=$(sed -nE 's/^const Version = "([^"]+)".*/\1/p' internal/version/version.go | head -1)
stand=$(sed -nE 's/^\*\*Last updated:\*\* (.*)$/\1/p' ROADMAP.md | head -1)

# Ziele bestimmen: aus den Push-URLs von origin, oder ein einzelnes per WIKI_URL.
urls=$( { git remote get-url --all origin 2>/dev/null; git config --get-all remote.origin.pushurl 2>/dev/null; } \
	| grep -Eo 'github\.com[:/][^ ]+|gitlab\.com[:/][^ ]+' | sort -u || true)
targets=()
if [ -n "${WIKI_URL:-}" ]; then
	targets=("$WIKI_URL")
else
	for u in $urls; do
		slug=$(printf '%s' "$u" | sed -E 's#^(github|gitlab)\.com[:/]##; s#\.git$##')
		host=$(printf '%s' "$u" | grep -oE '^(github|gitlab)\.com')
		targets+=("https://${host}/${slug}.wiki.git")
	done
fi
if [ ${#targets[@]} -eq 0 ]; then
	echo "Keine GitHub-/GitLab-Push-URL gefunden. WIKI_URL=https://host/owner/repo.wiki.git setzen." >&2
	exit 1
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# Seiten einmal vorbereiten
mkdir -p "$work/pages"
cat >"$work/pages/Home.md" <<EOF
# PhysioBase

Self-hosted knowledge base and documentation tool for physiotherapy: one static
Go binary, one SQLite file, patient data stays on your own server.

- **Source code:** https://github.com/dobo290/physiobase (mirror: https://gitlab.com/dobo290/physiobase)
- **Live demo:** https://physiobase.gpl-nas.de (admin / demo1234)
- **Feedback:** https://feedback.gpl-nas.de/feedback
- **Version in the repository:** ${version} · Roadmap stand: ${stand}

## Pages

- [Roadmap](Roadmap) – what is done, what is planned, and what is deliberately
  not planned. The source of truth is \`ROADMAP.md\` in the repository; this page
  is generated from it by \`scripts/wiki-sync.sh\`.
- [Versioning and release process](Versioning) – how version numbers are assigned
  and how a release reaches the forge, GitHub, GitLab and the running instances.
- [Compliance roadmap](Compliance) – data protection and medical requirements: what
  is covered, what is missing, and the legal basis per item.

## Principles

- One static Go binary, one SQLite file, no cloud dependency.
- Pages are rendered on the server; JavaScript only where it adds real value.
- Data stays where it belongs: on your own server. Official reference data
  (ICD-10-GM, ICF, remedy catalogue, prices) is imported from official sources and
  kept locally, so no feature needs an internet connection at runtime.
- German user interface today, built so that other languages can be added.
EOF

{
	echo "<!-- Generated from ROADMAP.md by scripts/wiki-sync.sh – do not edit here. -->"
	echo
	cat ROADMAP.md
} >"$work/pages/Roadmap.md"

{
	echo "<!-- Generated from ROADMAP.md by scripts/wiki-sync.sh – do not edit here. -->"
	echo
	sed -n '/^## Versioning$/,/^## Release process$/p' ROADMAP.md | sed '$d'
	sed -n '/^## Release process$/,$p' ROADMAP.md
} >"$work/pages/Versioning.md"

# Compliance-Roadmap: unveraendert aus dem Repository
{
	echo "<!-- Generated from docs/compliance.md by scripts/wiki-sync.sh – do not edit here. -->"
	echo
	cat docs/compliance.md
} >"$work/pages/Compliance.md"

synced=0
for target in "${targets[@]}"; do
	dir="$work/$(printf '%s' "$target" | md5sum | cut -c1-8)"
	if ! git clone -q "$target" "$dir" 2>/dev/null; then
		echo "Übersprungen (Wiki-Repository noch nicht vorhanden): $target" >&2
		echo "  Bei GitHub einmal https://github.com/<owner>/<repo>/wiki öffnen und" >&2
		echo "  \"Create the first page\" klicken, dann erneut starten." >&2
		continue
	fi
	cp "$work/pages"/*.md "$dir"/
	git -C "$dir" add -A
	if git -C "$dir" diff --cached --quiet; then
		echo "Schon aktuell: ${target%.git}"
		continue
	fi
	git -C "$dir" -c user.name="${GIT_AUTHOR_NAME:-Admin}" -c user.email="${GIT_AUTHOR_EMAIL:-admin@gpl-nas.de}" \
		commit -q -m "Sync roadmap from repository (${version})"
	git -C "$dir" push -q origin HEAD
	echo "Wiki aktualisiert: ${target%.git} (${version})"
	synced=$((synced + 1))
done

[ "$synced" -gt 0 ] || true
