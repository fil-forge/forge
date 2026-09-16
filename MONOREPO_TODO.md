# Monorepo TODO

Two kinds of thing the consolidation turns up and cannot deal with where it
finds them.

**Questions for the whole repository**, below, which only make sense once the
monorepo is built and can be looked at as a whole. Each needs a team decision
rather than an implementation. Add to it when a branch turns up a question it
cannot answer by itself: say what was found, why it is not decidable yet, and
what someone would have to choose — not what you would do.

**Findings in the imported code**, at the end: bugs noticed while moving a
service in, which are not the migration's to fix. Add to it when a review of
an import turns up something that was already true upstream.

Neither is a place for ordinary work, which belongs in issues. What is
blocking or waiting on a person right now lives on the wiki's
**Needs Human Work** page.

---

# Questions for the whole repository

## Restore the s3-compat report pipeline

Upstream `ingot` publishes an S3 compatibility report to GitHub Pages
(`2365944c`, [ingot#131](https://github.com/fil-forge/ingot/pull/131)). The
consolidation brought the test suite across and left the reporting behind:
there is no s3compat step here, no `s3-compat-report` artifact, and no `pages`
job anywhere in the repository.

**Why it waits.** A report pipeline has to publish *somewhere*, and where that
is depends on decisions the monorepo has not made yet — whether services keep
per-service Pages sites or share one, and what the release flow looks like
once Phase 1 adds tags and publishing. Rebuilding ingot's version now would
bake in the polyrepo's answer to a question the monorepo gets to ask fresh.

**The choice.** One Pages site per service, one shared site, or drop the
published report and keep the suite's own output.

## Turn Renovate on

`renovate.json` is in the repository as of
[#6](https://github.com/fil-forge/forge-2/pull/6), covering the 18 images the
stack pulls plus the Go and Actions dependencies. It does nothing until the
Renovate GitHub App is installed on the organisation.

**Why it waits.** Installing it is an org-level action with org-level
consequences: it opens PRs across every repository it is granted, so the
scope, the schedule, and who triages the PRs are decisions for whoever owns
that surface — not for the branch that happened to write the config.

**Until then**, the pins are frozen rather than maintained. That is a real
cost and a deliberate one: a frozen pin is still reproducible, which an
unpinned tag is not, and #6 existed because a mutable `:main` tag broke a
conformance test.

**The choice.** Install it and decide the scope, or adopt something else
(Dependabot covers Docker and Go but reads `docker-compose` files less
willingly), or keep bumping by hand and accept the drift.

## Narrow hilt's Docker build context again

`hilt/Dockerfile` builds from the repository root, because hilt links swarf
through `replace => ../swarf` and Go resolves every replace target before it
downloads anything — a context scoped to `hilt/` dies in `go mod download`.
piri and ingot are the same. Recorded in the Dockerfile itself.

**Why it waits.** Narrowing it is not a Dockerfile change. An in-repo module
reached by a `replace` always lives outside `hilt/`, so no restructuring
helps — not even extracting an `internal/client/swarf`, which would just be a
different sibling directory. The only thing that narrows the context is
consuming swarf as a published, tagged module through the proxy, which is
Phase 1 work and gives up same-commit co-development in exchange.

**Agreed 2026-09-16**: keep it as it is for now, resolve before the
consolidation is finished. So this one has a decision already; what it needs
is Phase 1 to happen.

## Decide what to do about the macOS test run

Each service's own CI ran its tests on macOS as well as ubuntu: the shared
go-test workflow defaults to `["ubuntu", "windows", "macos"]` and every
service's config skipped only Windows. The monorepo runs ubuntu only, and
[#9](https://github.com/fil-forge/forge-2/pull/9) restored the other four
lost checks while deliberately leaving this one alone.

**Why it waits.** It is not clear the macOS jobs were ever green. Several
modules' tests boot containers through testcontainers, and GitHub's macOS
runners have no Docker daemon — so either those jobs were failing upstream,
or something not visible from here supplied one. Reproducing a job that was
already red buys nothing, and macOS runners bill at a higher multiplier than
ubuntu, so this is not free to find out by trying.

**The choice.** Establish what those runs actually did — one look at a recent
`Go Test` run on any of the eight upstream repositories settles it — then
restore macOS, restore it only for the modules that do not need Docker, or
decide ubuntu-only is what the monorepo wants and say so.

---

# Findings in the imported code

Problems noticed while bringing a service in, and deliberately not fixed by
the branch that found them: changing behaviour inside a commit whose job is to
move code makes a regression and a migration fault indistinguishable. Recorded
here so they are not lost with the review thread. They belong in issues
against the owning code once someone picks them up — nobody has filed them
yet.

Each was checked against the tree rather than taken on the reviewer's word.

## swarf

Raised by the Copilot reviewer on
[#3](https://github.com/fil-forge/forge-2/pull/3), and present at `c43af97`,
the commit the subtree imported — so none of these are migration damage.

### The revocation lookup is served as immutable for a year, and is not

`swarf/pkg/fx/app.go:348` sets `public, max-age=31536000, immutable` on the
by-delegation lookup. But the route is mutable: the schema permits several
rows per `revoked_delegation`, and `Get` returns the newest of them —
`swarf/pkg/store/postgres/store.go:83` is `ORDER BY recorded_at DESC, id DESC
LIMIT 1`. A cache may therefore serve a superseded record for a year, and
`immutable` tells it not even to revalidate.

On a revocation endpoint that fails in the wrong direction: a client holding
the cached older record sees a *narrower* revocation than the one actually
recorded.

It interacts with the next finding, and the two want deciding together. If
"newest matching" is the intended contract then the caching is wrong; if the
endpoint is meant to be immutable then it should be content-addressed by cause
CID, and `Get`-by-delegation is the wrong route shape.

### The memory and PostgreSQL stores disagree about what `Get` returns

`swarf/pkg/store/memory/store.go:64` keys records by the revoked delegation, so
a second revocation of the same delegation overwrites the first. PostgreSQL
keeps every row and returns the newest. The two backends therefore answer the
same question differently after a repeated revocation, and a memory-backed
stream cannot satisfy the interface's "all revocation records" contract.

The overwrite is the small part. The divergence is the real one: a test that
passes against the memory store can be wrong about production.

### The firehose client drops oversized events and then hangs

`swarf/pkg/client/client.go:259` builds a default `bufio.NewScanner`, which
caps a token at 64 KiB, and line 286 is `_ = scanner.Err()` under a comment
saying the caller reconnects. A valid event can exceed the cap, because
`api.FirehoseRevocation.Path` may carry many CIDs. `ErrTooLong` is then
discarded and `Stream` reconnects at the same cursor indefinitely, never
yielding the record and never returning an error — it presents as a hang
rather than a failure, which is the worse of the two.

**The fix already exists in this repository, in the wrong place.** The CLI
raises the limit and checks the error — `cmd/swarf/stream.go:79` is
`scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)` and line 101 is
`if err := scanner.Err(); err != nil` — while the library every other service
consumes does neither.

### The stream's settle window assumes a bound on transaction duration

`swarf/pkg/store/postgres/store.go:22` defines `streamSettleWindow = 10 *
time.Second`, documented as bounding "how long an insert may take between its
`recorded_at` (NOW() at transaction start) and its row becoming visible".

That is an assumption, not a bound. PostgreSQL assigns `NOW()` at transaction
start, so an INSERT that blocks for longer commits with a `recorded_at`
already behind the cursor, and the stream misses that revocation permanently.
The code is aware of the shape of the problem — the comment at line 111
explains that rows can become visible out of `recorded_at` order — but ten
seconds is a guess at how far out of order. A monotonic database sequence
would make publication and cursor advancement independent of how long a
transaction took.
