# Playwright fixture

Backs the five PW tasks in `docs/tier2-task-suites.md` section 2.2.

| Path | Purpose |
| --- | --- |
| `site/` | The static pages, served read-only. No external assets, no fonts, no images, no network calls: every byte the browser loads comes from this directory. |
| `serve.sh` | Starts `python3 -m http.server` bound to `127.0.0.1` on `${LOADLINE_TIER2_HTTP_PORT:-8930}`, serving `site/` and nothing else. `./serve.sh --check` verifies the pages are already up. |

## Pages

```
site/
  landing.html    nav bar with a link "Docs" -> docs.html
  docs.html       <title>Tier 2 Fixture Docs</title>
  catalog.html    product table; the "Widget Pro" row's price cell is "$42.00"
                  plus a link with visible text "Support", href="/support.html"
  form.html       "Email" and "Message" fields, a Submit button, and a
                  div#confirmation hidden until submit
  support.html    not referenced by any task
```

`support.html` is not in section 2.2. It exists so that a client which follows the Support link during PW-02 gets a page rather than a 404. PW-02 asks only for the href, and a 404 is a distractor that would add variance to a task whose criterion does not involve it.

Two determinism properties the tasks depend on:

- **`$42.00` appears exactly once on `catalog.html`**, in the Widget Pro row. No other price contains the substring `42.00`, so PW-01's transcript check cannot pass on the wrong row.
- **`form.html` does no server round trip and runs no validation.** The submit handler calls `preventDefault()` and writes the confirmation string with `textContent`, so the confirmation is a pure function of what was typed. Both fields are `type="text"` with no `required` attribute: a browser rejecting an address format is a failure mode the fixture does not need.

## Browser

`@playwright/mcp` defaults to the `chrome` channel, which means branded Google Chrome at `/opt/google/chrome/chrome`, and that is not installed on this estate. The failure mode is nasty because it is quiet: the server starts normally and lists all 24 tools, and only the first tool call that needs a page fails, with "Chromium distribution 'chrome' is not found". A run without the fix reads as five task failures rather than as a setup error.

The runner therefore passes `--browser chromium`, which resolves to the Playwright-managed chrome-for-testing build under `~/.cache/ms-playwright/`. `LOADLINE_TIER2_PW_BROWSER` overrides the choice.

`/usr/bin/chromium-browser` exists on this box but is deliberately not used: it is a shim for the chromium snap, and a snap-confined browser cannot reliably write a screenshot to an arbitrary path, which is exactly what PW-05 requires.

Installing the browser is `npx @playwright/mcp@<version> install-browser chromium`. `run-suite.sh` runs this as a preflight before the first playwright trial, so a missing browser stops the run rather than consuming five trials of plan quota.

The runner also passes `--output-dir <the trial's output directory>`, so the server's own artifacts (page snapshots, console logs) and PW-05's screenshot land in the per-trial output directory instead of a `.playwright-mcp/` directory in the working directory.

## Per-run setup

The pages are read-only and need no per-trial copy. Start the server once before the run and leave it up:

```sh
tier2/fixtures/playwright/serve.sh &
tier2/fixtures/playwright/serve.sh --check
```

`{port}` in every rendered prompt is `${LOADLINE_TIER2_HTTP_PORT:-8930}`. PW-05 additionally needs `{output_dir}`, a per-trial scratch directory the runner creates and passes into the prompt.

## Task map

`$TRANSCRIPT` is the file holding the client's final assistant-facing message, captured per section 3.2. `$PORT`, `$TRIAL_ID` and `$OUTPUT_DIR` are the values the runner substituted into the prompt.

| Task | Fixture prerequisites | Check | Mechanical success check |
| --- | --- | --- | --- |
| **PW-01** | `catalog.html` served; `$42.00` unique to the Widget Pro row | Transcript | `grep -Fq '42.00' "$TRANSCRIPT"` |
| **PW-02** | `catalog.html` served; exactly one link with visible text `Support` | Transcript | `grep -Fq '/support.html' "$TRANSCRIPT"` |
| **PW-03** | `landing.html` and `docs.html` served; the Docs link resolves | Transcript | `grep -Fq 'Tier 2 Fixture Docs' "$TRANSCRIPT"` |
| **PW-04** | `form.html` served; `div#confirmation` hidden before submit | Transcript | `grep -Fq "Thanks, agent-$TRIAL_ID@example.com! Message received: loadline tier2 probe" "$TRANSCRIPT"` |
| **PW-05** | `catalog.html` served; `$OUTPUT_DIR` exists and is writable | State | `[ -s "$OUTPUT_DIR/$TRIAL_ID.png" ]` |

Notes on the checks:

- PW-01 matches `42.00` rather than `$42.00`, which is section 2.2's stated tolerance: both renderings count, and the bare form matches either.
- PW-04's expected string is built from `$TRIAL_ID`, so a stale transcript from an earlier trial cannot satisfy it.
- PW-05 checks existence and non-zero size only. Screenshot rendering is not byte-stable across runs, so a byte comparison would fail honest trials.
- `browser_navigate` returns its page snapshot as a link to a `.yml` file rather than inline, so a client that needs the page content calls `browser_snapshot` afterwards, which does return the snapshot inline. That extra call is real cost the tasks measure and is not a fixture problem; verified against `@playwright/mcp@0.0.79` on 2026-08-18.
