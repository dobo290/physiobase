#!/usr/bin/env bash
# Creates a GitHub release with a changelog for a tag that is already pushed.
#
#   scripts/github-release.sh v0.15.0 [notes.md]
#
# Without a notes file the annotated tag message is used as the changelog.
# The token is read from $GH_TOKEN or /root/.config/gh-token. The repository is
# the GitHub mirror (dobo290/physiobase); override with REPO=owner/name.
set -euo pipefail

tag=${1:?Aufruf: github-release.sh vX.Y.Z [notes.md]}
notes=${2:-}
repo=${REPO:-dobo290/physiobase}
token=${GH_TOKEN:-$(cat /root/.config/gh-token 2>/dev/null || true)}
[ -n "$token" ] || { echo "Kein GitHub-Token (GH_TOKEN oder /root/.config/gh-token)" >&2; exit 1; }

if [ -z "$notes" ]; then
	body=$(git tag -l --format='%(contents)' "$tag")
	[ -n "$body" ] || { echo "Tag $tag nicht gefunden und keine Notizen angegeben" >&2; exit 1; }
else
	body=$(cat "$notes")
fi

python3 - "$repo" "$tag" "$body" "$token" <<'PY'
import json, sys, urllib.error, urllib.request

repo, tag, body, token = sys.argv[1:5]
title = tag
payload = json.dumps({
    "tag_name": tag,
    "name": title,
    "body": body,
    "draft": False,
    "prerelease": False,
}).encode()
req = urllib.request.Request(
    f"https://api.github.com/repos/{repo}/releases",
    data=payload,
    headers={
        "Authorization": "Bearer " + token,
        "Accept": "application/vnd.github+json",
        "Content-Type": "application/json",
    },
    method="POST",
)
try:
    with urllib.request.urlopen(req) as r:
        print("Release angelegt:", json.load(r).get("html_url"))
except urllib.error.HTTPError as e:
    print("HTTP", e.code, e.read().decode()[:800], file=sys.stderr)
    sys.exit(1)
PY
