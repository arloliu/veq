# data/

Real inputs (vendor manuals, EAP logs, captures, configuration) and everything derived from them.
Only this file and `.gitignore` are tracked; every other file here stays local.
How to bring an input in and convert it: `docs/guides/data-intake.md`.

## Rules

- Keep the tree owner-only: `chmod -R go-rwx data`.
- Nothing read from here enters a tracked file, a commit message or a message to another agent session:
  no company, site, tool ID, person or internal-service name, no local path, no quoted manual text, no real traffic
  (`.agents/rules/100-overview.md`, prime directive 1).
  Tracked text names tools by role ("the main tool", "the load-port controller") and cites manuals by page number only.
  A message to another session carries counts, not content, and is sent only with the owner's approval.
- `make data-check` fails when git tracks or stages any other file under `data/`;
  `.gitignore` here re-includes only top-level, anchored names.
  `make hooks`, once per clone, runs the same check before every commit.
- `derived/` holds regenerated output only; never edit it by hand.
- `LOCAL.md` is this deployment's index, untracked:
  per input, its source, document number or revision, software version and time span, and the commands that regenerate `derived/`.
  Update it whenever an input is added.

## Layout

```
data/
  LOCAL.md                  local index (untracked)
  <equipment-model>/        one directory per tool model; tools of one model share a manual
    manual/
      <manual>.pdf
      text/pages/pNNNN.txt    one file per PDF page
      text/toc.tsv            PDF bookmarks
      text/rotated_pages.txt  pages with a nonzero rotation
    traces/<format>/<tool-id>/<yyyy-mm-dd>.log
                            raw logs as received, one directory per tool; <format> is eap-sml for EAP SML logs
    derived/<analysis>/     outputs regenerated from manual/ and traces/
```
