# Forge monorepo wiki

Working notes for the polyrepo → monorepo consolidation of the Fil Forge
services. Kept in the wiki rather than the repository so the repository's
history stays about the code.

## Pages

- **[[Plan]]** — one page: what is done, what is in flight, what is left.
  Start here if you want the shape rather than the detail.
- **[[Current State]]** — where the work is right now: approach, what
  has landed, what is next, with the evidence. A snapshot.
- **[[Needs Human Work]]** — the short list of things waiting on a person:
  what is blocking, what decisions are open, and what the agent cannot do
  itself.
- **[[Wins]]** — what this process found or fixed that was broken before it
  started, each marked solved or not solved. The page to draw the end-of-work
  report from.
- **[[Consolidation Findings]]** — the running log: latent bugs found, blind
  spots where a test did not cover what it appeared to, and the open items
  each one leaves behind.
- **[[MinIO Image Removal]]** — open and blocking: `minio/minio` was
  withdrawn from Docker Hub, breaking CI on every branch. Investigation,
  evidence, and the options.

## What lives where

| | |
|---|---|
| the plan | `forge-consolidation-plan.md` (phases 0–6, the traps section) |
| pre-consolidation research | `docs/consolidation/` in the old `fil-forge/forge` |
| this wiki | what the work has actually turned up since |

The first two are a plan and a survey, written before the work. This wiki is
the record of where they turned out to be incomplete, and of the things that
were only discoverable by doing it.
