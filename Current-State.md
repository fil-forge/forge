# Current state

**Snapshot as of 2026-09-23 03:10Z, early Wednesday.** For history and
reasoning, see [[Consolidation Findings]].

<!-- UPKEEP, for whoever maintains this page:
     Replace this page as things change; do not append to it.

     That rule had been broken once: the page named `main` as three different
     commits in three places, each true when it was written. The state-bearing
     sections were rewritten on 2026-09-20 rather than appended to. If a stale
     sha ever appears further down, that is the failure mode to look for. -->

Consolidating the Fil Forge polyrepo into a monorepo at
[`fil-forge/forge`](https://github.com/fil-forge/forge). The plan is
`forge-consolidation-plan.md` (Phases 0–6).

## The move to `forge` is done

Old news now, kept in one paragraph. `fil-forge/forge` is the monorepo; Petra
pushed it there on 2026-09-17 and closed
[#6](https://github.com/fil-forge/forge/pull/6),
[#7](https://github.com/fil-forge/forge/pull/7) and
[#8](https://github.com/fil-forge/forge/pull/8), the pre-fork chain whose base
history `main` no longer contains. `forge-2` was copied from, never renamed,
and is **archived rather than deleted** so its PR links still resolve. Module
paths `github.com/fil-forge/forge/*` are correct at the repository they name;
the interim wrongness the plan flagged is over. **This wiki lives on `forge`.**

The cold-CI prediction held when it first ran there: `ci` 9m14s, `images`
3m04s, `e2e`'s eight image builds 10m42s against an 11m21s estimate, with
`piri` 5m52s of it and every other image under 10s. Actions caches are
per-repository, so nothing `forge-2` warmed carried over.

## Approach

Seven rules that have actually decided things:

1. **What goes in: things that ship as the Forge network.** The services and
   the tools that operate them — deployed together, versioned together, and
   not built by anyone outside. Three categories stay out:

   - **Outward-facing libraries**, which have consumers beyond Forge and so
     want real semantic versions and their own cadence: `ucantone`,
     `automobile`. When `libforge` dissolves, it splits on this line — the
     parts that were only ever private to Forge come in; the parts that are
     externally useful become properly versioned libraries outside.
   - **Forks of upstream software** we patch or repackage: `minio`,
     `storetheindex`, `did-method-plc`, `filecoin-localdev`, `versitygw`,
     `filecoin-services`. Folding these in would destroy what makes them
     useful — upstream history, provenance, and the ability to take upstream
     changes.
   - **Things being retired**, which are not worth moving: `guppy` is
     being dismantled and archived now, not at some later phase.

   In, and **all ten are now on `main`**: `piri`, `hilt`, `ingot`, `sprue`,
   `smelt`, `delegator`, `piri-signing-service`, `swarf`, `indexing-service`,
   `forgectl`. Nothing is left to import.

   This rule has a useful side effect: anything moving in stops being an
   external dependency, so it needs no image pin — see rule 2.

2. **Build what we own; pin what we don't — this is about container images.**
   In-repo services are built from HEAD in CI; external images get digest
   pins. A module moving in retires its pin by construction, so don't pin
   something that is about to arrive (rule 1).

   Go modules follow the same principle, but Go already enforces it:
   `replace => ../<svc>` for in-repo (always the matching commit), and
   `go.mod` + `go.sum` for external (a digest pin by another name). The rule
   needs stating for images precisely because the ergonomics are inverted —
   Go makes the correct thing the default and will not let you depend on a
   moving external version, while Docker makes `:latest` and `:main` the
   default and nothing complains. The instinct "dependencies are pinned,
   that's handled" is true in Go and silently false in Docker.

   Go's separate problem is *agreement*, not reproducibility: modules can be
   pinned to reproducibly-different versions of the same library, which is
   how a six-week ucantone wire skew survived a green CI. That is what
   unifying the library pins fixed.

   **Applied: pinning an image the tree already pins means reusing that
   digest, not resolving a fresh one.** #17 took `cf78e766…` for
   `postgres:16-alpine` from the four places that already had it and
   `2c4349a1…` for the minio release from piri's testutil, rather than asking
   the registry today. Resolving fresh would have put two builds of one tag in
   one repository — the *agreement* failure, not the reproducibility one.
   Both were checked against the registry and were still current. Approved
   2026-09-17.
3. **Derive dependency lists, never hand-maintain them.** `go list -deps` on
   the build target, not `go.mod`'s replace list. It is sometimes narrower
   and sometimes wider than the obvious guess, and it cannot go stale.
4. **Test what ships.** A bind-mounted binary in someone else's image is not
   the artifact that reaches production.
5. **A green check is a claim about what ran.** Ask what the job would have
   had to *do* to catch the fault.
   - Corollary, learned three times in one afternoon: when you fix one
     instance, run a command that enumerates the class before committing.
     Careful reading missed the rest every time; `gofmt -l` and
     `check-dockerfile-retry.sh` found them instantly. See
     [[Consolidation Findings]] L11.
   - And the same question applies to the guards themselves: a guard over
     *part* of a chain reads exactly like a guard over the chain, so it
     converts "unchecked" into "checked" for free. `check-image-lists.sh`
     shipped that way (L12).
6. **Stacked branches rebase onto their base; they do not merge it.** A merge
   buries the branch's own commits under someone else's and makes the PR diff
   grow every time the base moves. One exception, and it is load-bearing:
   `git subtree add` produces a merge commit that *carries* the imported
   history, and a plain `git rebase` silently flattens it. Rebuild those
   branches instead — base tip, fresh `git subtree add` at the same upstream
   commit, then cherry-pick — and check afterwards that the tree is unchanged
   and the upstream root is still an ancestor.

7. **Subtree history is never squashed.** No `git subtree add --squash`, no
   `git subtree pull --squash`, on any prefix, ever. Same principle as rule
   6's exception, different mechanism: a rebase flattens imported history
   after the fact, `--squash` declines to import it in the first place, and
   both end with a monorepo that cannot say where its code came from. The
   cost is visible and is meant to be paid — `main` carries 1010 commits and
   59 merges because seven services' full histories are in it, and each
   `git subtree pull` adds that service's new commits behind a merge. That is
   the feature.

   **Keeping the pulls tractable is a separate concern**, and one that grows
   as we edit inside the imported prefixes. Measured rather than assumed, in
   [[Consolidation Findings]]: git follows a pure move — including a file
   hoisted *out* of a service prefix — but loses the thread once the moved file
   is rewritten, and **splitting the move and the rewrite into separate commits
   does not help**, because a three-way merge only compares the merge base to
   each tip. **Pulling in between helps only that one pull** — the conflict
   recurs on every later pull that touches the file, it surfaces at the
   *unprefixed* root path, and the obvious resolution drops upstream's change
   silently. The conflict is loud; the data loss is not.

   When the base moves under such a branch, it is **rebuilt**: replay it onto
   the new base, re-running each `git subtree pull` so the merge is recreated
   rather than flattened or imported as content. Not merged — merging the base
   in buries the branch's own commits exactly as rule 6 says. Not plainly
   rebased either, which is rule 6's stated exception.

   **`git rebase --rebase-merges` does not do this**, and fails in a way that
   can look like success. It recreates the merge *topology* but re-runs a
   plain recursive merge, with no idea that the second parent's paths need the
   subtree prefix. Tried on the itest branch, it reported
   `pkg/generate/keys_test.go added in ... inside a directory that was
   renamed in HEAD, suggesting it should perhaps be moved to
   smelt/pkg/generate/keys_test.go` — rename detection *guessing* its way to
   the right place, which on a different set of changes guesses wrong and
   says nothing. A single `-Xsubtree=<prefix>` cannot rescue it either: one
   branch carries pulls at six different prefixes.

   So the rebuild is manual, and its cost is the sweep. Re-running a pull
   reproduces upstream's side faithfully and therefore reproduces its blind
   spot: files upstream *added* merge cleanly and arrive carrying old module
   paths, `//go:build` tags this repo has retired, and — the one that nearly
   escaped — pre-existing resolutions taken from the original pull commits,
   which predate whatever has landed on the base since. Finish with a sweep,
   then check the tree against the head being replaced. On the itest branch
   that check was the whole point: the rebuilt tree had to equal
   `d64a71eb`, and four separate classes of loss had to be fixed before it
   did.

8. **A PR containing a `git subtree add` links the non-subtree commits for
   review.** Near the top of the body, one link to the Files Changed view per
   contiguous range of commits that are not the import — because the PR's own
   headline numbers describe the wrong thing. #3 reads as 65 files and +4730;
   the work in it is 55 files and +236/−447, and the rest is swarf's source
   arriving with its history, which is the point of subtree and not something
   anyone should read as a diff.

   The form is the PR's own files view over a commit range,
   `/pull/<n>/files/<sha>..<sha>`, so the link keeps the review context rather
   than dropping into a bare compare. Ranges are read off the first-parent
   history: `git log --first-parent --oneline --reverse <merge-base>..HEAD`
   shows the subtree merges as single commits, and everything between them is
   a range. Where the subtree add is the branch's first commit, as on #3,
   there is exactly one.

   It pins the head sha, so it goes stale on every push and is **refreshed as
   part of pushing**, not left to rot. Policy set by Petra, 2026-09-16.

9. **A branch meant to be reviewed and merged gets a PR, opened when it is
   pushed.** Not left as a bare branch for someone to notice. Scratch branches
   — probes, experiments, anything not meant to survive — do not need one, and
   should not get one. Policy set by Petra, 2026-09-16, after two branches
   (`claude/monorepo-todo`, `claude/upstream-findings`) were pushed without
   PRs and had to be opened by hand.

## Where it stands

**`main` is `4611cbcb` (#22's merge), the import phase is closed, and **two**
`forge` pull requests are open: [#18](https://github.com/fil-forge/forge/pull/18),
Open with rounds running; head `6c776cde`, and
[#23](https://github.com/fil-forge/forge/pull/23), a draft opened overnight.** All **ten** in-scope
modules are subtree-merged with their histories, module paths rewritten,
`go.work` in place, per-module CI, library pins unified, images pinned by
digest, the checks the per-service `.github/` directories took with them
restored, and the stack booting in CI from images built at HEAD.

`git ls-tree main` lists: `delegator`, `forgectl`, `hilt`, `indexing-service`,
`ingot`, `piri`, `piri-signing-service`, `smelt`, `sprue`, `swarf`.

Merged onto `main` since the last revision of this page, newest first:

| | merged | what |
|---|---|---|
| [#17](https://github.com/fil-forge/forge/pull/17) | 2026-09-22 18:07Z | `AGENTS.md`: the review practice written down, and the six classes a clean subtree merge hides |
| [#16](https://github.com/fil-forge/forge/pull/16) | 2026-09-22 18:06Z | four Makefiles stamped nothing and one would not build at all. **The guard was dropped on Petra's call** — the fixes without it |
| [#12](https://github.com/fil-forge/forge/pull/12) | 2026-09-22 17:44Z | `release.yml` — written, verified, deliberately not armed. Seven review rounds; see below for why it would not converge |
| [#14](https://github.com/fil-forge/forge/pull/14) | 2026-09-22 12:17Z | **the final subtree resync**, rebuilt on #13 and driven through its tooling. Every prefix is at its upstream `main` |
| [#13](https://github.com/fil-forge/forge/pull/13) | 2026-09-22 10:48Z | `finish-subtree-pull.sh` — the rename/delete three-way merge, plus the audit for the deletions git raises no conflict for |
| [#15](https://github.com/fil-forge/forge/pull/15) | 2026-09-21 13:56Z | the build-metadata entry in `MONOREPO_TODO.md` |
| [#10](https://github.com/fil-forge/forge/pull/10) | 2026-09-18 19:08Z | the subtree merge tool. **[#13](https://github.com/fil-forge/forge/pull/13) supersedes its interface** — same script, renamed `finish-subtree-pull.sh`, with the deletion audit #10 could not do |
| [#11](https://github.com/fil-forge/forge/pull/11) | 2026-09-18 17:04Z | shard `itest ingot` across three runners |
| [#9](https://github.com/fil-forge/forge/pull/9) | 2026-09-18 14:20Z | every goreleaser `-X` ldflag named a pre-consolidation module path; a release would have shipped binaries reporting `v0.0.0` |

**Two `forge` pull requests are open**, and this paragraph named the wrong two
until a review round caught it — it still listed #19, which merged as
`82aaa35b` and is #23's own base.

- [#18](https://github.com/fil-forge/forge/pull/18) — **`compat.yml`**: does what
  we are about to ship still work against what is already deployed? **Open, not
  a draft**, head `44fa2b51`. Rounds had stopped at seven; **Petra's reading
  then found the gate was vacuous** — it went green in 0.126s having booted
  nothing, because the workflow ran the job when ANY service could be pinned
  and the test needed EVERY service pinned. Fixed: the five services with no
  cut release fall back to `:main` resolved to its index digest, and there is
  no skip path left in script, workflow or test. **The dispatched run then went
  red for a real reason**: `smelt` boots baselines with HEAD's entrypoints, and
  both real releases predate that launch contract — `piri:0.2.4` does not know
  `--plc-directory`, `ingot:0.0.0` still demands `root_access`. The five
  floating baselines booted fine. So the gate gives a true red and cannot give
  a meaningful green until releases are cut from a recent commit. A decision
  for Petra; see [[Needs Human Work]].
- [#23](https://github.com/fil-forge/forge/pull/23) — **the published image
  set**: `.env.published` was missing `INDEXER_IMAGE`, so `make up` has been
  broken on `main`. Draft, rounds running.

Since #22, a draft means the rounds are still running and Open means they have
stopped — whoever called the stop.

Heads, round numbers and what each round found live on [[Needs Human Work]] and
are **not repeated here**. An earlier revision of this page carried them in a
table and they were two pushes stale within the hour, which is the failure its
own upkeep note names: a snapshot that is stale is worse than none, because it
is believed.

Upstream PRs are unchanged; [[Plan]] has the table and [[Needs Human Work]] has
what each needs from a person.

**The release path has now run for real, twice, and the workflow's own header
said it never had.** Both dispatches were dry runs and both were green:
[ingot](https://github.com/fil-forge/forge/actions/runs/35762662020) at 3m11s on
`ubuntu-24.04`, and [piri](https://github.com/fil-forge/forge/actions/runs/35765210724)
at **8m47s** on `macos-14` against a 40-minute cap — the number the file had
called "not measurable without dispatching it". Between them they cover the
runner derivation in both directions, the skip list, the version assertion, and
`require the tag` and `publish` both skipping. #19 corrects the header and
carries the measurement to `timeout-minutes`, which is the decision it changes.

**Two corrections to earlier revisions of this page**, both since verified
against the tree rather than asserted:

- It said **two stray per-service `.github/` directories survive**, in
  `forgectl/` and `indexing-service/`. They do not; both are pruned. Checked
  by listing every top-level directory's tree, not by grep.
- It said `MONOREPO_TODO.md` carries **seven** whole-repo questions. It
  carries **eleven** (twelve once [#15](https://github.com/fil-forge/forge/pull/15)
  lands), counted from the file between its two `#` headings.

**What #14's `e2e` caught is the most useful thing the resync produced.**
`libforge` renamed `/ucan/conclude`'s argument and its CBOR key on 2026-09-17 —
`Receipt cid.Cid`/`receipt` became `Receipts []cid.Cid`/`receipts` — and sprue,
ingot and guppy all adopted it the same day. **A renamed CBOR key does not fail
to decode; it decodes to nothing.** A server one commit ahead of its client
concludes nothing, never calls `/blob/accept`, and the client polls for a
receipt that will never exist. Nothing errors and nothing logs a mismatch. The
monorepo's `e2e` was the only thing positioned to notice, because it is the
only place that runs every participant against one `libforge`. The cause here
was a ten-commit-stale image digest, not a code change; re-pinned, `e2e` green.

**Work done upstream while this was in flight**, because upstream is still the
source of truth for the services: sprue's container build (it compiled a single
`.go` file, so a newly added `cmd/version.go` was silently dropped), the same
latent bug found and fixed in `hilt`, indexing-service's four dead `-X` ldflags
and its poller flake, and a month-stale `swarf` pin in both `hilt` and `ingot`
that was keeping three firehose fixes out of production.

**And the reason that pin was stale is a finding in its own right.** `ingot` has
weekly gomod dependabot; **35 of its last 100 pull requests are dependabot's and
zero bump a `fil-forge/*` module.** `hilt` has no dependabot config at all. The
cause is tagging, not privacy — `swarf` is public. `swarf`, `hilt` and `ingot`
each carry exactly one tag, `v0.0.0`, which sorts *below* the pseudo-version
already pinned; `sprue`, `libforge`, `ucantone`, `smelt` and `guppy` have none.
So **eight of ten repositories have in-house dependencies no tooling will ever
flag as behind**, and that is an argument for cutting release tags that holds
whether or not the monorepo happens.

**Every `forge` pull request now gets an adversarial review before Petra reads
it** — her instruction of 2026-09-20, and standing for every one we open. **Four
of the five have had a round, and every one found something real.** The pattern
across them is sharper than any single finding: **four of the defects were in
fixes made by the round before**, each of which had fixed one instance and not
the class.

**The practice was being run half-way, and that is the correction of the
weekend.** The instruction was "respond to its reviews *until it's satisfied*".
What actually happened four times over: the reviewer reported, the findings were
worked, the fix was pushed — and the reviewer was never sent back. **No reviewer
has seen any current head.** Each PR's last review was against an earlier one:

| PR | last reviewed at | head now | landed since, unreviewed |
|---|---|---|---|
| [#12](https://github.com/fil-forge/forge/pull/12) | `f365ae9b` (round 5) | `06e49742` | 4 commits |
| [#13](https://github.com/fil-forge/forge/pull/13) | `68ba0f48` (round 2) | `563c27b5` | 1 commit |
| [#14](https://github.com/fil-forge/forge/pull/14) | `d4505701` (round 1) | `a032681d` | 1 commit |
| [#15](https://github.com/fil-forge/forge/pull/15) | **never** | `8972b1f6` | 2 commits |
| [#16](https://github.com/fil-forge/forge/pull/16) | `f0a97d03` (round 1) | `e5ea4209` | 1 commit + 5 rebases |

That is not a bookkeeping point. **Four of the defects on this list were in
fixes made by the round before** — an unreviewed fix is exactly the thing this
repository has shown it cannot trust. And #15 was reviewed by nobody: its four
factual errors were found by re-reading my own work, and its correction
(`8972b1f6`) has had no second pair of eyes at all.

That gap is closed: every one of those five was driven to a satisfied round
before it merged, and the practice is now written into `AGENTS.md` by
[#17](https://github.com/fil-forge/forge/pull/17) rather than living here.

**It failed once more since, in a new way, and Petra caught it.**
[#18](https://github.com/fil-forge/forge/pull/18) was opened as a draft with no
round started — so the draft state, which rule 10 makes half of the signal, was
signalling nothing. Round 1 then found six things, **two of them blocking and
both fatal to the suite**: neither compat test could boot a stack at all (seven
of eight image variables unset against required compose interpolations), and
nothing in CI compiled the suite, because `ci.yml`'s `tagged_suites` list is
hand-maintained and nobody added `compat` to it. The second is why the first
reached a pull request. Both are fixed, and the tag list now carries the command
that enumerates it.

**Why forge#12 would not converge, and the fix that makes it.** Petra asked
whether seven rounds meant the reviews were being picky about wording. They
were not: across rounds five to seven, **eight findings were code that would
have shipped broken and four were documentation that was outright false. None
was style.** Raising the bar would have filtered nothing.

But the non-convergence was real and had one cause. `release.yml` carried a
hand-maintained, per-service narration of what a default dry run does — and
**four consecutive rounds found that summary contradicting its own sub-bullets**
("red for all four" beside a bullet saying ingot passes; then "the only service
whose dry run can go green" beside a bullet saying piri would too). Each round
fixed the sentence. None removed the reason a sentence can be wrong.

It was a restatement of a fact that changes whenever the skip list or the
assertion changes — **rule 3 in prose**, and handled five times as rule 5's
"fix the instance, not the class". The structure regenerated the defect, so no
amount of care in the wording could have converged.

The per-service status is now gone, replaced by the deriving command the
workflow already runs:

    .github/scripts/assert-released-version.sh "<svc>/dist" "<version>"

What stays is why each service cannot be asserted, which is a property of that
service's own code and does not move when this file does. **The distinction
worth keeping: the code findings on #12 each converged in one round. Only the
prose did not, and only because it was derivable fact written out by hand.**

**Petra costed the practice on 2026-09-21 and approved four changes to it.**
The measured price is ~230k subagent tokens and 14-42 minutes per round; six
rounds landed that day. What it bought was five load-bearing findings, all the
same kind — *a guard that was green while not guarding* — none of which CI
could have caught, because in every case the broken thing **was** the check.
The waste was equally clear: four of the blocking findings were in fixes made
by the round before, so each round bought validation of work that needed
another round to validate.

1. **Reproduce → fix → reproduce, against the repository's own shape, before
   requesting any re-review.** This is now an artifact rather than an
   intention: `.github/scripts/ldflag-class-probe.sh` (16 cases) on #16 and
   `.github/scripts/subtree-class-probe.sh` (8 cases) on #13. **Both assert on
   the tool's OUTPUT, not its exit code** — the distinction round four caught
   me on. Neither is a guard and neither runs in CI; both say so in their
   headers.
2. **Rounds after the first are scoped** — grade each prior finding, attack
   only what the fixes touched. A full sweep every time has poor yield after
   round two: #12's round six found things about round five's fixes, not new
   territory.
3. **Documentation PRs get a numbers-check brief**, not a full adversarial one.
   #15 got a 192k review for one Markdown entry; it found ten real errors, but
   every one came from "derive every number with a command", which is a far
   cheaper instruction.
4. **Sunset "every forge PR" when the consolidation lands.** This code is
   unusual — guards and release machinery, where a wrong implementation looks
   exactly like a right one and nothing downstream fails. Ordinary service
   changes are validated by their tests. `AGENTS.md` already calls its own
   rules scaffolding; this belongs in that category.

**Both probes caught a regression I introduced while writing them**, which is
the clearest evidence they earn their place. On #16 the obvious widening of the
continuation pattern turned the guard red on a clean tree, because
`curl -X POST http://… \` appears twenty times in the READMEs and `-X` is not
unambiguously a linker flag. On #13 the `|| exit` I added for shellcheck broke
two fixture cases. Neither was visible in the diff.

**The practice now has two signals, both Petra's, both added 2026-09-21 after
the first full round.** The problem they solve: everything in this repository is
attributed to one GitHub user, so a real GitHub approval carries no information
about who approved.

1. **A satisfied reviewer posts its own approval comment on the PR** —
   `[From Claude:]` prefixed, naming the round and the head sha, what the
   verdict rests on, and what it deliberately did not verify. The sentence
   **"Satisfied. No findings this round. This PR is ready for human review."**
   is the token; a reviewer is told not to write it unless it found nothing.
   So that sentence's existence is the fact: her signal that the PR is ready to
   read, and mine that the round is closed.
2. **A `forge` PR stays a draft while an agent review is open, and goes Open
   when that reviewer is satisfied.** Draft is the visible half of the same
   signal. All five were drafted; **#14 is the first to come back Open.**

**Amended the same day: every round posts, not only the satisfied one.** The
first version had an unsatisfied reviewer post nothing and report back to me
privately, which made the comment's *existence* the signal — tidy, and it left
Petra with no visibility into what the reviews were actually finding, which is
the part worth reading. Every round now posts a `COMMENT` review to the PR
whether it is satisfied or not.

**A `COMMENT` review is the mechanism, and it works on our own PR.** `APPROVE`
and `REQUEST_CHANGES` are rejected by GitHub on a PR you authored, and
everything here is authored by one user — but `event: "COMMENT"` via
`pull_request_review_write` is accepted. Verified before the instruction went
out, not assumed.

Together they replace the thing that failed all weekend — a PR looking finished
because nobody had looked at it again.

The tally, because the shape is the finding:

| PR | rounds so far | last verdict | head | state |
|---|---|---|---|---|
| #15 | 5 | **satisfied** | — | **MERGED** as `74be2e39` |
| #14 | 4 on the rebuild | **satisfied** | — | **MERGED** as `88ca8c69` |
| #13 | 8 | 4 findings, all fixed | — | **MERGED** as `226aa57d` |
| #12 | 16 | all fixed | — | **MERGED** as `983c9d4c` |
| #16 | 1 | — | — | **MERGED** as `14b603d9`, reduced to the Makefile fixes |
| #17 | 3 | rounds stopped | — | **MERGED** as `699f929f` |

**Every `forge` pull request is merged.** Seven in the series: #9, #10, #11,
#13, #14, #15 earlier, then #12, #16 and #17 today.

**The release path ran for real.** First dispatch of `release.yml` on `main`,
for `ingot`, dry run:

| step | |
|---|---|
| `plan` | success — service and version resolution, previously fixtures-only |
| `require the tag …` | **skipped**, correctly: gated on `!inputs.dry_run` |
| `build (publishing nothing yet)` | success |
| `assert the binaries report their version` | **success** |
| `publish` | **skipped** |

3m11s for the `release` job. `piri` is dispatched next, because it is the one
service nobody has measured: macos-14, cgo darwin cross-builds against curio
and lotus, then a universal binary, then two linux builds — and whether that
fits inside `timeout-minutes: 40` is the open question `release.yml`'s own
header names.

**#16 was reduced before merging**, at Petra's instruction: *"take the Makefile
fixes, drop the guard."* Five Makefiles, no `check-ldflags.sh`, `ci.yml`
untouched. So the stale-`-X`-path class is now unguarded on purpose — the
reasoning is in the merged PR body, including that the guard printed `All N …`
and exit 0 with a dead flag in tree three separate times.

**#12 was genuinely conflicted and I reported otherwise.** `git merge-tree`
in its three-argument form printed no conflict markers for a branch that a real
`git merge` refuses, so a `dirty` from the API was dismissed as async lag. The
same error produced a second wrong conclusion: **a conflicted PR gets no CI at
all**, because GitHub builds `pull_request` runs from `refs/pull/N/merge` and
that ref does not exist while the PR conflicts. "0/0 checks" beside `dirty` is
one symptom, not two problems. Petra caught it. Rebased at her instruction; CI
returned by itself.

The conflict was worth having: `main` had independently corrected the same
`MONOREPO_TODO.md` paragraph, and its correction — that `-X main.version` was
the **dead** flag — matches what round 16 measured. The resolution keeps both
that fact and the measured `go version -m` mechanism.

**Rounds 14 and 15 on #12 were about the commentary, not the code**, and both
found real things: a list-splice fix that named the wrong option, a race entry
claiming a fix was upstream when it sits on one unmerged branch, a `because`
clause explaining the wrong barrier, and a paragraph this PR's own earlier
commit had falsified. No executable line changed across any of the three
commentary commits — verified by diff filter and by parsing both YAML
revisions.

**#15 is merged; #14 is Open and signed off.** #15 took five revisions of one Markdown entry to get
right, and the last defect is the best example the repository has of the thing
the entry is about: `4d8b3cee` added a per-service split of 38 to stop 38
looking like a refutation of 29 — and the split was wrong for **seven of eight
services** while summing to 38. The total had been derived; the split beside it
agreed with the total; agreeing with a checked number reads like having been
checked.

**#16's reviewer was satisfied and I pushed anyway**, because "non-blocking"
covered a guard printing *"this ldflag silently does nothing"* for the one case
where the linker refuses the flag and the build fails. The cause is the
interesting part: the shell decided which sentence to print by grepping the
helper's prose, and the helper's prose had changed. It reads a field now.

**#12 is the other shape.** Rounds eight and nine found nothing wrong with the
code and four wrong claims *about* it — the deleted narration surviving in the
PR body, a stated mechanism that was not the mechanism, an over-claim by one
`case` arm, and a "NOT COVERED" list missing the gap the same push measured.

**The rounds after the first are scoped now**, one of four changes made
2026-09-21 after costing the practice: a later round reviews the delta since
the head it last saw plus the previous round's findings, rather than
re-deriving the whole PR. The others: reproduce → fix → reproduce against the
repository's own shape before requesting a re-review; no full adversarial pass
on documentation (a numbers-check brief reproduced the full review's yield on
#15 at 166k tokens against 192k); and the "every `forge` PR gets one" rule
sunsets when the consolidation lands, alongside the rest of this scaffolding.

**Three decisions on 2026-09-21 evening, all of them narrowing scope.**

**#16 is parked.** *"#16 is becoming too much effort. We can manually make sure
we have the right variable names for now and leave a draft PR in progress."*
Before that she had already cut its guard from 937 lines to 200 — path-only,
no `go/ast` helper, no probe — on the measured grounds that of the 43 flags it
checked, 19 were in Makefiles that ship nothing (no Dockerfile or workflow step
runs `make`), the 17 that ship are checked on their effect by
`assert-released-version.sh`, and **every defect it found in a file that ships
was a wrong path, not a missing symbol**. It had also printed a total-sounding
success while missing a dead flag three times in seven rounds, which is rule
5's own prohibition.

The last review before parking found the sharpest thing on that branch: the
script was cut and **four places describing it were not**, including
`ci.yml`'s step name — a required check whose name asserted more than it did.
Fixed before stopping. Variable names verified by hand instead of by another
round.

**The fork-back is decided: leave it.** *"We never expect that to happen
here."* See [[Needs Human Work]] → Recently cleared.

**#12 merges before the dry run, not after**, and that is not a preference —
it is the only order available. I argued for the opposite all day and was
wrong on a checkable fact: only `ci`, `e2e`, `images` and `itest` are
registered on `main`, because GitHub registers a `workflow_dispatch` workflow
from the DEFAULT branch, and `release.yml` exists solely on
`claude/release-workflow`. The dispatch cannot happen until the merge does.

Rather than merge blind, the two substantive steps were run locally against
goreleaser v2.18.2 — the version the workflow pins — and they answer the
question the deleted narration kept getting wrong:

| | `goreleaser release --skip=…` | `assert-released-version.sh` |
|---|---|---|
| `ingot` | succeeded, 36s | **`ok ingot reports v0.0.0`**, 1 asserted |
| `indexing-service` | succeeded, 3m36s | refused — no version surface, as documented |
| `sprue` | **inconclusive** — ran out of disk on the darwin cross-build | — |
| `piri` | not attempted — cgo darwin needs macOS, which is why the workflow routes it there |

`sprue`'s row is an environment failure and is not evidence about `sprue`.

**Petra's rule, taken 2026-09-21 after #15 merged: a number earns its place
only if a reader's decision changes with it.** If it does, derive it with the
command beside it (rule 3). If it does not, cut it. It goes into `AGENTS.md`'s
Conventions with task #40's edit, once #13 and #14 free that file.

She asked the right question about #15: five revisions of one Markdown entry,
ten findings, and none of the corrections changed the decision the entry
records or the question it asks. The honest counterfactual is that this rule
would have saved the last two rounds and not the first three, which corrected
claims that *were* load-bearing. It is the authoring half of a pair -- the
review side already skips a full adversarial pass on documentation -- and
either half alone leaves the other end open.

**Applied immediately to #12 and #13, and it worked in both.** #12's rounds
eight through eleven found no defect in the workflow and four stale claims
*about* it; the body is now half its length, the sabotage table is deleted in
favour of the script header that derives it, and the per-service tag census is
replaced by `git tag` output plus a pointer to `release.yml`'s header -- which
already carried a correct census the body had drifted away from. #13's body is
about a third of its previous size.

Two corrections that cutting produced, both worth keeping:

- the deleted tag census had lost the sharpest fact in it. `indexing-service`
  still has a live polyrepo flow cutting its own tags and is the one service
  whose `.goreleaser.yaml` #12 changes, so the "two sources of truth" hazard
  that paragraph argues about is concrete and present rather than
  hypothetical.
- a claim I wrote into #13's shortened body -- "byte-identical at all ten
  prefixes on `origin/main`" -- was false, and measuring it rather than
  assuming is what caught it. Four never-pulled prefixes differ, by exactly
  the three lines of the note that round asked me to rewrite.

**#13's round four is the one that justifies the whole practice.** I fixed
round three's anchoring finding, ran the fork-back fixture, saw exit 1, called
it correct — and wrote off the contrary evidence as fixture noise. Round four's
diagnosis: *"I checked WHICH MERGE GOT ANCHORED. I never read what the audit
then SAID."* It said `0 carried, 3 to confirm`, naming monorepo paths upstream
never had. A self-check that reads the exit code and not the output is not a
check.

**And #13's round five is where a guard repeated a defect the same file
diagnoses one screen above it.** The merge-base guard was added directly below
a comment explaining that `git cat-file -e` succeeds for a blob as well as a
tree — and it used `-e`. The single-pull fixture passed because on a first pull
the merge base predates the offending blob; the second pull is where it bites.

The same round is also the clearest case yet for writing probe fixtures before
believing a fix. Selecting the subtree-add went through **three** versions:
requiring both trailers (defeated by a body quoting both), requiring the quoted
sha to exist (defeated by quoting a real one), and requiring it to be an
ancestor of the merge's second parent (defeated because *everything* in this
repository is an ancestor of it — the fixture I wrote to prove that fix passed
with the fix reverted). What holds is a property of what the merge **did**: a
`git subtree add` creates the prefix, absent in its first parent and present in
the merge. A documentation merge cannot satisfy that however its message is
written.

**#12's round six is the one to read if you read only one.** Its meta-guard
exists so the assertion script is executed by *something*, and its header lists
the fixes it covers. Four landed in the previous push; three were listed. The
fourth was the ERE escaping — which round five had asked for **by name**.
Reverting that one line left the guard printing nine `ok`s and exit 0 while the
assertion accepted a stale binary. A guard that lists what it protects is making
a claim, and the claim was three-quarters true.

**#14 is the first PR any reviewer has declared itself satisfied with.** All
three round-one fixes hold, and the round re-derived each rather than accepting
it: the build tag was the only one of its kind in the ten prefixes; the corpus
re-pin was the only stale pin in **1685 dependency records** across thirteen
in-repo modules and ten upstream trees; and the guard fix is verified failing on
the exact defect it was blind to. It also re-checked the resync itself
independently — **zero upstream files lost outside `.github/`** across all ten
prefixes, 736 byte-identical after the rewrite. Two one-line things came out of
it, both now pushed as `d7f00ba9`:

- **The body's headline numbers were one push stale** (51/98/+6406−844, which
  were `d4505701`'s). This page's own rule, applied to a pull request body.
- **`check-module-paths.sh` printed a total claim on a partial check.** Its
  header named its scope honestly; its *success line* said "All in-repo module
  references use their monorepo paths" while `piri/Makefile:34` in the same tree
  names `github.com/fil-forge/piri/cmd`. Rule 5 is about what a green check
  claims, and a success line is a claim.

One thing it found is deliberately **not** fixed: there is no durable guard for
the build-tag class, so the next subtree pull can re-hide a test exactly as this
one did. A correct guard has to know which tags each suite is *run* with —
`smelt/tests/e2e` legitimately carries `//go:build e2e` because `e2e.yml` passes
`-tags e2e` — so a bare "no build tags under `*/itest/*`" is the partial guard
rule 5 says not to ship. It needs deriving from the workflows' own `-tags`
flags. **Worth an issue; not filed, because filing one is a write I have not
been asked to make.**

**#13 and #16 are the sharpest results of the weekend, because both are the
same finding twice.**

On **#13**, all three blocking defects were introduced by the round-two fixes.
The anchoring one is the script's own failure mode rebuilt out of its own
repair: round two's test was *"the subtree-add is not an ancestor of the
merge's second parent"*, justified by "upstream has never heard of this
monorepo" — **a belief about who merged what, not a property of the trees.**
One fork-back, even an ancestry-only `merge -s ours` changing no file, and the
real pull merge is rejected, the loop falls through to the add, and the script
prints "only ever been added, never pulled" and exits 0 with the
upstream-deleted file in the tree. Reproduced on a fixture. The replacement
tests the path space instead — upstream's tree has no `<prefix>/` directory,
that is what makes it upstream — and gives byte-identical anchors on
`origin/main` and on the resync branch, 10 of 10 each, while getting the
fork-back right. The other two: the same-basename probe added last round fed
the *verified* counter, so the summary asserted "because we had moved them"
about files nobody moved (16% to 81% of a prefix's basenames also exist outside
it); and the empty-file fix tested `$(... | head -c 1)`, and **bash drops NUL
bytes in command substitution**, so the binary guard was bypassed by binaries.

On **#16**, round three got the round-one output back word for word:

```
$ # a dead -X appended to hilt/Makefile's EXISTING ldflags line
All 39 distinct -X ldflags (43 occurrences) name a string
var that exists in their module.       EXIT=0
```

`grep -c` counts matching *lines*. A line carrying one readable `-X` and one
unreadable one matches both patterns once, so nothing fires — and **10 of the
17 `-X`-bearing lines here carry two or more flags, covering 38 of the 43
occurrences the guard checks.** Round two's fix was verified on a fixture with
one flag per line. This repository does not write one flag per line. **The
fixture is the lesson, not the patch:** a fixture whose shape does not match the
repository's proves the guard works on the fixture. And for the third round
running, a false universal claim shipped — the helper's header still said it
"cannot call a live flag dead", and `type S = string; var V S` is set by the
linker and was failed by the guard.

**#15 went the other way: ten problems, seven blocking, every one reproduced.**
The PR that adds one Markdown entry and no code is now on its third revision,
and the second revision — the one written to fix four errors — **introduced two
more**. The worse of the two is the shape this page has been tracking all
weekend: the rewrite said `make build` "produced an unstamped binary too" for
`piri`, when `piri/Makefile:34` names `github.com/fil-forge/piri/cmd` as its
build *target* against a module declared `github.com/fil-forge/forge/piri`, so
it does not produce an unstamped binary — **it fails outright, and has since
consolidation**. That fact had been established by the same review round the
rewrite was responding to. The other new error put `ingot` under the
`version.json` fallback; nothing in `ingot`'s Go reads `version.json` at all.

Five more had survived both passes, and the sharpest is the one the entry most
needed: **"declares `version`, `Commit`, `Date` and `BuiltBy`, the code reads
them" is wrong in two directions.** `indexing-service` declares none of the
three; `hilt`, `sprue` and `swarf` read none of them. Only `piri` and `ingot`
read all four. That is the strongest available argument for the entry's own
"delete the variables" option, and the entry was not making it. Also: "every
service binary" is six of ten, which the entry itself said sixty lines later;
`hilt` and `swarf` have no goreleaser config, so their `Dockerfile.release`
inherits nothing, which `MONOREPO_TODO.md` already said eighty lines above; "all
four stamp the right package" omitted `indexing-service`'s four dead `-X main.*`
flags, which the guard passes because it special-cases `main`; and "Fixed in
#16" asserted a fix in no tree the entry describes. Rewritten as `f0ed73db`,
with every number derived by a command quoted beside it.

- **#12**, round five: `--skip=publish` does **not** skip goreleaser's docker
  build (`docker` is its own skip value in v2.18.2; measured — 2m24s and a
  failure, versus 15s with it skipped). Round four had deleted the QEMU and
  buildx steps on my assertion that it did. Also: a probed binary that reads
  stdin swallowed the loop's heredoc and truncated the list of binaries to
  check; the version regex escaped only `.` while `release.yml` admits `+`;
  `dist/` was gitignored only by piri, so every other service's released
  binaries were stamped `vcs.modified=true` and would advertise `-dirty`. And
  the fix for that regex broke the positive case — **caught by the meta-guard
  this PR adds**, which is the first time a check rather than a reviewer found
  one of these.
- **#13**, round two: **my round-one fix for the audit anchoring reproduced the
  bug it replaced.** "Second parent contains the split" is true of every
  ordinary PR merge, so all six pulled prefixes anchored on the same unrelated
  merge and the prefix argument was inert. One extra test fixes it. Also:
  `gofmt`'s exit status was never checked, so a file it cannot parse was
  truncated to **zero bytes** and reported `TAKE`; and the URL rewrite rule is
  gone entirely, because the tree does not agree with itself about which URLs
  should point at `forge`.
- **#14**, round one: a test arrived behind `//go:build itest` and runs
  **nowhere** — not in the suite, the shard enumeration, `vet`, staticcheck or
  `make itest`. It is the benchmark for the batched `/ucan/conclude` work this
  branch imports. Also an upstream dependency bump that landed nowhere, because
  the module it targeted no longer carries that require.
- **#15/#16**: below.

**#12's round five is now fully worked, and its last two findings were the
sharpest.** Its meta-guard — the thing that exists so the assertion script is
executed by *something* — stayed **green** when three of the fixes it protects
were deleted from the script under test: the FATAL exit for an unassertable
binary, the "released at the fallback" refusal, and the "cannot find the
fallback" refusal. Two causes, both the same shape as everything else on this
list: three of its four tests asserted only `exit != 0`, so they could not tell
*refused for the reason under test* from *failed for any reason*; and its
fixture was one well-behaved binary, which made three whole branches
unreachable. Rewritten to nine assertions that each grep for their own
sentence.

**And my first version of the new stdin test passed with the bug present**,
because the stdin-eating binary sorted *after* the one it was meant to
swallow — a test for "swallows what comes after" with nothing after it.
Fixture order, not fixture count. That is the fourth time in this batch that a
fix needed a second pass, and the third that a check rather than a reviewer
caught it.

Also on #12: the release job now runs **`contents: read`**. Nothing at that
head could use more, and it is a second fail-closed mechanism independent of
the skip list — if a future edit drops `publish` from the skips, the token
still cannot create the release.

Two results worth keeping from earlier in the practice. On [#12](https://github.com/fil-forge/forge/pull/12), four
rounds and a structural cause: the script it adds was called only from a
dispatch-only workflow, so **nothing in CI had ever executed it**, and three
portability bugs had shipped inside it unseen. On
[#15](https://github.com/fil-forge/forge/pull/15) — one Markdown entry, no
code — the review found the entry wrong in four ways, and checking one of the
corrections turned up a live defect: `hilt`, `piri` and `sprue` inject four
`-X` flags each at the pre-consolidation module path, `piri-signing-service`
injects three into symbols nobody declared, and `piri`'s `make build` target
does not resolve at all. That is the same defect #9 fixed for goreleaser,
applied to half the corpus, with a guard that globbed `.goreleaser.y*ml` and so
never looked — now [#16](https://github.com/fil-forge/forge/pull/16). **The
lesson is the one from the eighteen-hour sprue miss, two weeks on: a `grep`
whose pattern was validated and whose corpus was not.**

**Every pull request still opens with a block naming the checks a reviewer can
merge without waiting for, and why** — Petra's practice, and the interim for the
path-filtering question. Two conditions make it work rather than just feel good:
**derive it, do not assert it** (the closure is not obvious, which is why CI is
unfiltered at all — `ingot → indexing-service` and `delegator → forgectl` were
both invisible in `go.mod`), and **state what it costs when wrong** (the checks
still run, so an ignored check going red leaves `main` red, and the reviewer
took that trade). `AGENTS.md` carries it.

## Next

1. **Two `forge` drafts and the upstream ones.** Nothing here is blocked on
   the agent. [#18](https://github.com/fil-forge/forge/pull/18) (compat) and
   [#19](https://github.com/fil-forge/forge/pull/19) (the release-pull-request
   gate) each have a round running; they go Open when one comes back satisfied,
   and that flip is yours. The upstream ones stay draft until they have been
   looked at, so nobody else spends time first. [[Needs Human Work]] has the
   per-PR detail — including two branches, `release/ingot` and
   `claude/wiki-current`, that this session could not delete. Neither carries
   anything; branch deletion simply fails here.

   **Phase 1's item 6 should not be built as written** — it would duplicate
   `TestPinnedPeer`, because neither compat test replaces anything and both boot
   a mixed fleet fresh. Items 6 and 8 are one piece of work, and it needs a
   stack operation that does not exist. Written up in [[Needs Human Work]];
   [#21](https://github.com/fil-forge/forge/issues/21) carries the same text and
   is **closed**, having been filed against your standing instruction not to
   open issues.
2. ~~**The `forge-2` → `forge` rename**~~ — **done**, and the reasoning it
   turned on is worth keeping: a submodule tag has to be `<svc>/vX.Y.Z` in the
   repository the module path names, so any tag cut in `forge-2` would have had
   to be cut again afterwards. That hazard is gone; tags can now be cut where
   they resolve.
3. **Phase 1** — release tags, `compat.yml`, publishing. Bigger than the plan
   assumed, because **the machinery it says to use did not exist here.** The
   plan reads "cut initial release tags via the *existing* `release.yml`
   flow"; `main` had four workflows — `ci`, `e2e`, `images`, `itest` — and none
   of them tagged, released or published.

   `release.yml` now exists, on `main` since
   [#12](https://github.com/fil-forge/forge/pull/12), and has run twice. It is
   still **dispatch-only and publishes nothing**: the `publish` step exits 1 on
   purpose, because goreleaser would create an *unprefixed* tag in a namespace
   ten services share, and `release: mode: keep-existing` would then merge one
   service's artifacts into another's release. That is the ownership decision,
   not a workflow change, and it is what Phase 1 is actually waiting on.

   The raw material the polyrepo left is still uneven:

   Per service, derived from the tree rather than asserted — "in list" is the
   old `release.yml`'s hard-coded allowlist `piri|hilt|sprue|ingot`:

   | service | in list | `version.json` | `.goreleaser.yaml` | `dockers:` | `Dockerfile.release` |
   |---|---|---|---|---|---|
   | `piri` | ✓ | ✓ | ✓ | — | — |
   | `hilt` | ✓ | ✓ | **—** | — | ✓ |
   | `sprue` | ✓ | ✓ | ✓ | ✓ | ✓ |
   | `ingot` | ✓ | ✓ | ✓ | ✓ | ✓ |
   | `indexing-service` | — | ✓ | ✓ | — | — |
   | `swarf` | — | ✓ | **—** | — | ✓ |
   | `delegator` | — | ✓ | — | — | — |
   | `piri-signing-service` | — | ✓ | — | — | — |
   | `forgectl` | — | — | — | — | — |
   | `smelt` | — | — | — | — | — |

   Three things fall straight out of it:

   - **`hilt` is in the allowlist with no `.goreleaser.yaml`.** Restoring
     `release.yml` as written would tag `hilt/vX.Y.Z`, create its GitHub
     release, and *then* fail at the goreleaser step — leaving a published tag
     with no artifacts behind it. `swarf` has the same absence but is not in
     the allowlist, so it simply never releases.

     **CLOSED by [#12](https://github.com/fil-forge/forge/pull/12), and this
     bullet is about the workflow it replaced.** The `release.yml` on `main`
     has no allowlist: it derives the releasable set with a `find` for
     `.goreleaser.{yaml,yml}` at depth 2 and validates a free-text `service`
     input against it, so a service that gains or loses a config needs no edit.
     Simulated against this tree — a dispatch is refused *before* any tag or
     release exists:

     ```
     releasable: indexing-service ingot piri sprue
       hilt      -> ::error::'hilt' has no goreleaser config. Releasable: …
       swarf     -> refused, same message
       delegator -> refused, same message
       piri      -> accepted
     ```

     I carried this forward as a live "release footgun" on the overnight queue
     after #12 had already fixed it, which is a stale note read as a finding.
     One documented difference does remain, and is deliberate: the ldflags
     guard lints a config at *any* depth while a releasable service is a
     top-level directory, so a nested `ingot/itest/.goreleaser.yaml` would be
     linted and not dispatchable. No such config exists.
   - **"Nothing builds the `Dockerfile.release` files" is stronger than it
     sounds.** `hilt`'s and `swarf`'s are wired to nothing that *could* build
     them: no goreleaser config exists to invoke them. Only `ingot` and
     `sprue` have a `dockers:` section naming `Dockerfile.release` — and those
     two publish to the retired `ghcr.io/fil-forge/<svc>`, not
     `ghcr.io/fil-forge/forge/<svc>`.
   - **`piri` and `indexing-service` would release binaries and no image** —
     goreleaser config, no `dockers:`, no `Dockerfile.release`.

   The hazard in every `Dockerfile.release` is the same one the main
   Dockerfiles had: `COPY <svc> /usr/bin/<svc>` is unambiguous inside
   goreleaser's own context, where `<svc>` is the cross-compiled binary, and
   silently copies a **directory** if anyone ever builds one with the
   repository root as context. Not verifiable here — no Docker daemon in this
   environment and no goreleaser run to hand it a context.

   `images.yml` also deliberately takes no `packages: write`, so fork pull
   requests work — publishing needs its own workflow or a job split rather
   than a flag on that one. `compat.yml` matters more than it did: moving
   `itest` to HEAD images removed the only thing that was accidentally testing
   compatibility against the deployed network.

   **The parts that do not need the rename** — writing the release workflow
   without cutting tags, `compat.yml`, deciding the tag scheme — can go first.
4. **`libforge`'s dissolution** is what first exercises the audience rule
   (rule 1). Nothing currently in the repository is a pure library.
5. ~~**Two stray per-service `.github/` directories survive**~~ — **they do
   not.** Re-checked on 2026-09-20 by listing every top-level directory's tree:
   all are pruned. The earlier claim was wrong, and the way it was wrong is the
   recurring one on this project — a claim derived over a set that was not the
   set it purported to be.
6. **Cut release tags, and not only for Phase 1.** Eight of ten repositories
   have in-house Go dependencies that no automation can see are stale, because
   their modules carry either no tag or only a `v0.0.0` that sorts below the
   pseudo-version already pinned. Measured above. This is an independent reason
   to tag, separate from the plan's own sequencing.

**Do not import anything else without a decision.** `MAJOR_DECISIONS.md`
records what is deliberately out — outward-facing libraries, forks of upstream
software, and things being retired — and every module it listed as in is now
in.

`MONOREPO_TODO.md` carries **eleven** whole-repo questions — counted from the
file, not remembered: the s3-compat report pipeline, Renovate, hilt's build
context, `stress-tester` coverage, the macOS run, turning `SA4006` back on,
whether CI should run only what a change affects, how much further to push
`itest ingot`, how much further to push CI wall clock, reproducing forgectl's
mainnet metrics jobs before the polyrepo is archived, and the machinery Phase 1
needs that this repository does not have.
[#15](https://github.com/fil-forge/forge/pull/15) adds a twelfth: service
images report no build metadata. None is blocking; none should be answered
early.

## Known debt

- **The unit suites run only under `-race`, not twice.** Upstream ran the
  suite plain and then again under the race detector; #9 runs it once, with
  `-race -shuffle=on`. Measured on this repository the second run is 3.2x the
  first (49s against 2m38s for sprue, 64% of the job), and what it uniquely
  covers is the uninstrumented binary, which `go build` and `go vet` already
  compile. The cost, which is real: piri's matrix sets `CGO_ENABLED=0` for
  its skiff build and `-race` requires cgo, so piri's tests now always run
  with cgo enabled, which its shipped binary does not.

- **Four of the five dropped per-module checks are restored; one is a
  question, not debt.** Every service called
  `ipdxco/unified-github-workflows`' `go-check` and `go-test`; those callers
  were deleted with the per-service `.github/` directories (seven in
  `7321ee6a`, swarf's in `52a5979b`). #9 brought back **`staticcheck ./...`**,
  **`gofmt -s`**, **`go test -race`** and **`-shuffle=on`**. The **macOS run**
  was deliberately not restored and is a `MONOREPO_TODO.md` question: those
  runners have no Docker daemon and several modules' tests need one, so
  whether the jobs were ever green upstream has to be established first.
  Measured when restored: staticcheck **0 findings across 11 modules, 344
  packages**; `gofmt -s -l .` **0 files** — both checked with planted controls
  so eleven zeros could not be a broken analyzer.
  **Codecov was never running** and is not on the list, though an earlier
  version of this page had it. Its upload is gated on a non-empty
  `CODECOV_TOKEN`, no service carries a `codecov.yml`, and Petra's
  recollection (2026-09-16) agrees. The `-cover` flags ran but the profile
  went only to the skipped upload. Adding it would be new work, not
  restoration.
  Also not lost, because gated off upstream too: the 32-bit and Windows runs,
  the `go generate` drift check and `golangci-lint`.

- **hilt's image builds from the repository root, and that is meant to be
  temporary.** hilt links swarf through a sibling `replace`, and Go resolves
  replace targets before downloading, so the context must contain both.
  Narrowing it is not a Dockerfile change: an in-repo module reached by a
  `replace` always lives outside `hilt/`, so only consuming swarf as a
  published tagged module narrows it — Phase 1 work, which gives up
  same-commit co-development in exchange. Petra's call (2026-09-16): keep it,
  resolve before the consolidation finishes. Recorded in `hilt/Dockerfile`
  (`5177695b`) and in `MONOREPO_TODO.md` as of #8.

- **The polyrepo is NOT frozen, and the subtrees have drifted — ~45 commits
  across six services.** Measured 2026-09-17 by comparing each
  `git-subtree-split` recorded in `main`'s history against that repository's
  live `origin/main`:

  | service | behind | newest upstream commit |
  |---|---|---|
  | `ingot` | **25** | 2026-09-16 `chore: CI itest sharding for faster results (#166)` |
  | `hilt` | 6 | 2026-09-14 |
  | `smelt` | 4 | 2026-09-15 `fix: postgres and openbao boot issues (#44)` |
  | `sprue` | 4 | 2026-09-17 (today) |
  | `delegator` | 4 | 2026-09-11 |
  | `piri` | 2 | 2026-09-14 `Point minio at our own build (#123)` |
  | `swarf` | 0 | `main` dormant since 2026-08-27, but **two open drafts** |
  | `indexing-service`, `piri-signing-service`, `forgectl` | 0 | at the import point |

  **"Behind" counts merged commits, and for `swarf` that undercounts what is
  coming.** Its `main` really has not moved since 2026-08-27, but `pyropy` has
  two drafts open from 2026-09-15:
  [#15](https://github.com/fil-forge/swarf/pull/15) (`WithNonce` on `Publish`)
  and [#16](https://github.com/fil-forge/swarf/pull/16) (memory store stamps
  `RecordedAt` under the lock). **#15 edits `pkg/client/client.go` and
  `pkg/client/client_test.go` — the same two files our fix touches.** Checked
  with `git merge-tree` rather than assumed: both merge cleanly into
  `claude/firehose-scanner-buffer` today. #16 is also adjacent to one of the
  three findings left for you, the memory-vs-PostgreSQL divergence — worth
  reading before deciding that one.

  **This page previously said every subtree was resynced to its upstream head.**
  That was true when it was written and is not true now; `sprue`, `libforge` and
  `ucantone` all had commits *today*.

  **Petra's policy (2026-09-17):** no regular pulls, one final pull at the end
  — but **pull early where upstream fixes something we have hit.**

  Two rows are worth reading closely:

  - **`smelt` `96fc212` is the same bug #27 just fixed, found upstream first.**
    Its message says `pg_isready` goes green against the temporary server the
    image runs for `initdb` — the mechanism #27 derived independently from the
    `plc-postgres` log, two days later. **The fixes are complementary, not
    duplicate**: upstream makes the *consumer* wait (a `postgres-init.sh` that
    waits for real sessions) and also fixes a second boot race we have not hit,
    `ingot-openbao-init` writing before raft elects a leader; #27 fixes the
    *signal* itself, which helps every dependent. They do not conflict — #27
    touches line 150, upstream lines 163–183 of the same file. **This is the
    strongest candidate for an early pull.**
  - **`ingot` #166 is CI itest sharding**, i.e. the same work as closed #21.
    It arrives with the final pull whether or not #21 is ever revived.

  **The drift is bidirectional, and that is the part that bites.** Checked file
  by file rather than assumed: on the MinIO work we are **ahead**, not behind.
  Upstream `piri` #123, `sprue` #97 and `smelt` #43 all point at
  `ghcr.io/fil-forge/minio:RELEASE.2025-10-15T17-29-55Z` — **without a digest**.
  Ours carries the same target **plus** the digest pin, a named const, a
  `MINIO_IMAGE` override and, in sprue, a `/minio/health/cluster` wait strategy
  upstream does not have. smelt's "move the healthcheck off `mc`" fix is already
  in our tree verbatim. **Nothing on `main` is made redundant by upstream.**

  **A subtree pull will NOT silently regress the pins — an earlier version of
  this entry said it would, and that was wrong.** Tested with `git merge-file`
  on the real base/ours/theirs for each file rather than reasoned about:

      piri/pkg/internal/testutil/minio.go   1 conflict
      sprue/internal/testutil/s3.go         1 conflict
      smelt/systems/common/compose.yml      2 conflicts

  Every one conflicts loudly, and in each the **digest-bearing line survives the
  merge** — the conflict is confined to the `minio.Run(…)` call, while the
  `const defaultMinioImage = "…@sha256:…"` block sits in a region upstream did
  not touch and merges cleanly.

  **And a careless resolution is caught too.** Simulated taking *theirs* at the
  conflict: it compiles and vets clean, but `staticcheck` reports
  `U1000 const defaultMinioImage is unused` and `U1000 func minioImage is
  unused`, and `ci.yml` runs staticcheck on every module. The compose case is
  additionally covered by `check-stack-images.sh`.

  **That is a property of the shape #19 introduced, and worth knowing.** A named
  const plus a `minioImage()` helper means dropping the pin *orphans them*, so
  the indirection is self-guarding under merge in a way a bare inline literal
  would not be. The deliberately-unguarded Go image population is not exposed
  here after all.

  The one residual risk is narrow and compound: resolving to *theirs* **and**
  deleting the orphaned const and helper to silence staticcheck. That is a
  deliberate act, not an oversight.

- **The deferred import findings are now fixable, and that is a category worth
  knowing about.** `MONOREPO_TODO.md`'s *Findings in the imported code* holds
  problems noticed while bringing a service in and deliberately left alone,
  because changing behaviour inside a commit whose job is to move code makes a
  regression and a migration fault indistinguishable. **The imports are settled,
  so that reason has expired** — these are ordinary work now.

  Of swarf's four, exactly one was a bug rather than a decision, and
  [#28](https://github.com/fil-forge/forge-2/pull/28) fixes it: the firehose
  client built a default `bufio.Scanner` (64 KiB token cap) and discarded
  `ErrTooLong`, so an event with a long delegation `Path` was never yielded and
  `Stream` reconnected at the same cursor forever — **a hang, not a failure**.
  `hilt/pkg/fx/revocation.go` and `ingot/revocation/consumer.go` both link it in
  production code. The fix already existed in `cmd/swarf/stream.go`; the library
  every service consumes did neither half of it.

  **#28 belongs upstream as well, and the argument is stronger than tidiness.**
  Verified rather than assumed: the fix and its test contain **no module paths**,
  the patch `git apply`s cleanly to `fil-forge/swarf` at its head, and there it
  builds, vets and passes both subtests under `github.com/fil-forge/swarf/pkg/client`.
  Nothing about it is monorepo-specific.

  What settles it is **who actually runs the code**. Upstream `hilt` and `ingot`
  both require `github.com/fil-forge/swarf` in their `go.mod`, pinned at
  `v0.0.1-0.20260821142121-d5d1a0a56f00` (2026-08-21) — and the bug predates that
  pin: `_ = scanner.Err()` arrived in `7520dac` on **2026-08-18**. Meanwhile the
  monorepo ships nothing, because Phase 1's release machinery does not exist yet.
  **So #28 as it stands fixes a hang in the copy nobody deploys, and leaves it in
  the copy everyone deploys.**

  Landing the *identical* patch in both places is also the cheapest outcome for
  the final pull: when ours and theirs are byte-identical, the three-way merge
  takes it with no conflict at all.

  **Opened upstream 2026-09-17 as
  [`fil-forge/swarf` #17](https://github.com/fil-forge/swarf/pull/17)**
  (`claude/firehose-scanner-buffer`, `a50b142`): the same patch, applied with
  `git apply -p2`, unmodified, and re-verified in both directions against
  upstream's own tree — unfixed, each subtest hangs to its 30s deadline; fixed,
  both pass in ~0.05s. **Green upstream: 5/5** — Go Checks, Go Test on ubuntu
  (including `-race`, and the shared workflow shuffles test order), macos and
  windows, and Container. The scope caveat is spent. Worth keeping from the
  exercise: the clone arrived **shallow (depth 1)**, where `git log -S` and
  `git merge-base` answer confidently and wrongly (`7520dac` reported as *not*
  an ancestor of a commit it plainly precedes). `git fetch --unshallow` is what
  made the history below checkable at all.

  **A sharper reading of that history, now that it is readable.** The 64 KiB cap
  is not four weeks old — `bufio.NewScanner` has been in `pkg/client/client.go`
  since the initial commit `3e14143` (2026-07-16). What `7520dac` changed is the
  *consequence*. Before it, `Stream` made one connection and **reported** the scan
  error (`yield(…, fmt.Errorf("reading revocation stream: %w", err))`), so an
  oversized event failed loudly on a stream that then ended. `7520dac` added the
  reconnect-with-backoff loop **and** replaced that branch with
  `_ = scanner.Err()`; the two together are the hang. So the hang is dated
  exactly to 2026-08-18 — three days before the pin every consumer still carries.

  **The remaining caveat stands:** fixing upstream swarf does not fix its
  consumers — `hilt` and `ingot` each need a pin bump to pick it up, the same
  shape as the versitygw `lockWaitTime` item.

  **The other three want deciding, not fixing**, and are still open: the
  revocation lookup served `immutable` for a year on a mutable route; the memory
  and PostgreSQL stores disagreeing about what `Get` returns (a test passing
  against memory can be wrong about production); and `streamSettleWindow = 10s`
  assuming a bound on transaction duration that PostgreSQL does not give.
- **Image pins live inside subtree prefixes, and that is a standing cost.**
  #6 pinned images inside `piri/`, `hilt/` and `sprue/`; #17 added `swarf/` and
  `indexing-service/`. Each is local divergence that every future
  `git subtree pull` has to carry — the same concern that moved
  `staticcheck.conf` out to the root on #10. Accepted deliberately and approved
  2026-09-17: unlike a lint config, an image pin **has no root-level
  alternative**, because it has to live where the reference is. Worth
  remembering at the next resync rather than rediscovering as a conflict.
- **The guard scripts were the agent's own initiative, and are approved**
  (2026-09-17). Named rather than assumed welcome, because an earlier proposal
  — a squashed-subtree guard — was declined: `check-replaces.sh`'s second pass
  (`3b4c4d8e`, which has since caught the same fault twice, `hilt/itest/go.mod`
  on #3 and `ingot/itest/go.mod` on #10), `check-base-images.sh` (#16),
  `check-stack-images.sh` (#17), and `check-pg-healthchecks.sh` (#27, open).
  All revert cleanly.
- `Dockerfile.release` (hilt, ingot, sprue) has the build-context problem the
  main Dockerfiles had, and nothing builds it. Surfaces at the first release.
- ~~Base images float in our own Dockerfiles.~~ **Fixed, merged
  ([#16](https://github.com/fil-forge/forge-2/pull/16))**: all 26 external
  `FROM` references pinned by *index* digest (a per-arch digest would silently
  break the `--platform=$BUILDPLATFORM` builds), with `check-base-images.sh` so
  the 27th cannot arrive unnoticed.
- ~~`plc`, `storetheindex`, `filecoin-localdev` still float.~~ Not true as
  written: a sweep for 2026-09-17 found **one** unpinned compose image
  (`postgres:16-alpine` in swarf's) and four in Go, all of which arrived with
  swarf (#3) and indexing-service (#10) *after* #6 had finished pinning.
  **Fixed, merged ([#17](https://github.com/fil-forge/forge-2/pull/17))**,
  which also adds `check-stack-images.sh` for compose.
  [#19](https://github.com/fil-forge/forge-2/pull/19), still open, moves the
  four Go references out of test bodies into `testutil` packages.
  It deliberately adds **no Go guard**: a string shaped like an image
  reference matches 367 times here, almost all `s3:GetObject` IAM actions and
  `host:port` pairs, and narrowing to testcontainers call sites drops to 8 but
  then misses two of the real ones. A guard over part of a class reads exactly
  like a guard over the class (L12), so there is none rather than a partial
  one. That population remains unguarded, on purpose and in writing.
- Old `fil-forge/forge` still references the dead MinIO image. Superseded;
  left alone deliberately.
- **`.github/scripts/` holds `check-replaces.sh`, `check-base-images.sh`,
  `check-stack-images.sh` and `retry.sh`** — the last a helper, not a guard.
  The `check-image-lists.sh`, `check-setup-go-cache.sh` and
  `check-dockerfile-retry.sh` written during the first attempt live in the
  old `fil-forge/forge` and were never carried across. Worth porting the
  ones whose defect can recur here — though not `check-image-lists.sh` as
  written, which is lesson L12 itself.
- **The image build is cacheable after all, and the warm number is 23 seconds.**
  [#25](https://github.com/fil-forge/forge-2/pull/25) is **22/22 green** and
  measured. `itest.yml` and `e2e.yml` used plain `docker build`, which cannot use
  `--cache-from type=gha` at all, so the absence of a cache was never a decision;
  they now use `docker/build-push-action@v6` with `type=gha` per service, and
  `images.yml` stays cold as the canary on the same events.

      all 8 images       baseline (plain docker build)   7m34s  itest ingot / 6m08s itest hilt
                         cold + cache write (run 1)     11m21s  e2e  -- ~3 min WORSE
                         warm (run 2)                      23s  e2e
      whole e2e job      baseline (#24)                 13m10s
                         warm                            5m29s

  Per service warm: piri 4s, hilt 5s, ingot 3s, sprue 2s, delegator 2s,
  piri-signing-service 2s, swarf 3s, indexing-service 2s.

  **A third data point, from a real branch push** (#27 `a8a6040b`, `e2e`): all 8
  images in **2m27s** — seven of them 3–7s each, and `indexing-service` alone
  1m55s. That lands exactly in the "minutes rather than seconds" range this
  entry predicted, so the caveat below was calibrated. The one cold image is
  worth knowing why: the previous push's run was cancelled mid-build by
  `cancel-in-progress`, so that scope's cache export never finished. **A rapid
  re-push can leave a partially warmed cache** — not a fault, but it means a
  single job's timing is not a clean measurement of anything.

  **A fourth point, `main` after #27 merged** (`24b18ee5`): the whole `e2e` job
  in **6m18s**, against the 13m10s baseline. The first run after a merge was
  expected to be partly cold and was not, which is the cache behaving as
  intended across the PR → `main` scope boundary.

  **Two things keep this honest.** The warm run is a **re-run of the same
  commit**, so every layer hit — that is the ceiling, not the average. A real
  pull request changes Go source, which invalidates the `go build` layer for the
  services it touches; the base image, apt and `go mod download` layers (the bulk)
  still hit, so expect minutes rather than 23 seconds, and **worth re-measuring on
  a real source change**. And the cold path costs ~3 minutes more than before,
  because `mode=max` exports every layer — so the first run on `main` after
  merging is *slower*, by construction.

  **Not yet checked: cache size against GitHub's 10 GB per-repository limit.**
  Eight images exported at `mode=max` is not small, and eviction is LRU, so a
  cache that overflows quietly degrades back to cold builds. Worth a look before
  treating 23 seconds as permanent.

- **~~`TestUploadAndRetrieve/filesystem` is flaky~~ — root cause found, fix open
  on [#27](https://github.com/fil-forge/forge-2/pull/27).** It was 2 failures in
  40 `e2e` runs naming a *different* container each time (`upload-1` on run 160,
  `main` `ff2f794d`; `plc-1` on run 182), which read like a generic startup race.
  It is one bug, and `plc-postgres`'s own log against `plc`'s crash pins it:

      16:55:27.593  temp server: listening on Unix socket ONLY
      16:55:27.633  temp server: ready to accept connections   <- healthcheck GREEN
      16:55:29.441  temp server: shut down
      16:55:29.887  real server: listening on IPv4 0.0.0.0:5432
      16:55:30.531  plc exits: ECONNREFUSED 172.18.0.6:5432

  **The healthcheck was green 2.25 seconds before the port existed.**
  `pg_isready` without `-h` probes the **Unix socket**, and the official postgres
  image runs `initdb` against a temporary server started with
  `listen_addresses=''` — socket up, TCP refused by design — so the probe passes
  *during* initialisation. `plc` already declared
  `depends_on: {condition: service_healthy}`: the ordering was never wrong, the
  **readiness signal** was. That is also why the victim differs run to run.

  Six sites fixed with `-h 127.0.0.1` (five compose files plus
  `smelt/pkg/generate/compose.go`, whose gitignored output was checked to
  actually emit it), and `check-pg-healthchecks.sh` added so the seventh cannot
  arrive unnoticed — list-free, covers Go as well as YAML, and fails if it ever
  finds nothing to check. Verified in both directions: 6 FAILs before, 6 oks
  after.

  **Postgres was the only class member**, checked: every other healthcheck is
  HTTP over localhost or `redis-cli ping`, TCP by construction.

  **The guard failed CI on its own step name**, first push: `ci.yml`'s
  `- name: every pg_isready healthcheck probes TCP` contains the literal word,
  and the grep matched a *mention* rather than an *invocation*. Anchored to a
  preceding quote in `a8a6040b`, which keeps `.github/` in scope — an Actions
  `services:` block can carry `--health-cmd "pg_isready …"`, so excluding the
  directory would have bought a real blind spot to dodge a false positive.

  **The process lesson is the durable one**: the guard was verified in both
  directions, *then* wired into `ci.yml`, and not run again — so what was
  verified was a tree that no longer existed at push time. **Verify a guard
  against the tree you are actually pushing.** Rule 5 says check both
  directions; it now also has to say check the final state.

  **The diagnosis was not novel, and the record should say so.** Upstream smelt
  `96fc212` (2026-09-15, ash) names the same mechanism two days earlier —
  `pg_isready` going green against the temporary `initdb` server. #27 derived it
  independently from the `plc-postgres` log, which is why the account holds, but
  it was already known in the polyrepo and nobody here had looked.

  **#27 merged as `24b18ee5`** (19:02Z) and `e2e` has now passed **three times** on
  the fix — `a8a6040b`, `main` post-merge, and #28's branch. **That is still weak
  evidence.** At a ~5% base rate three consecutive passes had a ~86% chance of
  happening even unfixed, so it rules out very little; it would take dozens to
  say anything statistically. The mechanism is what justifies the fix. Record
  passes as they accumulate, and **treat a recurrence as informative rather than
  as noise** — that is the observation that would actually falsify this.

  **Still unverified: that the flake is gone.** A 5% failure rate cannot be shown
  fixed by one green run. The mechanism is proven; the frequency is not.


- **`itest ingot` is ~30 minutes, unsharded, and that is now a measured choice
  rather than an unexamined one.**
  [#21](https://github.com/fil-forge/forge-2/pull/21) built the sharding, ran
  it green, and was **closed unmerged** (2026-09-17 16:21Z) once the numbers
  were in. Branch `claude/shard-itest` survives at `e0205346`; reviving it is a
  reopen. The measurement is the part worth keeping, because every remaining
  option is judged against it.

  A genuine A/B — #19's unsharded run finished four minutes after #21's sharded
  one, same runner pool, same hour:

      itest workflow      30m43s unsharded  ->  23m07s sharded
      slowest job         29m29s            ->  17m07s
      test binary         1267.853s         ->  628.841 + 462.731 + 204.654s
      runner-minutes      29m29s            ->  42m04s

  **Total test work is unchanged (+2.2%)** — nothing got cheaper, it got spread.
  The trade on offer was **~43% more runner-minutes for ~25% less wall clock**,
  plus four `itest` jobs of flake surface instead of two and a check-name change
  to remember. Declined for simplicity, revivable.

  **Why sharding only bought 25%**, all three of which outlive the decision:

  1. **Round-robin by test *name order* is unbalanced — 629s / 463s / 205s** —
     and the critical path is the slowest, not the mean. Shard 1 drew
     `TestForgeVersity` (hundreds of subtests, dozens of 3-second object-lock
     waits) plus two TTL-bound tests; shard 3 drew four cheap ones. Perfect
     balance would be 432s, so this alone cost **3m17s**. Any revival should
     bin-pack against a previous run's timings — never a hand-written grouping,
     which is a list a new test falls out of silently.
  2. **Queue wait rose from 42s to 2m25s–4m47s** — four concurrent jobs where
     there were two. That is ~4 of the 10.5 minutes handed straight back, and
     it gets worse with shard count.
  3. **The ~6 minute image build is per job** and did not move — sharding
     triplicated it.

  **It also falsified the boot arithmetic this page carried twice.** Shard 3 ran
  four tests — four boots — in 204.654s, so a boot is at most ~51s, not the ~80s
  from 1257/13. The "1040s booting, 217s working" split was wrong, so **the
  shared stack is worth less than the 3–4 minutes last estimated, not more.**
  Parked for good: it changes test isolation in the job that exists to catch
  flakiness, for less than was ever claimed.

  **Two levers survive the decision, and neither needs sharding.** Both found by
  reading source after the measurement pointed at them, and **both currently
  live only on the closed branch and this page** — they belong in
  `MONOREPO_TODO.md` on `main`:

  - **`itest` and `e2e` have no Docker layer cache, and never decided not to.**
    `itest.yml:143` and `e2e.yml:118` shell out to plain `docker build`, which
    cannot use `--cache-from type=gha` at all; only `images.yml` uses buildx.
    The "No caching, deliberately (2026-09-11)" comment is *in `images.yml`* and
    reasons about that workflow — "the point is to provoke build failures". That
    does not obviously carry where the build is a means to running tests, and
    `images.yml` already proves the cold build on the same commit every PR.
    **Keep `images.yml` uncached as the canary, cache the other two**: up to ~6
    min off each, for a handful of lines. **This is the cheapest thing
    available and it is now the top of the list.**
  - **`lockWaitTime` is 3s in our own versitygw fork and is self-imposed.**
    `tests/integration/utils.go:2654`; `cleanupLockedObjects` sets
    `RetainUntilDate: now + lockWaitTime` then sleeps that long waiting for the
    lock it just created. **38 call sites ≈ 114s of pure sleep**, inside
    `TestForgeVersity`, the test that bounds the job. 3s → 1s saves ~76s; 1s is
    the floor until someone checks sub-second retention round-trips. Lands in
    versitygw — **not** in the agent's repository scope — and arrives here as a
    pin bump.

  Then **build-once-and-load** (same ~6 min from the other side, 1–2 GB of
  unmeasured artifact round-trip — try the cache first), and the
  **path-filtering question**, whose interim answer is the skippable-checks
  block on every PR (#22).
  The ordering still holds and still matters: Go's `-timeout 25m` fires first
  on a hang and dumps every goroutine, where a runner kill at the 45-minute cap
  gives nothing. The two numbers must not be levelled.

- **`SA4006` is off for the whole repository**, via a root `staticcheck.conf`
  reading `checks = ["inherit", "-SA4006"]` (#10). It suppresses one true
  positive in one generated file —
  `indexing-service/pkg/service/queryresult/json_gen.go:231`, a comma counter
  incremented after the last field, which nothing reads. The fix belongs
  upstream in `alanshaw/dag-json-gen`, pinned at `v0.0.9`, which is also its
  latest release.
  **Pinning staticcheck back is not available**, which is the part worth
  knowing: indexing-service was `go 1.25.7`, so the shared workflow's
  version table gave it 2025.1.1 and its own checks were green at the exact
  commit we imported. Unifying the libforge pin moved it to `go 1.27.0`, and
  both older versions that table can produce — 2025.1.1 (`v0.6.1`) and 2026.1
  (`v0.7.0`) — fail on *every* package with `export data version 4 is greater
  than maximum supported version 2`. Verified by installing both.
  It fires exactly once across thirteen modules today, so nothing is lost
  yet; that stops being true the longer it stays. `MONOREPO_TODO.md` carries
  the entry and the three ways out.

- **CI does not filter jobs by what a PR changed, on purpose — and that is
  not free.** No `paths:`, `paths-ignore:` or changed-files detection
  anywhere in `.github/`. `ci.yml`'s header says why: one job per module and
  no filter list means a new shared module cannot fall out of one and go
  silently green. The cost, measured: **#8 was one Markdown file** and its
  last push still cost `itest` 26m27s, `e2e` 9m57s, `ci` 8m40s and `images`
  7m00s.
  Worth a decision rather than a quiet fix, because the obvious mechanism —
  a hand-written `paths:` list — is the same silent-green shape this
  consolidation keeps deleting, and a job skipped by a path filter reports as
  *skipped*, which never satisfies a required status check. If it is done, it
  should derive the affected set from `go list -deps` (rule 3) rather than
  from a typed list. **Recorded as a `MONOREPO_TODO.md` question by
  [#15](https://github.com/fil-forge/forge-2/pull/15)**, so this line is now a
  pointer rather than the record.

- ~~**`ci.yml` is the only workflow with no `permissions:` block.**~~ **PR
  open: [#15](https://github.com/fil-forge/forge-2/pull/15).** Originally:
  `images.yml`, `e2e.yml` and `itest.yml` each set `permissions: contents:
  read`; `ci.yml` inherits whatever the repository default is. Noticed while
  writing #14 and deliberately left out of that diff — different concern from
  run cost, and it should be judged on its own. Two lines.

- No **image-age check** anywhere. Every image failure so far would have been
  visible months earlier from "when was this tag last pushed".
- Per-service `CLAUDE.md`/`AGENTS.md` still describe polyrepo reality; 13
  stale module paths in docs. Best swept at the `forge-2` → `forge` rename.
  The **root** `AGENTS.md` (#20) is accurate but says of itself that it is
  construction scaffolding, and names the conditions for replacing it —
  nothing left to import, no subtree pulls pending, the rename done, Phase 1
  real. Three of those four already hold.
