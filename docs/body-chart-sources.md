# Sources of the body chart images

All three templates are **CC0 (public domain)** from Wikimedia Commons – free to
use, no attribution required.

| View | File | Source |
|---|---|---|
| Front | `web/static/body/body-front.png` | Wikimedia Commons, CC0 |
| Back | `web/static/body/body-back.png` | Wikimedia Commons, CC0 |
| Side | `web/static/body/body-side.png` | Wikimedia Commons, CC0 |

The aspect ratios are defined in `internal/befund/schema.go` → `Views`; the
drawing coordinates are stored normalised (0..1), so images can be replaced as long
as the aspect ratio matches.
