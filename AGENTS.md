# forge — agent guide

The Fil Forge monorepo. Ten services, each brought in by `git subtree` with its
full history: `delegator`, `forgectl`, `hilt`, `indexing-service`, `ingot`,
`piri`, `piri-signing-service`, `smelt`, `sprue`, `swarf`. Each is its own Go
module; `go.work` ties them together.

> **This file is the short form.** The long form — why each rule exists, what
> was tried and failed, what is still open — lives on the wiki, which is the
> `wiki` branch of this repository. Read [[Current State]] before starting
> anything; it is a kept-current snapshot and it names what is in flight.

## This file is scaffolding, and should be replaced when the scaffolding comes down

Most of what follows exists because this repository is being *assembled*, not
because it is how Forge is worked on. Rules 7 and 8 are entirely about
`git subtree`; rule 1 is about what is still being moved in; the wiki and the
four-document split exist to carry decisions across a construction that spans
many sessions and one person's attention. None of that is a durable guide to a
finished monorepo, and left in place it will read as though it were.

**Replace it when the consolidation is finished** — when nothing is left to
import, no subtree pulls are pending, the `forge-2` → `forge` rename has
happened, and Phase 1's release machinery is real. That is a checkable set of
conditions rather than a feeling, on purpose; the same reason the wiki trigger
below is an event.

What a replacement probably keeps: rules 2 through 5, which are about the code
and its CI rather than about moving it, and the commands and conventions at the
end. What it probably drops: rules 1 and 6 through 9, the wiki, and most of the
document table — by then the wiki's content belongs in the repository or in
issues, and *Needs Human Work* should be empty. Do not treat that split as
settled; decide it when you can see the finished shape.

## Where state lives, and what goes where

Four places, and putting something in the wrong one is how it gets lost:

| | holds | lifetime |
|---|---|---|
| **wiki → Current State** | what is true right now: `main`'s sha, open PRs, the rules, known debt | replaced as things change, never appended to |
| **wiki → Needs Human Work** | what is blocked on a person, and decisions taken on their behalf awaiting confirmation | items leave via *Recently cleared* |
| **`MAJOR_DECISIONS.md`** | decisions with a rationale someone will otherwise re-litigate — what is deliberately *out* of the monorepo, and why | permanent |
| **`MONOREPO_TODO.md`** | questions only answerable once the monorepo is whole, and bugs found in imported code that are not the migration's to fix | until answered |

Ordinary unfinished work belongs in issues, not in any of these.

## Update the wiki in the same turn as the thing it describes

**Not "keep it current".** That phrasing has failed here more than once: the
wiki is updated when someone happens to look, and drifts in between. The
trigger is an event, so it can be checked:

**Opened, pushed to, rebased, merged or closed a PR? Made a decision, or took
one on someone's behalf? Found something you are deliberately not fixing?**
→ update the wiki *before* you report what you did, not after.

A `main` sha or a PR list that is one merge stale is worse than no snapshot,
because it is believed. If you touched the repository and the wiki still
describes the previous state, you are not finished.

The wiki lives in two places that must stay in sync: the `wiki` branch here,
and the GitHub wiki at `forge-2.wiki.git`. Sessions can normally push only the
branch; check [[Needs Human Work]] for who is mirroring.

## Rules that have actually decided things

Numbered as the wiki numbers them; it carries the reasoning.

1. **What goes in: things that ship as the Forge network.** Outward-facing
   libraries, forks of upstream software, and things being retired stay out.
   `MAJOR_DECISIONS.md` has the list. **Do not import another module without a
   human decision** — every module in scope is already in.
2. **Build what we own; pin what we don't.** In-repo services are built from
   HEAD in CI; everything else is pinned by digest — compose images, Go
   testcontainers constants, and Dockerfile `FROM` lines alike. Pin to the
   **index** digest, never a per-architecture one, or cross-platform builds
   break silently. Reuse a digest the repository already names rather than
   resolving fresh: two builds of one tag in one repo is the failure this
   prevents.
3. **Derive dependency lists, never hand-maintain them.** `go list -deps` on
   the build target, not `go.mod`'s replace list. It is sometimes wider than
   the obvious guess — that is how `ingot → indexing-service` and
   `delegator → forgectl` were found, neither of which `go.mod` showed.
4. **Test what ships.** A bind-mounted binary in someone else's image is not
   the artifact that reaches production.
5. **A green check is a claim about what ran.** Ask what the job would have had
   to *do* to catch the fault.
   - When you fix one instance, run a command that enumerates the class before
     committing. Careful reading has missed the rest every time.
   - A guard over *part* of a chain reads exactly like a guard over the chain.
     Ship no guard rather than a partial one that looks total.
6. **Stacked branches rebase onto their base; they do not merge it.**
7. **Subtree history is never squashed, and a branch containing a
   `git subtree add` is never rebased.** A plain rebase silently flattens the
   merge that carries the imported history. When the base moves under such a
   branch, **rebuild** it: base tip, fresh `git subtree add` at the same
   upstream commit, then cherry-pick — then check the tree against the head it
   replaced and that the upstream root is still an ancestor.
8. **A PR containing a `git subtree add` links its non-subtree commits.** One
   `/pull/<n>/files/<sha>..<sha>` link per contiguous range, near the top of
   the body, refreshed on every push. The PR's own headline numbers describe
   the import, not the work.
9. **A branch meant to be reviewed and merged gets a PR when it is pushed.**
   Scratch branches do not.

## Commands

```
make -C deploy …          # per-service deploy tooling, where it exists
GOWORK=off go build ./...  # inside a module: what CI does, standalone
go build ./...             # workspace mode, across go.work
```

CI is four workflows — `ci` (per-module build/vet/staticcheck/tidy/test plus
the `guards` job), `images`, `e2e`, `itest`. **Nothing is filtered by path, on
purpose**: `ci.yml`'s header says why, and `MONOREPO_TODO.md` carries the
question of whether that should change. A documentation-only change therefore
costs a full run; that is known, not an oversight.

The `guards` job runs the `check-*.sh` scripts in `.github/scripts/`. **That
directory is the list** — this file deliberately does not enumerate them,
because a hand-maintained copy of a derivable list is the thing that goes
stale (rule 3). Each script's header says what it enforces and why. Between
them they cover in-repo `replace` directives, Dockerfile `FROM` pins and
compose `image:` pins.

Image references in **Go** are deliberately unguarded — every pattern narrow
enough to avoid hundreds of false positives also misses real references, and
rule 5 says ship none rather than a partial one. Keep them in a module's
`testutil` package with a named const and an env override, not inline in a
`_test.go`.

## Conventions

- **Never hand-transcribe a digest or a sha.** Resolve and apply it with one
  script reading its own output, and read shas rather than completing a prefix.
- **Verify both directions.** A guard that passes proves nothing until you have
  seen it fail on the defect it is for.
- Commit messages say *why*, and record what was tried and rejected. Several of
  this repo's subtleties are only written down in commit messages.
