## What kind of PR is this?

- [ ] **Entry** for `entries/<task>` (language: `<language>`) — measured
      trust score: `<vendored>/<hazards>` (current champion:
      `<v>/<h>` or *empty niche*)
- [ ] **Test case** for `tasks/<task>` — breaks the champion / closes
      spec gap `#<issue>`
- [ ] **New task** (spec + ≥4 cases + one passing entry)
- [ ] **Language proposal** (`languages.json` only)
- [ ] Tooling / docs

## Claims

- [ ] I ran `./arena check <dir> --no-sandbox` locally and it passed —
      output pasted below.
- [ ] All third-party code in this entry sits under `vendor/`, licenses
      included; everything outside `vendor/` I wrote for this entry.
- [ ] The diff touches nothing outside the directory named above.
- [ ] I license this contribution under GPL-3.0-or-later.

```
(paste your arena check output here — CI re-measures, this is for the diff review)
```
