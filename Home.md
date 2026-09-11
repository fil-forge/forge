# Forge monorepo wiki

Working notes for the polyrepo → monorepo consolidation of the Fil Forge
services. Kept in the wiki rather than the repository so the repository's
history stays about the code.

## Pages

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
