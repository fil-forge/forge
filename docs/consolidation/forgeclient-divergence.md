# `ingot/forgeclient` + `ingot/tokenstore` vs `guppy/pkg/client` + `guppy/pkg/tokenstore` — divergence measurement

Plan question 8. Measured 2026-09-03 against:

- forge `f60dd59` (`/home/user/forge`, branch `claude/forge-monorepo-poc-p9w0yr`; the
  subject files are byte-identical to committed HEAD — `git diff --quiet HEAD -- ingot/forgeclient/*.go ingot/tokenstore/*.go` reports no difference; only `*/go.mod`, `*/go.sum` are dirty in that tree, which is the coordinator's in-flight Experiment A bump and was not touched).
- guppy main `e87812b` (`/home/user/guppy`, full clone, not shallow).
- guppy **carry-time** revision `11f8a6c` (see §1).

Reproduce: the script inlined in §2 (bash + git only). Raw diffs are in
`(session scratch, not committed) inv-forgeclient/{diff-carry,diff-main,diff-upstream}/*.diff`.

## 0. Answer in four lines

1. Of ~1,500 carried lines the **substantive (behaviour) divergence is four items, all in `blobadd.go`/`indexadd.go`**: (d1) per-call `WithProofStore` proof-chain override, (d2) explicit `Content-Length` on the blob PUT, (d3) dropped hard requirement that the accept receipt carry a `/pdp/accept` invocation, (d4) parameter-order swap + options on `BlobAdd`/`IndexAdd`. Everything else is deletion of unused surface (retrieval, spaces, progress/stall readers), the go-log/otel → injected `*zap.Logger` swap, import/rename, and comments.
2. **`ingot/tokenstore` has zero divergence**: `fs.go` and `mem.go` differ from guppy main by exactly one header comment line each (`git diff --numstat` `+1 -0`); `types.go` differs only by doc comments.
3. **Upstream fixes were received, not missed**: guppy's three post-carry commits to these packages (`3fb0878` attested signatures 06-05, `6bb73e3` drop allocate/accept delegations 06-19, `f2df335` dead code 06-19) were all hand-applied to the copy on 07-02 (`867ccd6`) and 07-21 (`a4f93ef`), 27–32 days late. The only upstream change never mirrored is the cosmetic `signer`→`issuer` field rename / `multikey.Issuer` type. guppy `pkg/client` has had **no commits since 2026-06-19**.
4. The flow is now **reversed**: d1–d3 above are fixes/features guppy does not have, and the LIVE ingot repo (`fil-forge/ingot` `59fcb5e`, 2026-08-29) has since added deferred-accept/`BlobConclude`/`BlobAbort`/`BlobRemove` to its copy (`blobadd.go` +138/−24 vs forge, two new files) with no guppy counterpart.

Planning-time numbers **all reproduced exactly** against guppy main (§2).

## 1. Dating the carry and picking the carry-time revision

`git -C /home/user/forge log --format='%h %ad %an %s' --date=iso --diff-filter=A -- ingot/forgeclient ingot/tokenstore`:

| forge commit | date | files added |
|---|---|---|
| `ebf08d6` | 2026-06-02 17:47 −0700 | `forgeclient/{blobadd,claimaccess,client,indexadd,options,pollclaim,requestaccess}.go`, `tokenstore/{fs,mem,types}.go` |
| `2077dbe` | 2026-06-03 12:04 −0700 | `forgeclient/{accessdelegate,accounts,accounts_test,provideradd,spaces}.go` |
| `a4f93ef` | 2026-07-21 (PR #35 "add Hilt client and wire with FX") | `forgeclient/proofstore_test.go` (ingot-only test; also modifies `blobadd.go`, `indexadd.go`) |

Other forge commits touching the copies (`git log -- ingot/forgeclient ingot/tokenstore`): `867ccd6` 2026-07-02 "feat: Attested Signatures + UCAN Principal Clarification", `18f05ac` 2026-07-30 "refactor: rewire modules to github.com/fil-forge/forge/*".

`git -C /home/user/guppy log -1 --before='2026-06-02T17:47:00-0700' --format='%h %ad %s' main` → **`11f8a6c` 2026-05-29 "feat!: UCAN 1.0"**. Same answer for the 06-03 carry. The next guppy commit touching `pkg/client` or `pkg/tokenstore` is `3fb0878` on 2026-06-05, so the carry-time content is unambiguous regardless of the exact hour of the carry.

`accounts_test.go` and `proofstore_test.go` have no guppy counterpart (`ls /home/user/guppy/pkg/client | grep -E 'accounts_test|proofstore'` → nothing); they are excluded from the file table.

## 2. LOC and `diff --stat` per carried file

Script (run from anywhere; writes into `$S`):

```bash
S=/tmp/claude-0/-home-user/cc0bd1c5-ddd8-5b18-827e-8db8c74e6de7/scratchpad/inv-forgeclient
CARRY=11f8a6c
cd /home/user/guppy
for f in accessdelegate accounts blobadd claimaccess client indexadd options pollclaim provideradd requestaccess spaces; do
  git show $CARRY:pkg/client/$f.go > $S/carry/client_$f.go; cp pkg/client/$f.go $S/main/client_$f.go; cp /home/user/forge/ingot/forgeclient/$f.go $S/forge/client_$f.go; done
for f in fs mem types; do
  git show $CARRY:pkg/tokenstore/$f.go > $S/carry/tokenstore_$f.go; cp pkg/tokenstore/$f.go $S/main/tokenstore_$f.go; cp /home/user/forge/ingot/tokenstore/$f.go $S/forge/tokenstore_$f.go; done
for f in $(ls $S/forge); do
  wc -l $S/carry/$f $S/main/$f $S/forge/$f
  git diff --no-index --numstat $S/carry/$f $S/forge/$f   # forge vs carry-time
  git diff --no-index --numstat $S/main/$f  $S/forge/$f   # forge vs main
  git diff --no-index --numstat $S/carry/$f $S/main/$f    # upstream drift carry..main
done
```

LOC = `wc -l`. Stat columns = `git diff --no-index --numstat` as `+added −deleted` (forge is the right-hand side for the first two columns; main is the right-hand side for the upstream column).

| file | guppy LOC @`11f8a6c` | guppy LOC @main `e87812b` | forge LOC | forge vs carry | forge vs main | upstream carry→main |
|---|---:|---:|---:|---|---|---|
| `client/accessdelegate.go` | 58 | 53 | 54 | +2 −6 | +4 −3 | +2 −7 |
| `client/accounts.go` | 28 | 28 | 32 | +5 −1 | +6 −2 | +1 −1 |
| `client/blobadd.go` | 588 | 486 | 360 | +84 −312 | +80 −206 | +9 −111 |
| `client/claimaccess.go` | 78 | 72 | 70 | +5 −13 | +9 −11 | +4 −10 |
| `client/client.go` | 204 | 197 | 109 | +32 −127 | +33 −121 | +7 −14 |
| `client/indexadd.go` | 80 | 69 | 83 | +22 −19 | +25 −11 | +4 −15 |
| `client/options.go` | 44 | 44 | 47 | +14 −11 | +14 −11 | identical |
| `client/pollclaim.go` | 99 | 99 | 96 | +11 −14 | +12 −15 | +1 −1 |
| `client/provideradd.go` | 51 | 46 | 49 | +5 −7 | +7 −4 | +2 −7 |
| `client/requestaccess.go` | 41 | 41 | 38 | +4 −7 | +6 −9 | +2 −2 |
| `client/spaces.go` | 222 | 222 | 13 | +6 −215 | +6 −215 | +1 −1 |
| **client subtotal** | **1493** | **1357** | **951** | | | |
| `tokenstore/fs.go` | 147 | 141 | 142 | +1 −6 | **+1 −0** | +0 −6 |
| `tokenstore/mem.go` | 116 | 98 | 99 | +1 −18 | **+1 −0** | +0 −18 |
| `tokenstore/types.go` | 26 | 26 | 35 | +9 −0 | +9 −0 | identical |
| **tokenstore subtotal** | **289** | **265** | **276** | | | |

### Planning-time numbers (plan §"Questions… 8") — reproduced?

| claim | measured (`wc -l`, guppy main → forge) | verdict |
|---|---|---|
| `spaces.go` 222→13 | 222 → 13 | reproduced |
| `client.go` 197→109 | 197 → 109 | reproduced |
| `blobadd.go` 486→360 | 486 → 360 | reproduced |
| `indexadd.go` 69→83 (grew) | 69 → 83 | reproduced |
| `pollclaim.go` 99→96 with 27 changed lines | 99 → 96; `--numstat` vs main = +12 −15 = 27 | reproduced (vs carry-time it is +11 −14 = 25) |

### Divergence timeline inside forge (`git show <c>:ingot/forgeclient/<f> | wc -l`)

| forge commit | `client.go` | `blobadd.go` | `indexadd.go` | `spaces.go` | `pollclaim.go` | `options.go` |
|---|---:|---:|---:|---:|---:|---:|
| `ebf08d6` 06-02 (carry) | 113 | 401 | 83 | – | 96 | 47 |
| `2077dbe` 06-03 | 114 | 401 | 83 | 13 | 96 | 47 |
| `867ccd6` 07-02 | 109 | 390 | 73 | 13 | 96 | 47 |
| `a4f93ef` 07-21 | 109 | 360 | 83 | 13 | 96 | 47 |
| `f60dd59` (HEAD) | 109 | 360 | 83 | 13 | 96 | 47 |

So `blobadd.go` was already trimmed 588→401 at the moment of carry (otel, go-log, progress/stall readers, PDP requirement removed on day one), then 401→390 (attestations removed, mirroring upstream), then 390→360 (`delegateWithProofs` removed mirroring upstream, `WithProofStore` added).

## 3. Hunk-by-hunk classification

Categories: **(a)** deletion of functionality ingot does not use; **(b)** logger/otel swap; **(c)** import-path/rename/comment/formatting only; **(d)** SUBSTANTIVE behaviour change (hunk quoted); **(e)** a change that mirrors an upstream guppy commit made after the carry (i.e. the copy *received* an upstream change — listed separately because it is neither drift nor local invention).

Baseline for the classification is forge vs **carry-time `11f8a6c`** (`diff-carry/`), which isolates edits made in the copy. Where forge-vs-main (`diff-main/`) differs, it is noted.

### 3.1 `blobadd.go` (forge vs carry +84 −312; vs main +80 −206)

| # | hunk (carry-time → forge) | class |
|---|---|---|
| 1 | header comment block "Carried/trimmed… Ingot divergences from upstream…", `package client` → `package forgeclient` | (c) |
| 2 | imports: drop `guppy/internal/ctxutil`, `otelhttp`, `otel/attribute`, `otel/trace`; add `go.uber.org/zap`, keep `ucanlib` | (b)/(c) |
| 3 | `ucantone/principal/ed25519` → `ucantone/multikey` + `ucantone/multikey/ed25519`; `receipt.IssueOK(signer, …)` → `issuer := multikey.KeyIssuer(signer); receipt.IssueOK(issuer, …)` | (e) — identical change landed upstream in `3fb0878`; present in guppy main |
| 4 | `BlobAddConfig`: `ProgressFn func(uploaded int64)` removed | (a) |
| 5 | `BlobAddConfig`: `ProofStore ucanlib.ProofStore` added; `WithProofStore(ps)` option added | **(d1)** — see quote below |
| 6 | `NewBlobAddConfig`: default `PutClient` `&http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}` → `&http.Client{}` | (b) (default PUT loses otel span propagation; no functional change) |
| 7 | `WithPutProgress` removed | (a) |
| 8 | `AddedBlob.PDPAccept ucan.Invocation` field removed | **(d3)** |
| 9 | `func (c *Client) BlobAdd(ctx, content io.Reader, space did.DID, options…)` → `BlobAdd(ctx, space did.DID, content io.Reader, options…)`; long doc comment → short | **(d4)** signature (API-incompatible, behaviour-neutral) |
| 10 | `tracer.Start(ctx, "blob-add", …)`/`defer span.End()` removed; `log.Infow("blob adding"…)` removed; deferred `log.Errorw/Infow` → `c.logger.Error(...)` / `c.logger.Debug(...)` (success demoted Info→Debug, `has_digest` field dropped) | (b) |
| 11 | `needsHash` block: `read-content`/`hash-content` spans removed; `contentBytes, err :=` → `contentBytes, rerr :=` (avoids shadowing the named return `err`) | (b)/(c) |
| 12 | `if cfg.ProgressFn != nil { contentReader = &progressReader{…} }` removed | (a) |
| 13 | `proofs, proofLinks, err := c.ProofChain(ctx, c.signer.DID(), …)` → `proofStore := ucanlib.ProofStore(c.tokenStore); if cfg.ProofStore != nil { proofStore = cfg.ProofStore }; proofs, proofLinks, err := proofStore.ProofChain(…)` | **(d1)** |
| 14 | `attestations, err := c.ProofAttestations(…)` + `execution.WithInvocations(attestations...)` removed | (e) — upstream `3fb0878` |
| 15 | `dlgPolicy := policy.Build(...)`, `allocDlg… := delegateWithProofs(... Allocate ...)`, `accDlg… := delegateWithProofs(... Accept ...)` and the six `WithDelegations/WithInvocations(alloc*/acc*)` options removed | (e) — upstream `6bb73e3` "fix: do not send allocate or accept delegations (#17)" (1 +/100 − in guppy); forge received it in `a4f93ef` |
| 16 | `blobcmds.Add.Invoke(` / `Execute[...](` argument lists collapsed onto one line | (c) |
| 17 | `log.Errorw("failed to unmarshal allocation execution failure", …)` and `…accept execution failure…` removed (error return unchanged) | (b) |
| 18 | `putBlob(ctx, putClient, url, headers, contentReader)` → `putBlob(…, contentReader, int64(*contentSizePtr))`; `putBlob` gains `size int64` and sets `req.ContentLength = size` | **(d2)** — see quote below; NOT in guppy main (`grep -n ContentLength /home/user/guppy/pkg/client/blobadd.go` → none) |
| 19 | `!putSuccess` branch: "Re-delegate, since the previous delegation may have expired" `delegateWithProofs(... Accept ...)` + `sendPutReceipt(ctx, putInv, WithDelegations(accDlg), …)` → `sendPutReceipt(ctx, putInv)` | (e) — upstream `6bb73e3`; guppy main has the same one-liner |
| 20 | accept-receipt metadata scan: `switch inv.Command() { case assertcmds.Location.Command: … default: if inv.Command().String() == "/pdp/accept" { pdpAcceptInv = inv } }` → `if inv.Command() == assertcmds.Location.Command {…}`; `if pdpAcceptInv == nil { return …, fmt.Errorf("blob accept receipt missing PDP accept invocation") }` removed; return drops `PDPAccept:` | **(d3)** — see quote below; guppy main still has the check |
| 21 | `putBlob`: start/`log.Infow("putting blob"…)`/deferred duration log removed; `newStallWarnReader(body, url, 30*time.Second)` wrapper removed (request body is `body` directly); `ctxutil.EnrichWithCause(err, ctx)` → `err` (×2); `if err := resp.Body.Close(); err != nil { log.Warnf(...) }` → `_ = resp.Body.Close()` | (a) stall reader; (b) logs; (c) `EnrichWithCause` — only changes error *text* when a context with a cause was cancelled (`"%w, cause: %w"` wrapper dropped); `errors.Is(err, context.Canceled)` behaviour unchanged |
| 22 | `sendPutReceipt`: local `did` → `id` (shadowed the `did` package) | (c) |
| 23 | `sendPutReceipt`: `ctxutil.EnrichWithCause(err, ctx)` → `err` | (c) as in #21 |
| 24 | `type progressReader`, `type stallWarnReader` + methods removed | (a) |
| 25 | `func delegateWithProofs(...)` removed | (e) — upstream `6bb73e3` |

**(d1) quoted** (`diff-carry/client_blobadd.go.diff`, also present vs main):

```diff
+	// ProofStore, when set, supplies the delegation proof chain for this
+	// call instead of the client's default token store — used to scope an
+	// invocation to a request's per-access-key proofs.
+	ProofStore ucanlib.ProofStore
 …
+// WithProofStore overrides the proof store used to build this call's
+// delegation chains (default: the client's token store).
+func WithProofStore(ps ucanlib.ProofStore) BlobAddOption {
+	return func(cfg *BlobAddConfig) { cfg.ProofStore = ps }
+}
 …
+	proofStore := ucanlib.ProofStore(c.tokenStore)
+	if cfg.ProofStore != nil {
+		proofStore = cfg.ProofStore
+	}
-	proofs, proofLinks, err := c.ProofChain(ctx, c.signer.DID(), blobcmds.Add.Command, space)
+	proofs, proofLinks, err := proofStore.ProofChain(ctx, c.signer.DID(), blobcmds.Add.Command, space)
```

Introduced in forge `a4f93ef` (2026-07-21). Used by `ingot/uploader/blob.go:71`, `ingot/uploader/forge.go:158,211,218`, tested by `ingot/forgeclient/proofstore_test.go`. Guppy has a single token store per client and no per-call override.

**(d2) quoted** (present since the initial carry `ebf08d6`; `grep -c ContentLength` = 1 at every forge revision):

```diff
+	// PUT the bytes when allocate handed us an address and we don't yet
+	// have a put receipt. Content-Length is set explicitly so net/http
+	// doesn't fall back to chunked transfer encoding for a file body —
+	// piri's PUT endpoint requires it.
 	if allocOK.Address != nil && !putSuccess {
-		if err := putBlob(ctx, putClient, allocOK.Address.URL.URL(), allocOK.Address.Headers, contentReader); err != nil {
+		if err := putBlob(ctx, putClient, allocOK.Address.URL.URL(), allocOK.Address.Headers, contentReader, int64(*contentSizePtr)); err != nil {
 …
-func putBlob(ctx context.Context, client *http.Client, url *url.URL, headers map[string]string, body io.Reader) error {
+func putBlob(ctx context.Context, client *http.Client, url *url.URL, headers map[string]string, body io.Reader, size int64) error {
+	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url.String(), body)
 …
+	if size >= 0 {
+		req.ContentLength = size
+	}
```

Behavioural: with `WithPrecomputedDigest` guppy streams the caller's reader (not a `*bytes.Reader`), so `net/http` sends `Transfer-Encoding: chunked`; ingot's copy always sends `Content-Length`. Guppy still lacks this (and, in guppy, the stall-warn wrapper always hides the body type, so guppy *always* chunks). Whether piri actually rejects chunked PUTs was not verified here — the claim is the comment's. Speculation: this is a latent guppy bug against piri worth a separate check.

**(d3) quoted** (present since the initial carry):

```diff
 	var locationCommitment ucan.Invocation
-	var pdpAcceptInv ucan.Invocation
 	for _, inv := range accMeta.Invocations() {
-		switch inv.Command() {
-		case assertcmds.Location.Command:
+		if inv.Command() == assertcmds.Location.Command {
 			locationCommitment = inv
-		// TODO: use PDP commands when landed https://github.com/fil-forge/libforge/pull/28
-		default:
-			if inv.Command().String() == "/pdp/accept" {
-				pdpAcceptInv = inv
-			}
 		}
 	}
 	if locationCommitment == nil {
 		return AddedBlob{}, fmt.Errorf("blob accept receipt missing location commitment invocation")
 	}
-	if pdpAcceptInv == nil {
-		return AddedBlob{}, fmt.Errorf("blob accept receipt missing PDP accept invocation")
-	}
```

Behavioural: guppy main fails a `BlobAdd` whose accept receipt lacks a `/pdp/accept` invocation and exposes it as `AddedBlob.PDPAccept`; ingot accepts the blob without it. Guppy still carries the `TODO … libforge/pull/28` string-comparison.

**(d4) quoted** (forge `a4f93ef`):

```diff
-func (c *Client) BlobAdd(ctx context.Context, content io.Reader, space did.DID, options ...BlobAddOption) (blob AddedBlob, err error) {
+func (c *Client) BlobAdd(ctx context.Context, space did.DID, content io.Reader, options ...BlobAddOption) (blob AddedBlob, err error) {
```

API-incompatible reorder; behaviour-neutral. Both types differ so a mis-port is a compile error, not a silent one.

### 3.2 `indexadd.go` (forge vs carry +22 −19; vs main +25 −11)

| # | hunk | class |
|---|---|---|
| 1 | header + package rename; `ucanlib` import added | (c) |
| 2 | `IndexAdd(ctx, indexCID cid.Cid, space did.DID) error` → `IndexAdd(ctx, space did.DID, indexCID cid.Cid, options ...BlobAddOption) error` + 7-line doc comment | **(d4)** signature; reuses `BlobAddOption` |
| 3 | `cfg := NewBlobAddConfig(options...); proofStore := ucanlib.ProofStore(c.tokenStore); if cfg.ProofStore != nil { proofStore = cfg.ProofStore }`; both `c.ProofChain(…)` → `proofStore.ProofChain(…)` | **(d1)** |
| 4 | `retrievalAttestations`/`attestations` `ProofAttestations` calls and the two `execution.WithInvocations(...)` options removed | (e) — upstream `3fb0878` |
| 5 | 5-line comment "The leaf delegation (guppy → upload)…" → 3-line "(agent → sprue)…" | (c) |

Quoted (d) hunk vs main (`diff-main/client_indexadd.go.diff`):

```diff
-func (c *Client) IndexAdd(ctx context.Context, indexCID cid.Cid, space did.DID) error {
+func (c *Client) IndexAdd(ctx context.Context, space did.DID, indexCID cid.Cid, options ...BlobAddOption) error {
+	cfg := NewBlobAddConfig(options...)
+	proofStore := ucanlib.ProofStore(c.tokenStore)
+	if cfg.ProofStore != nil {
+		proofStore = cfg.ProofStore
+	}
 …
-	retrievalProofs, retrievalProofLinks, err := c.ProofChain(ctx, c.issuer.DID(), contentcmds.Retrieve.Command, space)
+	retrievalProofs, retrievalProofLinks, err := proofStore.ProofChain(ctx, c.signer.DID(), contentcmds.Retrieve.Command, space)
 …
-	proofs, proofLinks, err := c.ProofChain(ctx, c.issuer.DID(), indexcmds.Add.Command, space)
+	proofs, proofLinks, err := proofStore.ProofChain(ctx, c.signer.DID(), indexcmds.Add.Command, space)
```

This is why `indexadd.go` *grew* (69→83): +14 lines of option plumbing and doc, −0 functionality.

### 3.3 `client.go` (forge vs carry +32 −127; vs main +33 −121)

| # | hunk | class |
|---|---|---|
| 1 | 11-line package doc ("carried, trimmed copy… guppy embeds ingot → import cycle… see DESIGN_NOTES.md §B") | (c) (note: F14 — there is no §B and no cycle) |
| 2 | imports: drop `bytes`, `reflect`, `guppy/pkg/tokenstore`, `libforge/ucan/retrieval`, `edm`, `ipld/datamodel`, `go-log/v2`, `otel`, `zapcore`; add `forge/ingot/internal/ucanexec`, `forge/ingot/tokenstore` | (b)/(c) |
| 3 | `var ( log = logging.Logger(...); tracer = otel.Tracer(...) )` removed | (b) |
| 4 | `Client.signer ucan.Signer` → `signer ucan.Issuer`; `New(signer ucan.Signer, …)` → `New(signer ucan.Issuer, …)`; `Issuer() ucan.Signer` → `Issuer() ucan.Issuer` | (c)/(e) — tracks the ucantone API change that moved `DID()` off `Signer` (`ucan.Issuer = Principal + Signer + String()` at both pins checked, `ccb77059` and `3a20cd59`). Upstream `3fb0878` did the same but chose `multikey.Issuer` and renamed the field to `issuer`. Forge did it in `867ccd6`. |
| 5 | `Client.retrievalOpts []retrieval.ClientOption` removed | (a) |
| 6 | `Client.logger *zap.Logger` added; `New` sets `logger: zap.NewNop()` | (b) |
| 7 | comment "Create a default memory store if none provided" removed; `DID()`, `Issuer()`, `ServiceID()` collapsed to one-liners; `ProofChain` doc shortened | (c) |
| 8 | `ProofAttestations` method removed | (e) — upstream `3fb0878` |
| 9 | `Execute[T]` body (structured zap logging of invocation + receipt, error decode, reflect-allocate, unmarshal) → `return ucanexec.Execute[T](ctx, executor, inv, options...)` | (b), with one observable text delta: on an undecodable failure payload guppy returns `fmt.Errorf("executing invocation")`, `ucanexec` returns `fmt.Errorf("executing invocation: undecodable failure")`; `ucanexec` also nil-guards `reflect.TypeOf(ok)` (guppy would panic if `T` were a non-pointer interface — not reachable with current instantiations). Control flow, error wrapping of `model`, and return values are otherwise identical (`/home/user/forge/ingot/internal/ucanexec/execute.go`, 61 lines). |
| 10 | `type RawMap []byte` + `MarshalLogObject` removed | (b) (logging helper only) |

### 3.4 `spaces.go` (222 → 13; vs carry and vs main +6 −215)

| # | hunk | class |
|---|---|---|
| 1 | header + package rename; imports reduced to `ipld/datamodel` | (c) |
| 2 | `attestCommand`, `SpaceNotFoundError`, `MultipleSpacesFoundError`, `type Space` (+ `DID`, `AccessProofs`, `Names`), `(*Client).Spaces`, `delegationsForAudience`, `SpacesNamed`, `SpaceNamed` removed | (a) |
| 3 | `SpaceNameMetaKey` const and `SpaceNameMetadata(name)` kept, comments shortened | (c) — **but** `grep -rn 'SpaceNameMetadata\|SpaceNameMetaKey' /home/user/forge --include=*.go` finds no caller outside `spaces.go` itself: the retained 13 lines are dead in forge. |

Upstream drift on this file since carry is one line (`c.signer`→`c.issuer`), so guppy's Spaces API is unchanged and could be re-adopted wholesale.

### 3.5 `options.go` (vs carry and vs main +14 −11; upstream unchanged since carry)

| # | hunk | class |
|---|---|---|
| 1 | header; `guppy/pkg/tokenstore` → `forge/ingot/tokenstore`; drop `libforge/ucan/retrieval`; add `zap` | (c) |
| 2 | doc comments reworded on `Option`, `WithUCANClientOptions` (guppy's comment was a stale copy-paste "WithHTTPClient configures…"), `WithReceiptsClient`, `WithTokenStore` | (c) |
| 3 | `WithRetrievalOptions(retrievalOpts ...retrieval.ClientOption)` removed | (a) |
| 4 | `WithLogger(l *zap.Logger)` added (nil-safe) | (b) |

### 3.6 `pollclaim.go` (vs carry +11 −14; vs main +12 −15 = 27)

| # | hunk | class |
|---|---|---|
| 1 | header ("guppy/internal/ctxutil.Cause replaced with stdlib context.Cause"); drop `ctxutil` import | (c) |
| 2 | `ClaimResult` doc added; `PollClaim`/`PollClaimWithTick` docs shortened | (c) |
| 3 | `ctxutil.Cause(ctx)` → `context.Cause(ctx)` in the `ctx.Done()` branch | (c) — guppy's `ctxutil.Cause` (read at `11f8a6c:internal/ctxutil/ctxutil.go`) returns `ctx.Err()` when there is no distinct cause, else `fmt.Errorf("%w, cause: %w", ctx.Err(), cause)`; stdlib `context.Cause` returns the cause alone (which equals `ctx.Err()` when none was given). Same `errors.Is` outcomes for the cause; the wrapped `context.Canceled` is lost from the chain only when a distinct cause was supplied. Error-text only in practice. |
| 4 | three inline `// Skip if …` comments and one block comment removed | (c) |
| vs main extra | `c.signer.DID()` vs `c.issuer.DID()` | (c) |

No (a), (b) or (d) hunks. 27 "changed lines" = 12 added, 15 removed, all comment/import/rename.

### 3.7 `accessdelegate.go`, `claimaccess.go`, `provideradd.go`, `requestaccess.go`, `accounts.go`

All four follow the same pattern:

| hunk | class | files |
|---|---|---|
| header + `package forgeclient` | (c) | all |
| doc comment rewritten/shortened (`ProviderAdd` gains 3 lines explaining the proof chain is keyed on the account; `Accounts` gains a 3-line doc) | (c) | all |
| `attestations, err := c.ProofAttestations(ctx, proofs, c.serviceID)` + `execution.WithInvocations(attestations...)` removed | (e) — upstream `3fb0878` | `accessdelegate`, `claimaccess`, `provideradd` |
| vs main only: `c.signer`/`c.signer.DID()` where main has `c.issuer` | (c) | all |

No (a), (b) or (d) hunks in these five files.

### 3.8 `tokenstore/{fs,mem,types}.go`

| file | forge vs carry | forge vs main |
|---|---|---|
| `fs.go` | header line; `(*FsStore).ProofAttestations` removed — (e) upstream `3fb0878` | **`+1 −0`: the header comment only** |
| `mem.go` | header line; `(*MemStore).ProofAttestations` removed — (e) `3fb0878`; `listInvocations` removed — (e) `f2df335` "refactor: Remove dead function" | **`+1 −0`: the header comment only** |
| `types.go` | 7-line package doc + 2-line `Store` doc | `+9 −0`, comments only; upstream identical since carry |

`ingot/tokenstore` is functionally byte-identical to `guppy/pkg/tokenstore` at main. Note `guppy/pkg/tokenstore/store_test.go` (270 lines) was **not** carried; ingot's tokenstore has no tests of its own (`ls /home/user/forge/ingot/tokenstore/` → three `.go` files, no `_test.go`).

### 3.9 Classification totals

Counting hunks as rows in the tables above (a hunk that mixes classes counted once under its most significant class):

| class | hunks | files |
|---|---:|---|
| (a) deletion of unused | 8 | `blobadd` ×5 (progress fn/option/reader, stall reader, wrapping), `spaces` ×1, `options` ×1, `client` ×1 |
| (b) logger/otel swap | 9 | `blobadd` ×5, `client` ×4 (incl. `Execute` delegation), `options` ×1 |
| (c) import/rename/comment | ~30 | every file |
| (e) mirrors a post-carry upstream commit | 12 | `blobadd` ×5, `indexadd` ×1, `client` ×1, `accessdelegate`/`claimaccess`/`provideradd` ×3, `tokenstore/fs` ×1, `tokenstore/mem` ×2 (counting `listInvocations`) |
| **(d) substantive** | **4 distinct changes, 6 hunks** | `blobadd` (d1 ×2 hunks, d2, d3, d4), `indexadd` (d1+d4 in one hunk) |

## 4. Upstream (guppy) changes since the carry, and whether ingot received them

`git -C /home/user/guppy log --since=2026-05-20 --name-status main -- pkg/client pkg/tokenstore` lists exactly four commits: the carry base `11f8a6c` and three later ones. `git log -1 main -- pkg/client pkg/tokenstore` → `f2df335 2026-06-19`: **guppy's client and tokenstore have not changed in the 76 days between 2026-06-19 and guppy main `e87812b` (2026-08-28).**

| guppy commit | date | change to carried files (`--numstat`, tests excluded) | received by forge copy? |
|---|---|---|---|
| `3fb0878` "feat: Support attested signatures" | 2026-06-05 | removes `ProofAttestations` everywhere (`client.go` 7/14, `blobadd.go` 8/11, `indexadd.go` 4/15, `accessdelegate` 2/7, `claimaccess` 4/10, `provideradd` 2/7, `tokenstore/fs` 0/6, `tokenstore/mem` 0/4); renames `signer`→`issuer` and types it `multikey.Issuer`; `principal/ed25519`→`multikey/ed25519` + `KeyIssuer` | **Partially, 27 days later** in `867ccd6` (2026-07-02; `--numstat`: `accessdelegate` 0/5, `claimaccess` 0/6, `provideradd` 0/5, `indexadd` 0/10, `blobadd` 15/26, `client` 4/9, `tokenstore/fs` 0/6, `tokenstore/mem` 0/18). Received: all `ProofAttestations` removals, `multikey.KeyIssuer`. **Not received**: the `signer`→`issuer` rename and `multikey.Issuer` type (forge uses `ucan.Issuer`, keeps the name `signer`). Cosmetic. |
| `6bb73e3` "fix: do not send allocate or accept delegations (#17)" | 2026-06-19 | `blobadd.go` 1/100: deletes `delegateWithProofs`, `dlgPolicy`, alloc/accept delegations and the accept re-delegation before `sendPutReceipt` | **Yes, 32 days later** in `a4f93ef` (2026-07-21; `blobadd.go` 28/58). `grep -c delegateWithProofs` on forge's `blobadd.go`: 4 at `ebf08d6`, `2077dbe`, `867ccd6`; 0 from `a4f93ef`. Forge's header note "No /blob/accept re-delegation: sprue owns accept" documents it as an ingot divergence, but it is now upstream behaviour too. |
| `f2df335` "refactor: Remove dead function" | 2026-06-19 | `tokenstore/mem.go` 0/14 (`listInvocations`) | **Yes** in `867ccd6` (`mem.go` 0/18 = 4 + 14). |

**Outstanding upstream changes ingot never received: only the `issuer` rename/`multikey.Issuer` type.** No behavioural fix is pending in that direction.

## 5. Reverse direction: what the copy has that guppy does not

| item | where | in guppy main? |
|---|---|---|
| (d1) per-call `WithProofStore` on `BlobAdd`/`IndexAdd` | forge `a4f93ef` | no |
| (d2) `req.ContentLength` on the blob PUT | since carry `ebf08d6` | no (`grep ContentLength` → none; guppy always wraps the body in `stallWarnReader`, so it always chunks) |
| (d3) no hard `/pdp/accept` requirement on the accept receipt | since carry | no — guppy still errors `"blob accept receipt missing PDP accept invocation"` and carries a `TODO … libforge/pull/28` string match |
| `ucanexec.Execute` nil-guard on `reflect.TypeOf` | forge | no |
| `WithLogger` injection instead of a package-level `go-log` | forge | no (guppy is a CLI; package logger is idiomatic there) |
| LIVE ingot only (not in forge `f60dd59`): `WithConclude(false)` deferred accept, `BlobConclude`, `AddedBlob.{AddTask,AcceptTask,PutInvocation}`, `blobabort.go` (58 lines), `blobremove.go` (55 lines) | `fil-forge/ingot` `a0250c3` 2026-08-05 "feat: deferred-accept multipart (FIL-520) + network blob removal on DeleteObject (FIL-588) (#40)"; live `blobadd.go` vs forge: `+138 −24` | no (`ls /home/user/guppy/pkg/client | grep -iE 'remove|abort'` → none) |

Live ingot (`59fcb5e`, 2026-08-29) otherwise matches forge's copy: `accessdelegate`, `accounts`, `claimaccess`, `indexadd`, `pollclaim`, `provideradd`, `requestaccess`, `spaces`, and all three `tokenstore` files are byte-identical; `client.go`/`options.go` differ only by the module path (`github.com/fil-forge/ingot/…` vs `github.com/fil-forge/forge/ingot/…`).

Note on `AddedBlob.PutInvocation` in live ingot: its own comment says the bytes "embed the derived signer keys needed to synthesize the put receipt … treat it as sensitive". Not examined further here; flagged because it is key material travelling in a returned struct.

## 6. Reading for the in/out decision

- The "keep in sync with upstream" instruction in the file headers was honoured for about seven weeks, by hand, with a 4–5 week lag, and then the direction of flow reversed: guppy's client froze on 2026-06-19 and ingot's copy became the more-developed one (d1–d3 in forge, then deferred-accept/abort/remove in live ingot). A shared library would today be extracted *from ingot*, not from guppy.
- The substantive delta between the two copies is small and enumerable (four items, all in two files). d1 is an additive option guppy could take as-is; d2 is arguably a bug fix guppy needs; d3 is a policy question (does the network still promise a `/pdp/accept` in the accept receipt? guppy's TODO suggests the check was a placeholder); d4 is a signature reorder that any unification would settle once.
- `tokenstore` is already identical and is the obvious first extraction candidate; it also has zero tests on the ingot side while guppy has a 270-line `store_test.go`.
- The plan's implicit hypothesis ("all of it is deletion plus the logger swap") is **mostly but not entirely right**: by line count ≥ 95% of the diff is (a)/(b)/(c)/(e); by behaviour there are four real divergences, none of which is an accident of the copy — each is a deliberate ingot-side change documented in the file's header.

## 7. Numbers a reviewer should double-check

- Carry-time revision `11f8a6c` chosen by `git log -1 --before='2026-06-02T17:47:00-0700' main`; alternative dates up to 2026-06-04 give the same file contents because the next `pkg/client` commit is `3fb0878` (2026-06-05).
- All LOC via `wc -l` (counts newline characters; a file without a trailing newline would be one short — none observed).
- `--numstat` figures are `git diff --no-index` with default heuristics; `diff -u` may split/merge hunks differently but adds/deletes are stable.
- Hunk classification is a judgement; the (d) list is the part worth contesting. I classified `ctxutil.Cause`→`context.Cause` and `EnrichWithCause` removal as (c) because they change error text, not control flow or `errors.Is` outcomes for the cause; someone who treats error text as behaviour would move them to (d).
- The `Content-Length` rationale ("piri's PUT endpoint requires it") is the code comment's claim, not verified against piri here.
