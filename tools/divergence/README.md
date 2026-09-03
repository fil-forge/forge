# tools/divergence

Measures, from git history alone, how far the consumers of `libforge` and
`ucantone` drifted from the libraries' `main` over a window of time, and what
kind of library change they had to absorb. It regenerates
`docs/consolidation/divergence-august-2026.md` (tables) and
`docs/consolidation/divergence-august-2026.json` (everything, machine-readable).

Go standard library only; shells out to `git`. Every repository is read-only:
the tool runs `git log`, `git diff`, `git archive`, `git merge-base`,
`git rev-list`, `git show`. Nothing is fetched, checked out or written outside
the output files.

## Run

```
cd tools/divergence && GOWORK=off go run . -from 2026-08-01 -to 2026-08-31 -today 2026-09-03
```

or `./run.sh` with the same arguments. `GOWORK=off` because the tool is its own
module and is deliberately not listed in the repository's `go.work`. Defaults
match this machine's layout; every path and ref is a flag (`-h`).

| flag | default | meaning |
|---|---|---|
| `-forge`, `-forge-ref` | the checkout holding the tool, `HEAD` | monorepo; its `go.work` modules with a libforge/ucantone `require` become the `forge/<svc>` consumers |
| `-libforge`, `-ucantone`, `-lib-ref` | `/home/user/libforge`, `/home/user/ucantone`, `origin/main` | the libraries; `-lib-ref` falls back to `main`, then `HEAD` |
| `-live-dir`, `-live-repos`, `-live-ref` | `/home/user/fil-forge`, the seven service repos, `HEAD` | the live per-service clones |
| `-guppy`, `-indexing-service`, `-consumer-ref` | `/home/user/guppy`, `/home/user/indexing-service`, `origin/main` | the two library consumers outside the service fleet |
| `-from`, `-to`, `-context-from` | `2026-08-01`, `2026-08-31`, one month before `-from` | measurement window and the context period shown next to it (UTC days) |
| `-today` | now | date used for the "lag today" figures |
| `-out` | `docs/consolidation/divergence-august-2026` | output base name, relative to `-forge` |
| `-classification` | `classification.json` next to the tool | curated per-commit classes and evidence |

The tool prints the SHA of every repository it read.

## What is measured

- **a.** Commits and merged PRs per repo per ISO week (committer date, UTC),
  dependabot separated by author name. Merged PRs are merge commits plus squash
  commits whose subject ends in `(#N)`. forge is counted first-parent only,
  because its history embeds the five imported service histories.
- **b.** For every first-parent commit on each library's main in the period:
  the packages whose non-test `.go` files changed, which consumers import one
  of them (static scan of the consumers' Go files at the analysed ref, via
  `git archive` and `go/parser`), and whether the change is wire-visible
  (libforge: `commands/**` or `blobindex/**`; ucantone: a path under `ucan/`,
  `validator/`, `execution/`, `varsig/`, `multikey/`, `did/` is a candidate and
  the curated file decides).
- **c.** The curated class of every human code commit — additive-optional,
  additive-required, breaking, internal — with the evidence hunk, and the
  uptake per consumer: the first consumer commit whose pin contains the change
  and how many days later that was. Containment is by ancestry; for pins that
  are PR-branch heads not on main, by per-file content identity with the
  change; for SHAs absent from the local clone, by pseudo-version timestamp.
- **d.** The pin-lag series: for every consumer, at each commit that changed
  its libforge or ucantone pin (first-parent history of the consumer's `go.mod`,
  following subtree imports), the days between the pinned commit and the
  library main head at that moment, plus the same figure today.
- **e.** Daily snapshots of the fleet's pins (distinct pins, spread between
  oldest and newest, max/median lag) and the straddles: periods during which one
  consumer already contained a breaking/additive-required change while another
  did not.

## Curation

`classification.json` maps a commit prefix to `{class, wire_visible, evidence,
note}`. Commits the file does not cover appear as `UNCLASSIFIED` in the report
rather than being guessed, so a re-run on newer history shows exactly what still
needs a human to read the diff. Dependabot, non-code and test-only commits are
classified automatically.

## Preserving prose

The markdown report ends with a marker comment. Everything below it is copied
verbatim from the previous version of the file on every run, so hand-written
analysis lives in the same document without being overwritten.
