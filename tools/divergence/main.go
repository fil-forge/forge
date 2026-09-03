// Command divergence measures, from git history alone, how far the consumers of
// libforge and ucantone (the forge monorepo modules, the live per-service repos,
// guppy and indexing-service) drifted from the libraries' main branches over a
// window of time, and what kind of library changes they had to absorb.
//
// It shells out to git and reads every repository read-only. Standard library
// only. See README.md next to this file for the full description of the
// measurements and the exact reproduce command.
package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	libforgeMod = "github.com/fil-forge/libforge"
	ucantoneMod = "github.com/fil-forge/ucantone"
)

// libs is the analysis order of the two libraries; libModules maps each to
// its module path.
var (
	libs       = []string{"libforge", "ucantone"}
	libModules = map[string]string{"libforge": libforgeMod, "ucantone": ucantoneMod}
)

// ---------------------------------------------------------------------------
// Options
// ---------------------------------------------------------------------------

type options struct {
	forge, forgeRef       string
	libforge, ucantone    string
	libRef                string
	guppy, indexing       string
	consumerRef           string
	liveDir               string
	liveRepos             []string
	liveRef               string
	from, to, contextFrom time.Time
	today                 time.Time
	out                   string
	classification        string
	verbose               bool
}

func parseDay(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, time.UTC)
}

// toolDir is the directory holding this source file, recorded at build time,
// so defaults resolve relative to the checkout the tool lives in.
func toolDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Dir(file)
}

func parseOptions() (*options, error) {
	o := &options{}
	defForge := filepath.Clean(filepath.Join(toolDir(), "..", ".."))
	var from, to, ctx, today, live string
	flag.StringVar(&o.forge, "forge", defForge, "path of the forge monorepo checkout (consumers: its Go modules)")
	flag.StringVar(&o.forgeRef, "forge-ref", "HEAD", "ref of forge to read (go.mod histories follow this ref)")
	flag.StringVar(&o.libforge, "libforge", "/home/user/libforge", "path of a libforge clone")
	flag.StringVar(&o.ucantone, "ucantone", "/home/user/ucantone", "path of a ucantone clone")
	flag.StringVar(&o.libRef, "lib-ref", "origin/main", "the libraries' main ref (falls back to main, then HEAD)")
	flag.StringVar(&o.guppy, "guppy", "/home/user/guppy", "path of a guppy clone ('' to skip)")
	flag.StringVar(&o.indexing, "indexing-service", "/home/user/indexing-service", "path of an indexing-service clone ('' to skip)")
	flag.StringVar(&o.consumerRef, "consumer-ref", "origin/main", "ref of guppy/indexing-service to read (falls back to main, then HEAD)")
	flag.StringVar(&o.liveDir, "live-dir", "/home/user/fil-forge", "directory holding the live per-service clones")
	flag.StringVar(&live, "live-repos", "piri,hilt,sprue,ingot,smelt,delegator,piri-signing-service", "comma-separated live repo names under -live-dir")
	flag.StringVar(&o.liveRef, "live-ref", "HEAD", "ref of the live repos to read")
	flag.StringVar(&from, "from", "2026-08-01", "first day of the measurement window (UTC)")
	flag.StringVar(&to, "to", "2026-08-31", "last day of the measurement window (UTC, inclusive)")
	flag.StringVar(&ctx, "context-from", "", "first day of the context period shown alongside (default: one month before -from)")
	flag.StringVar(&today, "today", "", "the 'today' used for current-lag figures (UTC date; default: now)")
	flag.StringVar(&o.out, "out", "docs/consolidation/divergence-august-2026", "output base name (relative to -forge); writes <out>.md and <out>.json")
	flag.StringVar(&o.classification, "classification", filepath.Join(toolDir(), "classification.json"), "curated per-commit classification file")
	flag.BoolVar(&o.verbose, "v", false, "progress on stderr")
	flag.Parse()

	var err error
	if o.from, err = parseDay(from); err != nil {
		return nil, fmt.Errorf("-from: %w", err)
	}
	if o.to, err = parseDay(to); err != nil {
		return nil, fmt.Errorf("-to: %w", err)
	}
	if ctx == "" {
		o.contextFrom = o.from.AddDate(0, -1, 0)
	} else if o.contextFrom, err = parseDay(ctx); err != nil {
		return nil, fmt.Errorf("-context-from: %w", err)
	}
	if today == "" {
		o.today = time.Now().UTC().Truncate(24 * time.Hour)
	} else if o.today, err = parseDay(today); err != nil {
		return nil, fmt.Errorf("-today: %w", err)
	}
	for _, r := range strings.Split(live, ",") {
		if r = strings.TrimSpace(r); r != "" {
			o.liveRepos = append(o.liveRepos, r)
		}
	}
	if !filepath.IsAbs(o.out) {
		o.out = filepath.Join(o.forge, o.out)
	}
	return o, nil
}

// ---------------------------------------------------------------------------
// git plumbing
// ---------------------------------------------------------------------------

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git -C %s %s: %v: %s", dir, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out.String(), nil
}

func gitOK(dir string, args ...string) bool {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	return cmd.Run() == nil
}

func resolveRef(dir string, candidates ...string) (string, error) {
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if gitOK(dir, "rev-parse", "--verify", "--quiet", c+"^{commit}") {
			return c, nil
		}
	}
	return "", fmt.Errorf("none of %v resolves in %s", candidates, dir)
}

var (
	reSquashPR = regexp.MustCompile(`\(#(\d+)\)\s*$`)
	reMergePR  = regexp.MustCompile(`^Merge pull request #(\d+)`)
)

func parseCommitRecord(rec string) (Commit, string) {
	head, diff, _ := strings.Cut(rec, "\n")
	f := strings.Split(head, "\x1f")
	for len(f) < 5 {
		f = append(f, "")
	}
	secs, _ := strconv.ParseInt(f[1], 10, 64)
	c := Commit{
		SHA:     f[0],
		Short:   short(f[0]),
		Time:    time.Unix(secs, 0).UTC(),
		Author:  f[2],
		Subject: f[4],
	}
	if f[3] != "" {
		c.Parents = strings.Fields(f[3])
	}
	c.IsMerge = len(c.Parents) > 1
	c.Dependabot = strings.Contains(strings.ToLower(c.Author), "dependabot")
	if m := reSquashPR.FindStringSubmatch(c.Subject); m != nil {
		c.PR, _ = strconv.Atoi(m[1])
	} else if m := reMergePR.FindStringSubmatch(c.Subject); m != nil {
		c.PR, _ = strconv.Atoi(m[1])
	}
	return c, diff
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

const logFormat = "--format=%x1e%H%x1f%ct%x1f%an%x1f%P%x1f%s"

// gitLog returns commits newest first.
func gitLog(dir, ref string, extra ...string) ([]Commit, error) {
	args := append([]string{"log", logFormat}, extra...)
	args = append(args, ref)
	out, err := git(dir, args...)
	if err != nil {
		return nil, err
	}
	var cs []Commit
	for _, rec := range strings.Split(out, "\x1e") {
		rec = strings.TrimLeft(rec, "\n")
		if strings.TrimSpace(rec) == "" {
			continue
		}
		c, _ := parseCommitRecord(rec)
		cs = append(cs, c)
	}
	return cs, nil
}

func headCommit(dir, ref string) (Commit, error) {
	cs, err := gitLog(dir, ref, "-1")
	if err != nil {
		return Commit{}, err
	}
	if len(cs) == 0 {
		return Commit{}, fmt.Errorf("no commit at %s in %s", ref, dir)
	}
	return cs[0], nil
}

func repoInfo(name, dir, ref string) (RepoInfo, error) {
	ri := RepoInfo{Name: name, Path: dir, Ref: ref}
	c, err := headCommit(dir, ref)
	if err != nil {
		return ri, err
	}
	ri.Head = c.SHA
	ri.HeadShort = c.Short
	ri.HeadTime = c.Time
	out, _ := git(dir, "rev-parse", "--is-shallow-repository")
	ri.Shallow = strings.TrimSpace(out) == "true"
	if out, err := git(dir, "log", "--reverse", "--format=%ct", "--max-parents=0", ref); err == nil {
		// oldest root reachable from ref; in a shallow clone this is the cut.
		var oldest int64
		for _, l := range strings.Fields(out) {
			if v, err := strconv.ParseInt(l, 10, 64); err == nil && (oldest == 0 || v < oldest) {
				oldest = v
			}
		}
		if oldest > 0 {
			ri.Oldest = time.Unix(oldest, 0).UTC()
		}
	}
	if out, err := git(dir, "remote", "get-url", "origin"); err == nil {
		ri.Remote = strings.TrimSpace(out)
	}
	return ri, nil
}

// ---------------------------------------------------------------------------
// Data model
// ---------------------------------------------------------------------------

type Commit struct {
	SHA        string    `json:"sha"`
	Short      string    `json:"short"`
	Time       time.Time `json:"time"`
	Author     string    `json:"author"`
	Parents    []string  `json:"parents,omitempty"`
	Subject    string    `json:"subject"`
	IsMerge    bool      `json:"is_merge"`
	PR         int       `json:"pr,omitempty"`
	Dependabot bool      `json:"dependabot"`
}

type RepoInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Ref       string    `json:"ref"`
	Head      string    `json:"head"`
	HeadShort string    `json:"head_short"`
	HeadTime  time.Time `json:"head_time"`
	Shallow   bool      `json:"shallow"`
	Oldest    time.Time `json:"oldest_commit"`
	Remote    string    `json:"remote,omitempty"`
	Note      string    `json:"note,omitempty"`
}

type WeekStats struct {
	Week         string `json:"week"`
	Commits      int    `json:"commits"`
	HumanCommits int    `json:"human_commits"`
	BotCommits   int    `json:"dependabot_commits"`
	PRs          int    `json:"merged_prs"`
	HumanPRs     int    `json:"human_prs"`
	BotPRs       int    `json:"dependabot_prs"`
	MergeCommits int    `json:"merge_commits"`
	SquashPRs    int    `json:"squash_prs"`
}

type Lib struct {
	Name     string
	Module   string
	Path     string
	Ref      string
	Main     []Commit // first-parent, oldest first
	byTime   []Commit // same, sorted by time
	Tip      Commit
	Changes  []*LibChange // first-parent commits in [contextFrom, to], oldest first
	wireRule func(pkg string) bool
	filesOf  map[string][]string
}

type LibChange struct {
	Commit
	Files           []string            `json:"files"`
	Packages        []string            `json:"packages"`
	RegenOnly       []string            `json:"regenerated_only_packages,omitempty"`
	GoChange        bool                `json:"go_change"`
	TestOnly        bool                `json:"test_only"`
	WireVisibleAuto bool                `json:"wire_visible_by_path"`
	ImportedBy      map[string][]string `json:"imported_by"`
	Class           string              `json:"class"`
	WireVisible     string              `json:"wire_visible"`
	Evidence        string              `json:"evidence,omitempty"`
	Note            string              `json:"note,omitempty"`
	Curated         bool                `json:"curated"`
	Uptake          []Uptake            `json:"uptake"`
	InWindow        bool                `json:"in_window"`
}

type Uptake struct {
	Consumer    string    `json:"consumer"`
	ConsumerSHA string    `json:"consumer_sha,omitempty"`
	Date        time.Time `json:"date,omitempty"`
	Days        float64   `json:"days"`
	PinSHA      string    `json:"pin_sha,omitempty"`
	Method      string    `json:"method"`
	NotYet      bool      `json:"not_yet"`
	Imports     bool      `json:"imports_changed_package"`
}

type Consumer struct {
	Name            string                 `json:"name"`
	Kind            string                 `json:"kind"`
	Repo            string                 `json:"repo"`
	Ref             string                 `json:"ref"`
	Head            string                 `json:"head"`
	Subdir          string                 `json:"subdir,omitempty"`
	GoMod           string                 `json:"gomod"`
	GoDirective     string                 `json:"go_directive"`
	Imports         map[string][]string    `json:"imports"`
	TestOnlyImports map[string][]string    `json:"test_only_imports"`
	Pins            map[string][]*PinEvent `json:"pins"`
	Current         map[string]*PinEvent   `json:"current"`
	Scan            *scanResult            `json:"scan"`
	imports         map[string]map[string]bool
}

type PinEvent struct {
	ConsumerSHA   string    `json:"consumer_sha"`
	ConsumerTime  time.Time `json:"consumer_time"`
	Subject       string    `json:"subject"`
	Version       string    `json:"version"`
	Removed       bool      `json:"removed,omitempty"`
	PinSHA        string    `json:"pin_sha"`
	PinFull       string    `json:"pin_full,omitempty"`
	PinTime       time.Time `json:"pin_time"`
	InLocal       bool      `json:"pin_in_local_clone"`
	OnMain        bool      `json:"pin_on_main"`
	MergeBase     string    `json:"merge_base,omitempty"`
	MergeBaseTime time.Time `json:"merge_base_time,omitempty"`
	MainHead      string    `json:"main_head_then"`
	MainHeadTime  time.Time `json:"main_head_then_time"`
	LagDays       float64   `json:"lag_days"`
	BaseLagDays   float64   `json:"base_lag_days"`
	CommitsBehind int       `json:"main_commits_behind_then"`
	Import        bool      `json:"subtree_import,omitempty"`
	Tag           bool      `json:"tagged_version,omitempty"`
}

type TodayLag struct {
	Consumer      string    `json:"consumer"`
	Lib           string    `json:"lib"`
	PinSHA        string    `json:"pin_sha"`
	PinTime       time.Time `json:"pin_time"`
	OnMain        bool      `json:"pin_on_main"`
	InLocal       bool      `json:"pin_in_local_clone"`
	MergeBase     string    `json:"merge_base,omitempty"`
	MergeBaseTime time.Time `json:"merge_base_time,omitempty"`
	TreeEqualMain string    `json:"tree_equal_main_commit,omitempty"`
	TipSHA        string    `json:"main_tip"`
	TipTime       time.Time `json:"main_tip_time"`
	LagDays       float64   `json:"lag_days"`
	BaseLagDays   float64   `json:"base_lag_days"`
	CommitsBehind int       `json:"main_commits_behind"`
	PinAgeDays    float64   `json:"pin_age_days"`
}

type Straddle struct {
	Lib       string   `json:"lib"`
	SHA       string   `json:"sha"`
	Subject   string   `json:"subject"`
	Class     string   `json:"class"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	Days      int      `json:"days"`
	Ahead     []string `json:"ahead_at_start"`
	Behind    []string `json:"behind_at_start"`
	BehindEnd []string `json:"behind_at_end"`
	Open      bool     `json:"still_open_today"`
}

type DaySnapshot struct {
	Day        string            `json:"day"`
	Pins       map[string]string `json:"pins"`
	Distinct   int               `json:"distinct_pins"`
	SpreadDays float64           `json:"spread_days"`
	OldestPin  string            `json:"oldest_pin_consumer"`
	NewestPin  string            `json:"newest_pin_consumer"`
	MaxLagDays float64           `json:"max_lag_vs_main_days"`
	MaxLagWho  string            `json:"max_lag_consumer"`
	MedianLag  float64           `json:"median_lag_vs_main_days"`
}

type Result struct {
	GeneratedAt time.Time                        `json:"generated_at"`
	Command     string                           `json:"reproduce_command"`
	Window      [2]string                        `json:"window"`
	Context     [2]string                        `json:"context"`
	Today       string                           `json:"today"`
	Repos       []RepoInfo                       `json:"repos"`
	Weekly      map[string][]*WeekStats          `json:"weekly"`
	WeeklyOrder []string                         `json:"weekly_repo_order"`
	Weeks       []string                         `json:"weeks"`
	Periods     map[string]map[string]*WeekStats `json:"periods"` // repo -> period -> stats
	Libraries   map[string]*LibSummary           `json:"libraries"`
	Consumers   []*Consumer                      `json:"consumers"`
	TodayLags   []TodayLag                       `json:"today_lags"`
	Snapshots   map[string][]DaySnapshot         `json:"snapshots"`
	Straddles   []Straddle                       `json:"straddles"`
	Caveats     []string                         `json:"caveats"`
}

type LibSummary struct {
	Module   string                    `json:"module"`
	Ref      string                    `json:"ref"`
	Tip      Commit                    `json:"tip"`
	Changes  []*LibChange              `json:"changes"`
	Counts   map[string]map[string]int `json:"counts"` // period -> metric -> n
	WireRule string                    `json:"wire_rule"`
}

// ---------------------------------------------------------------------------
// Curated classification
// ---------------------------------------------------------------------------

type curated struct {
	Class       string `json:"class"`
	WireVisible *bool  `json:"wire_visible,omitempty"`
	Evidence    string `json:"evidence"`
	Note        string `json:"note,omitempty"`
}

type classificationFile struct {
	Comment  string             `json:"_comment,omitempty"`
	Classes  map[string]string  `json:"classes,omitempty"`
	Libforge map[string]curated `json:"libforge"`
	Ucantone map[string]curated `json:"ucantone"`
}

func loadClassification(path string) (*classificationFile, error) {
	var cf classificationFile
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &cf, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, &cf); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &cf, nil
}

func (cf *classificationFile) lookup(lib, sha string) (curated, bool) {
	var m map[string]curated
	switch lib {
	case "libforge":
		m = cf.Libforge
	case "ucantone":
		m = cf.Ucantone
	}
	for k, v := range m {
		if strings.HasPrefix(sha, k) {
			return v, true
		}
	}
	return curated{}, false
}

// ---------------------------------------------------------------------------
// Library analysis
// ---------------------------------------------------------------------------

func loadLib(name, dir, ref string, verbose bool) (*Lib, error) {
	ref, err := resolveRef(dir, ref, "main", "HEAD")
	if err != nil {
		return nil, err
	}
	l := &Lib{Name: name, Module: libModules[name], Path: dir, Ref: ref, filesOf: map[string][]string{}}
	cs, err := gitLog(dir, ref, "--first-parent")
	if err != nil {
		return nil, err
	}
	for i := len(cs) - 1; i >= 0; i-- {
		l.Main = append(l.Main, cs[i])
	}
	l.Tip = l.Main[len(l.Main)-1]
	l.byTime = append([]Commit(nil), l.Main...)
	sort.SliceStable(l.byTime, func(i, j int) bool { return l.byTime[i].Time.Before(l.byTime[j].Time) })
	switch name {
	case "libforge":
		l.wireRule = func(pkg string) bool {
			return pkg == "commands" || strings.HasPrefix(pkg, "commands/") || pkg == "blobindex" || strings.HasPrefix(pkg, "blobindex/")
		}
	case "ucantone":
		l.wireRule = func(pkg string) bool {
			for _, p := range []string{"ucan", "validator", "execution", "varsig", "multikey", "did"} {
				if pkg == p || strings.HasPrefix(pkg, p+"/") {
					return true
				}
			}
			return false
		}
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "%s: %d first-parent commits on %s, tip %s (%s)\n", name, len(l.Main), ref, l.Tip.Short, l.Tip.Time.Format("2006-01-02"))
	}
	return l, nil
}

// mainHeadAt returns the newest first-parent main commit committed at or before t.
func (l *Lib) mainHeadAt(t time.Time) (Commit, bool) {
	i := sort.Search(len(l.byTime), func(i int) bool { return l.byTime[i].Time.After(t) })
	if i == 0 {
		return Commit{}, false
	}
	return l.byTime[i-1], true
}

func (l *Lib) changedFiles(c Commit) []string {
	if f, ok := l.filesOf[c.SHA]; ok {
		return f
	}
	var out string
	var err error
	if len(c.Parents) == 0 {
		out, err = git(l.Path, "show", "--format=", "--name-only", c.SHA)
	} else {
		out, err = git(l.Path, "diff", "--name-only", c.Parents[0], c.SHA)
	}
	if err != nil {
		return nil
	}
	files := strings.Fields(out)
	sort.Strings(files)
	l.filesOf[c.SHA] = files
	return files
}

// packagesOf maps changed files to library packages (import path suffixes).
// gen/ directories fold into the package they generate for. Packages where
// only generated codec files (`*_gen*.go`) changed are returned separately as
// regenOnly: a regeneration with no hand-written change (a generator version
// bump) is not counted as a change to the package. Test-only changes are
// reported but do not count as a package change either.
func packagesOf(files []string) (pkgs, regenOnly []string, goChange, testOnly bool) {
	set := map[string]bool{}
	regen := map[string]bool{}
	anyGo, anyNonTest := false, false
	for _, f := range files {
		if !strings.HasSuffix(f, ".go") {
			continue
		}
		anyGo = true
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		anyNonTest = true
		d := path.Dir(f)
		if d == "." {
			d = ""
		}
		d = strings.TrimSuffix(d, "/gen")
		if d == "gen" {
			d = ""
		}
		if isGenerated(f) {
			regen[d] = true
		} else {
			set[d] = true
		}
	}
	for p := range set {
		pkgs = append(pkgs, p)
	}
	for p := range regen {
		if !set[p] {
			regenOnly = append(regenOnly, p)
		}
	}
	sort.Strings(pkgs)
	sort.Strings(regenOnly)
	return pkgs, regenOnly, anyGo, anyGo && !anyNonTest
}

// isGenerated recognises the cbor-gen / dag-json-gen outputs used across the
// fil-forge repositories (cbor_gen.go, json_gen.go, dag_json_gen.maps.go, ...).
func isGenerated(f string) bool {
	return strings.Contains(path.Base(f), "_gen")
}

func (l *Lib) analyseChanges(o *options, consumers []*Consumer, cf *classificationFile) {
	end := o.to.Add(24 * time.Hour)
	for _, c := range l.Main {
		if c.Time.Before(o.contextFrom) || !c.Time.Before(end) {
			continue
		}
		ch := &LibChange{Commit: c, ImportedBy: map[string][]string{}}
		ch.Files = l.changedFiles(c)
		ch.Packages, ch.RegenOnly, ch.GoChange, ch.TestOnly = packagesOf(ch.Files)
		ch.InWindow = !c.Time.Before(o.from)
		for _, p := range ch.Packages {
			if l.wireRule(p) {
				ch.WireVisibleAuto = true
			}
		}
		for _, cons := range consumers {
			var hit []string
			for _, p := range ch.Packages {
				if cons.imports[l.Name][p] {
					hit = append(hit, p)
				}
			}
			if len(hit) > 0 {
				ch.ImportedBy[cons.Name] = hit
			}
		}
		switch {
		case c.Dependabot:
			ch.Class = "dependabot"
			ch.WireVisible = "no"
		case !ch.GoChange:
			ch.Class = "non-code"
			ch.WireVisible = "no"
		case ch.TestOnly:
			ch.Class = "test-only"
			ch.WireVisible = "no"
		case len(ch.Packages) == 0 && len(ch.RegenOnly) > 0:
			ch.Class = "regen-only"
			ch.WireVisible = "no"
		default:
			ch.Class = "UNCLASSIFIED"
			if l.Name == "libforge" {
				if ch.WireVisibleAuto {
					ch.WireVisible = "yes"
				} else {
					ch.WireVisible = "no"
				}
			} else if ch.WireVisibleAuto {
				ch.WireVisible = "candidate (unreviewed)"
			} else {
				ch.WireVisible = "no"
			}
		}
		if cur, ok := cf.lookup(l.Name, c.SHA); ok {
			ch.Curated = true
			ch.Class = cur.Class
			ch.Evidence = cur.Evidence
			ch.Note = cur.Note
			if cur.WireVisible != nil {
				if *cur.WireVisible {
					ch.WireVisible = "yes"
				} else {
					ch.WireVisible = "no"
				}
			}
		}
		l.Changes = append(l.Changes, ch)
	}
}

// includes reports whether the library state pinned by ev contains change c,
// and by which method it decided: ancestry (c is an ancestor of the pin),
// content (the pin is not on main but every file c touched is identical in
// the pin and in c), or timestamp (the pin is absent from the local clone, so
// only its pseudo-version time can be compared).
func (l *Lib) includes(c *LibChange, ev *PinEvent) (bool, string) {
	if ev.Removed {
		return false, "removed"
	}
	if !ev.InLocal {
		return !ev.PinTime.Before(c.Time), "timestamp"
	}
	if gitOK(l.Path, "merge-base", "--is-ancestor", c.SHA, ev.PinFull) {
		return true, "ancestry"
	}
	if ev.OnMain {
		return false, "ancestry"
	}
	if len(c.Files) > 0 {
		args := append([]string{"diff", "--quiet", c.SHA, ev.PinFull, "--"}, c.Files...)
		if gitOK(l.Path, args...) {
			return true, "content"
		}
	}
	return false, "content"
}

// treeEqualMain finds a first-parent main commit whose tree equals the pin's
// (a PR-branch head that was squash-merged unchanged), searching commits
// committed up to 14 days after the pin.
func (l *Lib) treeEqualMain(ev *PinEvent) string {
	if !ev.InLocal || ev.OnMain {
		return ""
	}
	limit := ev.PinTime.Add(14 * 24 * time.Hour)
	for _, m := range l.byTime {
		if m.Time.Before(ev.PinTime.Add(-24*time.Hour)) || m.Time.After(limit) {
			continue
		}
		if gitOK(l.Path, "diff", "--quiet", m.SHA, ev.PinFull) {
			return m.Short
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Consumers: import sets
// ---------------------------------------------------------------------------

// scanResult carries what the import scan saw besides the import sets, so the
// report can say how much of a consumer was read and what it could not parse.
type scanResult struct {
	Files     int      `json:"go_files_scanned"`
	ParseErrs []string `json:"parse_errors,omitempty"`
}

func scanImports(repo, ref, subdir string) (map[string]map[string]bool, map[string]map[string]bool, *scanResult, error) {
	args := []string{"-C", repo, "archive", "--format=tar", ref}
	if subdir != "" {
		args = append(args, subdir)
	}
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, err
	}
	nonTest := map[string]map[string]bool{"libforge": {}, "ucantone": {}}
	test := map[string]map[string]bool{"libforge": {}, "ucantone": {}}
	tr := tar.NewReader(stdout)
	fset := token.NewFileSet()
	sr := &scanResult{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, nil, err
		}
		if h.Typeflag != tar.TypeReg || !strings.HasSuffix(h.Name, ".go") {
			continue
		}
		if strings.Contains(h.Name, "/vendor/") || strings.HasPrefix(h.Name, "vendor/") || strings.Contains(h.Name, "/testdata/") {
			continue
		}
		src, err := io.ReadAll(tr)
		if err != nil {
			return nil, nil, nil, err
		}
		sr.Files++
		f, err := parser.ParseFile(fset, h.Name, src, parser.ImportsOnly)
		if err != nil {
			sr.ParseErrs = append(sr.ParseErrs, err.Error())
			continue
		}
		target := nonTest
		if strings.HasSuffix(h.Name, "_test.go") {
			target = test
		}
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			for lib, mod := range libModules {
				if p == mod {
					target[lib][""] = true
				} else if strings.HasPrefix(p, mod+"/") {
					target[lib][strings.TrimPrefix(p, mod+"/")] = true
				}
			}
		}
	}
	if err := cmd.Wait(); err != nil {
		return nil, nil, nil, fmt.Errorf("git archive %s %s: %v: %s", repo, ref, err, strings.TrimSpace(stderr.String()))
	}
	return nonTest, test, sr, nil
}

func sortedKeys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// Consumers: go.mod pin history
// ---------------------------------------------------------------------------

var rePseudo = regexp.MustCompile(`^v\d+\.\d+\.\d+-(?:[0-9A-Za-z.]*\.)?(\d{14})-([0-9a-f]{12})(?:\+incompatible)?$`)
var reRequire = regexp.MustCompile(`^([+-])\s*(?:require\s+)?(github\.com/fil-forge/(libforge|ucantone))\s+(\S+)`)

type rawPin struct {
	commit  Commit
	lib     string
	version string
	removed bool
	created bool
}

// gomodHistory walks the first-parent history of ref for changes to the
// libforge/ucantone require lines in gomod. When a merge commit *creates* the
// file (a subtree import, as forge's `chore: import <svc>` commits do), the
// walk continues into the merge's second parent so the imported history is
// included.
func gomodHistory(repo, ref, gomod string, depth int) ([]rawPin, error) {
	if depth > 3 {
		return nil, nil
	}
	out, err := git(repo, "log", "--first-parent", "--diff-merges=first-parent", "-p", logFormat, ref, "--", gomod)
	if err != nil {
		return nil, err
	}
	var pins []rawPin
	for _, rec := range strings.Split(out, "\x1e") {
		rec = strings.TrimLeft(rec, "\n")
		if strings.TrimSpace(rec) == "" {
			continue
		}
		c, diff := parseCommitRecord(rec)
		created := strings.Contains(diff, "\n--- /dev/null\n") || strings.Contains(diff, "\nnew file mode")
		var minus, plus []rawPin
		for _, line := range strings.Split(diff, "\n") {
			m := reRequire.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			p := rawPin{commit: c, lib: m[3], version: strings.TrimSuffix(m[4], ","), created: created}
			if strings.HasSuffix(p.version, "//") {
				p.version = strings.TrimSpace(strings.TrimSuffix(p.version, "//"))
			}
			if m[1] == "+" {
				plus = append(plus, p)
			} else {
				minus = append(minus, p)
			}
		}
		pins = append(pins, plus...)
		for _, mp := range minus {
			found := false
			for _, pp := range plus {
				if pp.lib == mp.lib {
					found = true
				}
			}
			if !found {
				mp.removed = true
				pins = append(pins, mp)
			}
		}
		if created && c.IsMerge && len(c.Parents) > 1 {
			more, err := gomodHistory(repo, c.Parents[1], gomod, depth+1)
			if err != nil {
				return nil, err
			}
			pins = append(pins, more...)
		}
	}
	return pins, nil
}

func (l *Lib) resolvePin(ev *PinEvent) {
	if m := rePseudo.FindStringSubmatch(ev.Version); m != nil {
		ev.PinSHA = m[2]
		ev.PinTime, _ = time.Parse("20060102150405", m[1])
	} else {
		// A tagged version: resolve the tag in the library clone.
		ev.Tag = true
		if out, err := git(l.Path, "rev-list", "-n1", ev.Version); err == nil {
			ev.PinSHA = short(strings.TrimSpace(out))
		}
	}
	if ev.PinSHA == "" {
		return
	}
	if out, err := git(l.Path, "rev-parse", "--verify", "--quiet", ev.PinSHA+"^{commit}"); err == nil {
		ev.PinFull = strings.TrimSpace(out)
		ev.InLocal = true
		if ev.Tag || ev.PinTime.IsZero() {
			if c, err := headCommit(l.Path, ev.PinFull); err == nil {
				ev.PinTime = c.Time
			}
		}
		ev.OnMain = gitOK(l.Path, "merge-base", "--is-ancestor", ev.PinFull, l.Ref)
		if !ev.OnMain {
			if out, err := git(l.Path, "merge-base", ev.PinFull, l.Ref); err == nil {
				mb := strings.TrimSpace(out)
				ev.MergeBase = short(mb)
				if c, err := headCommit(l.Path, mb); err == nil {
					ev.MergeBaseTime = c.Time
				}
			}
		}
	}
	if mh, ok := l.mainHeadAt(ev.ConsumerTime); ok {
		ev.MainHead = mh.Short
		ev.MainHeadTime = mh.Time
		ev.LagDays = days(mh.Time.Sub(ev.PinTime))
		base := ev.PinFull
		if !ev.OnMain && ev.MergeBase != "" {
			ev.BaseLagDays = days(mh.Time.Sub(ev.MergeBaseTime))
			base = ev.MergeBase
		} else {
			ev.BaseLagDays = ev.LagDays
		}
		if ev.InLocal {
			if out, err := git(l.Path, "rev-list", "--count", "--first-parent", base+".."+mh.SHA); err == nil {
				ev.CommitsBehind, _ = strconv.Atoi(strings.TrimSpace(out))
			}
		} else {
			ev.CommitsBehind = -1
		}
	}
}

func days(d time.Duration) float64 {
	return float64(int(d.Hours()/24*10+0.5)) / 10 // one decimal
}

func loadConsumer(name, kind, repo, ref, subdir string, libsByName map[string]*Lib, verbose bool) (*Consumer, error) {
	ref, err := resolveRef(repo, ref, "main", "HEAD")
	if err != nil {
		return nil, err
	}
	hc, err := headCommit(repo, ref)
	if err != nil {
		return nil, err
	}
	c := &Consumer{Name: name, Kind: kind, Repo: repo, Ref: ref, Head: hc.SHA, Subdir: subdir,
		Imports: map[string][]string{}, TestOnlyImports: map[string][]string{},
		Pins: map[string][]*PinEvent{}, Current: map[string]*PinEvent{}}
	c.GoMod = "go.mod"
	if subdir != "" {
		c.GoMod = path.Join(subdir, "go.mod")
	}
	if out, err := git(repo, "show", ref+":"+c.GoMod); err == nil {
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, "go ") {
				c.GoDirective = strings.TrimSpace(strings.TrimPrefix(l, "go "))
			}
		}
	}
	nonTest, test, sr, err := scanImports(repo, ref, subdir)
	if err != nil {
		return nil, err
	}
	c.imports = nonTest
	c.Scan = sr
	for lib := range libModules {
		c.Imports[lib] = sortedKeys(nonTest[lib])
		var only []string
		for p := range test[lib] {
			if !nonTest[lib][p] {
				only = append(only, p)
			}
		}
		sort.Strings(only)
		c.TestOnlyImports[lib] = only
	}
	raw, err := gomodHistory(repo, ref, c.GoMod, 0)
	if err != nil {
		return nil, err
	}
	for _, rp := range raw {
		ev := &PinEvent{ConsumerSHA: rp.commit.Short, ConsumerTime: rp.commit.Time, Subject: rp.commit.Subject,
			Version: rp.version, Removed: rp.removed, Import: rp.created && rp.commit.IsMerge}
		if !rp.removed {
			libsByName[rp.lib].resolvePin(ev)
		}
		c.Pins[rp.lib] = append(c.Pins[rp.lib], ev)
	}
	for lib, evs := range c.Pins {
		sort.SliceStable(evs, func(i, j int) bool { return evs[i].ConsumerTime.Before(evs[j].ConsumerTime) })
		// Drop exact duplicates (the same version re-stated in the same commit).
		var dedup []*PinEvent
		for _, e := range evs {
			if n := len(dedup); n > 0 && dedup[n-1].ConsumerSHA == e.ConsumerSHA && dedup[n-1].Version == e.Version {
				continue
			}
			dedup = append(dedup, e)
		}
		c.Pins[lib] = dedup
		if len(dedup) > 0 && !dedup[len(dedup)-1].Removed {
			c.Current[lib] = dedup[len(dedup)-1]
		}
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "%s: %s@%s files=%d parse-errors=%d imports libforge=%d ucantone=%d pins libforge=%d ucantone=%d\n", name, ref, short(hc.SHA),
			sr.Files, len(sr.ParseErrs), len(c.Imports["libforge"]), len(c.Imports["ucantone"]), len(c.Pins["libforge"]), len(c.Pins["ucantone"]))
		for _, e := range sr.ParseErrs {
			fmt.Fprintf(os.Stderr, "   parse error: %s\n", e)
		}
	}
	return c, nil
}

// pinAsOf returns the consumer's pin for lib in force at the end of day d.
func (c *Consumer) pinAsOf(lib string, d time.Time) *PinEvent {
	end := d.Add(24 * time.Hour)
	var cur *PinEvent
	for _, ev := range c.Pins[lib] {
		if ev.ConsumerTime.Before(end) {
			cur = ev
		}
	}
	if cur != nil && cur.Removed {
		return nil
	}
	return cur
}

// ---------------------------------------------------------------------------
// forge modules from go.work
// ---------------------------------------------------------------------------

func forgeModules(forge, ref string) ([]string, error) {
	out, err := git(forge, "show", ref+":go.work")
	if err != nil {
		return nil, err
	}
	var mods []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "./") {
			mods = append(mods, strings.TrimPrefix(l, "./"))
		} else if strings.HasPrefix(l, "use ./") {
			mods = append(mods, strings.TrimPrefix(l, "use ./"))
		}
	}
	var keep []string
	for _, m := range mods {
		gm, err := git(forge, "show", ref+":"+m+"/go.mod")
		if err != nil {
			continue
		}
		if strings.Contains(gm, libforgeMod) || strings.Contains(gm, ucantoneMod) {
			keep = append(keep, m)
		}
	}
	return keep, nil
}

// ---------------------------------------------------------------------------
// Weekly statistics
// ---------------------------------------------------------------------------

func isoWeek(t time.Time) string {
	y, w := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}

func weekStart(t time.Time) time.Time {
	t = t.Truncate(24 * time.Hour)
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	return t.AddDate(0, 0, -(wd - 1))
}

func addStats(ws *WeekStats, c Commit) {
	ws.Commits++
	if c.Dependabot {
		ws.BotCommits++
	} else {
		ws.HumanCommits++
	}
	isPR := c.IsMerge || c.PR > 0
	if c.IsMerge {
		ws.MergeCommits++
	} else if c.PR > 0 {
		ws.SquashPRs++
	}
	if isPR {
		ws.PRs++
		if c.Dependabot {
			ws.BotPRs++
		} else {
			ws.HumanPRs++
		}
	}
}

func weeklyStats(dir, ref string, firstParent bool, from, to time.Time) (map[string]*WeekStats, map[string]*WeekStats, error) {
	args := []string{}
	if firstParent {
		args = append(args, "--first-parent")
	}
	args = append(args, "--since="+from.Format(time.RFC3339), "--until="+to.Add(24*time.Hour).Format(time.RFC3339))
	cs, err := gitLog(dir, ref, args...)
	if err != nil {
		return nil, nil, err
	}
	weeks := map[string]*WeekStats{}
	periods := map[string]*WeekStats{}
	for _, c := range cs {
		if c.Time.Before(from) || !c.Time.Before(to.Add(24*time.Hour)) {
			continue
		}
		w := isoWeek(c.Time)
		if weeks[w] == nil {
			weeks[w] = &WeekStats{Week: w}
		}
		addStats(weeks[w], c)
		p := c.Time.Format("2006-01")
		if periods[p] == nil {
			periods[p] = &WeekStats{Week: p}
		}
		addStats(periods[p], c)
	}
	return weeks, periods, nil
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	o, err := parseOptions()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	res, err := run(o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if err := writeOutputs(o, res); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println("repositories read:")
	for _, r := range res.Repos {
		sh := ""
		if r.Shallow {
			sh = fmt.Sprintf(" (shallow; oldest %s)", r.Oldest.Format("2006-01-02"))
		}
		fmt.Printf("  %-22s %-12s %s %s%s\n", r.Name, r.Ref, r.Head, r.HeadTime.Format("2006-01-02"), sh)
	}
	fmt.Printf("wrote %s.md and %s.json\n", o.out, o.out)
}

func run(o *options) (*Result, error) {
	res := &Result{GeneratedAt: time.Now().UTC(), Weekly: map[string][]*WeekStats{}, Periods: map[string]map[string]*WeekStats{},
		Libraries: map[string]*LibSummary{}, Snapshots: map[string][]DaySnapshot{}}
	res.Window = [2]string{o.from.Format("2006-01-02"), o.to.Format("2006-01-02")}
	res.Context = [2]string{o.contextFrom.Format("2006-01-02"), o.from.AddDate(0, 0, -1).Format("2006-01-02")}
	res.Today = o.today.Format("2006-01-02")
	res.Command = reproduceCommand(o)

	cf, err := loadClassification(o.classification)
	if err != nil {
		return nil, err
	}

	// Libraries.
	libsByName := map[string]*Lib{}
	for name, dir := range map[string]string{"libforge": o.libforge, "ucantone": o.ucantone} {
		l, err := loadLib(name, dir, o.libRef, o.verbose)
		if err != nil {
			return nil, err
		}
		libsByName[name] = l
		ri, err := repoInfo(name, dir, l.Ref)
		if err != nil {
			return nil, err
		}
		res.Repos = append(res.Repos, ri)
	}

	// Consumers.
	var consumers []*Consumer
	forgeRef, err := resolveRef(o.forge, o.forgeRef, "HEAD")
	if err != nil {
		return nil, err
	}
	mods, err := forgeModules(o.forge, forgeRef)
	if err != nil {
		return nil, err
	}
	ri, err := repoInfo("forge", o.forge, forgeRef)
	if err != nil {
		return nil, err
	}
	ri.Note = fmt.Sprintf("modules with libforge/ucantone requires: %s", strings.Join(mods, ", "))
	res.Repos = append(res.Repos, ri)
	for _, m := range mods {
		c, err := loadConsumer("forge/"+path.Base(m), "forge-module", o.forge, forgeRef, m, libsByName, o.verbose)
		if err != nil {
			return nil, fmt.Errorf("forge module %s: %w", m, err)
		}
		consumers = append(consumers, c)
	}
	for _, r := range o.liveRepos {
		dir := filepath.Join(o.liveDir, r)
		if _, err := os.Stat(dir); err != nil {
			res.Caveats = append(res.Caveats, fmt.Sprintf("live repo `%s` not found at `%s`; skipped", r, dir))
			continue
		}
		c, err := loadConsumer("live/"+r, "live-service", dir, o.liveRef, "", libsByName, o.verbose)
		if err != nil {
			return nil, fmt.Errorf("live repo %s: %w", r, err)
		}
		consumers = append(consumers, c)
		ri, err := repoInfo("live/"+r, dir, c.Ref)
		if err != nil {
			return nil, err
		}
		res.Repos = append(res.Repos, ri)
	}
	for _, x := range []struct{ name, dir string }{{"guppy", o.guppy}, {"indexing-service", o.indexing}} {
		if x.dir == "" {
			continue
		}
		c, err := loadConsumer(x.name, "library-consumer", x.dir, o.consumerRef, "", libsByName, o.verbose)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", x.name, err)
		}
		consumers = append(consumers, c)
		ri, err := repoInfo(x.name, x.dir, c.Ref)
		if err != nil {
			return nil, err
		}
		res.Repos = append(res.Repos, ri)
	}
	res.Consumers = consumers

	// (a) weekly statistics. forge is counted first-parent only: its history
	// contains the imported service histories, which would double count.
	type repoRef struct {
		name, dir, ref string
		firstParent    bool
	}
	var wrepos []repoRef
	for _, n := range []string{"libforge", "ucantone"} {
		wrepos = append(wrepos, repoRef{n, libsByName[n].Path, libsByName[n].Ref, false})
	}
	for _, c := range consumers {
		switch c.Kind {
		case "live-service", "library-consumer":
			wrepos = append(wrepos, repoRef{c.Name, c.Repo, c.Ref, false})
		}
	}
	wrepos = append(wrepos, repoRef{"forge (first-parent)", o.forge, forgeRef, true})
	weekSet := map[string]bool{}
	for _, wr := range wrepos {
		weeks, periods, err := weeklyStats(wr.dir, wr.ref, wr.firstParent, o.contextFrom, o.to)
		if err != nil {
			return nil, err
		}
		res.WeeklyOrder = append(res.WeeklyOrder, wr.name)
		res.Periods[wr.name] = periods
		var list []*WeekStats
		for w, ws := range weeks {
			weekSet[w] = true
			list = append(list, ws)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Week < list[j].Week })
		res.Weekly[wr.name] = list
	}
	for d := weekStart(o.contextFrom); !d.After(o.to); d = d.AddDate(0, 0, 7) {
		weekSet[isoWeek(d)] = true
	}
	for w := range weekSet {
		res.Weeks = append(res.Weeks, w)
	}
	sort.Strings(res.Weeks)

	// (b)+(c) library changes, classification and uptake.
	for _, name := range libs {
		l := libsByName[name]
		l.analyseChanges(o, consumers, cf)
		for _, ch := range l.Changes {
			if ch.Class == "dependabot" || ch.Class == "non-code" || ch.Class == "test-only" || ch.Class == "regen-only" {
				continue
			}
			for _, cons := range consumers {
				up := Uptake{Consumer: cons.Name, NotYet: true, Imports: len(ch.ImportedBy[cons.Name]) > 0}
				for _, ev := range cons.Pins[name] {
					if ev.ConsumerTime.Before(ch.Time.Add(-30 * 24 * time.Hour)) {
						// A pin older than the change by a month cannot contain it; skip the git calls.
						continue
					}
					ok, method := l.includes(ch, ev)
					if ok {
						up.NotYet = false
						up.ConsumerSHA = ev.ConsumerSHA
						up.Date = ev.ConsumerTime
						up.Days = days(ev.ConsumerTime.Sub(ch.Time))
						up.PinSHA = ev.PinSHA
						up.Method = method
						break
					}
				}
				if up.NotYet {
					if cur := cons.Current[name]; cur != nil {
						up.PinSHA = cur.PinSHA
					}
				}
				ch.Uptake = append(ch.Uptake, up)
			}
		}
		ls := &LibSummary{Module: l.Module, Ref: l.Ref, Tip: l.Tip, Changes: l.Changes, Counts: map[string]map[string]int{}}
		switch name {
		case "libforge":
			ls.WireRule = "a change to a non-test .go file under `commands/**` or `blobindex/**`"
		case "ucantone":
			ls.WireRule = "a change under `ucan/`, `validator/`, `execution/`, `varsig/`, `multikey/` or `did/` that alters an exported type, an encoding or a validation rule (path match = candidate; final call curated per commit from the diff)"
		}
		for _, ch := range l.Changes {
			for _, period := range []string{"context", "window"} {
				if (period == "window") != ch.InWindow {
					continue
				}
				m := ls.Counts[period]
				if m == nil {
					m = map[string]int{}
					ls.Counts[period] = m
				}
				m["commits"]++
				if ch.Dependabot {
					m["dependabot"]++
				} else {
					m["human"]++
					if ch.GoChange && !ch.TestOnly {
						m["human_code"]++
					}
				}
				if len(ch.ImportedBy) > 0 {
					m["changed_imported_package"]++
					if ch.WireVisible == "yes" {
						m["imported_wire_visible"]++
					} else {
						m["imported_internal"]++
					}
				}
				if ch.WireVisible == "yes" {
					m["wire_visible"]++
				}
				m["class:"+ch.Class]++
				if ch.Class == "breaking" || ch.Class == "additive-required" {
					m["needs_coordination"]++
				}
			}
		}
		res.Libraries[name] = ls
	}

	// (d) lag today.
	for _, cons := range consumers {
		for _, name := range libs {
			l := libsByName[name]
			cur := cons.Current[name]
			if cur == nil {
				continue
			}
			tl := TodayLag{Consumer: cons.Name, Lib: name, PinSHA: cur.PinSHA, PinTime: cur.PinTime, OnMain: cur.OnMain, InLocal: cur.InLocal,
				MergeBase: cur.MergeBase, MergeBaseTime: cur.MergeBaseTime, TipSHA: l.Tip.Short, TipTime: l.Tip.Time}
			tl.LagDays = days(l.Tip.Time.Sub(cur.PinTime))
			tl.PinAgeDays = days(o.today.Add(24 * time.Hour).Sub(cur.PinTime))
			tl.BaseLagDays = tl.LagDays
			base := cur.PinFull
			if !cur.OnMain && cur.MergeBase != "" {
				tl.BaseLagDays = days(l.Tip.Time.Sub(cur.MergeBaseTime))
				base = cur.MergeBase
				tl.TreeEqualMain = l.treeEqualMain(cur)
			}
			if cur.InLocal {
				if out, err := git(l.Path, "rev-list", "--count", "--first-parent", base+".."+l.Tip.SHA); err == nil {
					tl.CommitsBehind, _ = strconv.Atoi(strings.TrimSpace(out))
				}
			} else {
				tl.CommitsBehind = -1
			}
			res.TodayLags = append(res.TodayLags, tl)
		}
	}

	// (e) daily snapshots and straddles over the non-forge consumers.
	var fleet []*Consumer
	for _, c := range consumers {
		if c.Kind != "forge-module" {
			fleet = append(fleet, c)
		}
	}
	for _, name := range libs {
		l := libsByName[name]
		var snaps []DaySnapshot
		for d := o.contextFrom; !d.After(o.today); d = d.AddDate(0, 0, 1) {
			snap := DaySnapshot{Day: d.Format("2006-01-02"), Pins: map[string]string{}}
			mh, _ := l.mainHeadAt(d.Add(24 * time.Hour))
			var oldest, newest *PinEvent
			var lags []float64
			distinct := map[string]bool{}
			for _, c := range fleet {
				ev := c.pinAsOf(name, d)
				if ev == nil || ev.PinSHA == "" {
					continue
				}
				snap.Pins[c.Name] = ev.PinSHA
				distinct[ev.PinSHA] = true
				if oldest == nil || ev.PinTime.Before(oldest.PinTime) {
					oldest = ev
					snap.OldestPin = c.Name
				}
				if newest == nil || ev.PinTime.After(newest.PinTime) {
					newest = ev
					snap.NewestPin = c.Name
				}
				lag := days(mh.Time.Sub(ev.PinTime))
				lags = append(lags, lag)
				if lag > snap.MaxLagDays {
					snap.MaxLagDays = lag
					snap.MaxLagWho = c.Name
				}
			}
			snap.Distinct = len(distinct)
			if oldest != nil && newest != nil {
				snap.SpreadDays = days(newest.PinTime.Sub(oldest.PinTime))
			}
			if len(lags) > 0 {
				sort.Float64s(lags)
				snap.MedianLag = lags[len(lags)/2]
			}
			snaps = append(snaps, snap)
		}
		res.Snapshots[name] = snaps

		for _, ch := range l.Changes {
			if ch.Class != "breaking" && ch.Class != "additive-required" {
				continue
			}
			var cur *Straddle
			for d := o.contextFrom; !d.After(o.today); d = d.AddDate(0, 0, 1) {
				if d.Add(24 * time.Hour).Before(ch.Time) {
					continue
				}
				var ahead, behind []string
				for _, c := range fleet {
					if len(ch.ImportedBy[c.Name]) == 0 {
						continue
					}
					ev := c.pinAsOf(name, d)
					if ev == nil || ev.PinSHA == "" {
						continue
					}
					if ok, _ := l.includes(ch, ev); ok {
						ahead = append(ahead, c.Name)
					} else {
						behind = append(behind, c.Name)
					}
				}
				straddling := len(ahead) > 0 && len(behind) > 0
				if straddling && cur == nil {
					cur = &Straddle{Lib: name, SHA: ch.Short, Subject: ch.Subject, Class: ch.Class, From: d.Format("2006-01-02"), Ahead: ahead, Behind: behind}
				}
				if cur != nil {
					if straddling {
						cur.To = d.Format("2006-01-02")
						cur.BehindEnd = behind
						cur.Days++
						if d.Equal(o.today) {
							cur.Open = true
						}
					} else {
						res.Straddles = append(res.Straddles, *cur)
						cur = nil
					}
				}
			}
			if cur != nil {
				res.Straddles = append(res.Straddles, *cur)
			}
		}
	}

	// Caveats derived from what was read.
	for _, r := range res.Repos {
		if r.Shallow {
			res.Caveats = append(res.Caveats, fmt.Sprintf("`%s` is a shallow clone: history starts %s; pin events and commit counts before that date are invisible.", r.Name, r.Oldest.Format("2006-01-02")))
		}
	}
	res.Caveats = append(res.Caveats,
		"Squash merges collapse a PR into one commit: \"merged PRs\" counts merge commits plus commits whose subject ends in `(#N)`; PRs merged with other subjects, and direct pushes to main, are counted as commits only.",
		"Dependabot is identified by author name. Its commits are counted separately everywhere and are excluded from the change classification.",
		"Dates are committer dates in UTC; a squash commit's date is the merge time, not the authoring time. Pin dates come from the pseudo-version timestamp, which is the pinned commit's committer time.",
		"\"Lag\" is the days between the pinned library commit and the library's newest first-parent main commit at the moment of the consumer commit (or at the library tip for \"today\"). For a pin that is not on main, the base lag measures from its merge-base with main instead; a PR-branch head can be tree-identical to the squash commit that merged it.",
		"The forge monorepo froze at its last push; its per-module go.mod histories before the import commits are the services' own pre-monorepo histories (paths rewritten), so forge/<svc> and live/<svc> share every event before 2026-07-30.",
		"Import sets are a static scan of the consumers' Go files at the analysed ref with `go/parser` (non-test files; test-only imports listed separately); import paths that only appear in comments or strings are not counted, so a plain grep gives larger sets. A library commit \"changed an imported package\" when a hand-written, non-test .go file in a package a consumer imports changed; codegen `gen/` directories fold into their parent package and packages where only `*_gen*.go` files changed are listed as regenerated-only and not counted.",
		"Whether a pin \"includes\" a library change is decided by ancestry; for pins that are PR-branch heads not on main, by per-file content identity with the change; for SHAs absent from the local library clone, by timestamp only (marked).",
	)
	return res, nil
}

func reproduceCommand(o *options) string {
	var b strings.Builder
	b.WriteString("cd tools/divergence && GOWORK=off go run .")
	fmt.Fprintf(&b, " -from %s -to %s -context-from %s -today %s", o.from.Format("2006-01-02"), o.to.Format("2006-01-02"), o.contextFrom.Format("2006-01-02"), o.today.Format("2006-01-02"))
	if o.forgeRef != "HEAD" {
		fmt.Fprintf(&b, " -forge-ref %s", o.forgeRef)
	}
	if o.libRef != "origin/main" {
		fmt.Fprintf(&b, " -lib-ref %s", o.libRef)
	}
	if o.consumerRef != "origin/main" {
		fmt.Fprintf(&b, " -consumer-ref %s", o.consumerRef)
	}
	if o.liveRef != "HEAD" {
		fmt.Fprintf(&b, " -live-ref %s", o.liveRef)
	}
	if o.libforge != "/home/user/libforge" {
		fmt.Fprintf(&b, " -libforge %s", o.libforge)
	}
	if o.ucantone != "/home/user/ucantone" {
		fmt.Fprintf(&b, " -ucantone %s", o.ucantone)
	}
	if o.liveDir != "/home/user/fil-forge" {
		fmt.Fprintf(&b, " -live-dir %s", o.liveDir)
	}
	return b.String()
}
