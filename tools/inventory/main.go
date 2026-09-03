// Command inventory produces the per-package evidence behind the libforge
// consolidation decision: for every package in each subject module it reports
// size, who imports it (by module), its transitive closure and external module
// count, whether it touches the wire contract, whether it registers anything in
// init(), and whether any shipped binary reaches it.
//
// It is deliberately a re-runnable tool rather than a hand-written table, so the
// inventory cannot go stale silently. Standard library only.
//
// Typical invocation (from tools/inventory, GOWORK=off so the tool's own module
// is not part of the workspace):
//
//	GOWORK=off go run . \
//	  -subject /home/user/libforge -subject /home/user/guppy -subject /home/user/ucantone \
//	  -consumer /home/user/wt/forge-baseline/hilt,... \
//	  -out ../../docs/consolidation/package-inventory.md \
//	  -json ../../docs/consolidation/package-inventory.json
//
// Static facts (LOC, imports, init bodies) come from go/parser over the module
// tree. Transitive facts (dependency closure, module of each dependency,
// reachability from package main) come from `go list` run inside each module
// with GOWORK=off; when `go list` fails for a module the tool degrades to
// direct-import approximations and says so in the output.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Package is one row of the inventory.
type Package struct {
	Module     string `json:"module"`
	ImportPath string `json:"importPath"`
	Rel        string `json:"rel"` // path relative to the module root ("." for the root package)
	Name       string `json:"name"`
	IsMain     bool   `json:"isMain"`
	IsGen      bool   `json:"isGen"` // a codegen driver (…/gen), a main package that is not shipped

	Files     int `json:"files"`     // non-test .go files
	TestFiles int `json:"testFiles"` // *_test.go files
	LOC       int `json:"loc"`       // lines in non-test .go files
	GenLOC    int `json:"genLoc"`    // of which in generated files
	HandLOC   int `json:"handLoc"`   // LOC - GenLOC
	TestLOC   int `json:"testLoc"`   // lines in *_test.go files

	Imports     []string `json:"imports"`     // direct non-test imports
	TestImports []string `json:"testImports"` // direct imports from *_test.go only

	// Importers maps consumer module path -> number of non-test files importing this package.
	Importers     map[string]int `json:"importers"`
	TestImporters map[string]int `json:"testImporters"`

	InternalClosure int      `json:"internalClosure"` // transitive same-module packages (excluding self)
	ExternalModules []string `json:"externalModules"` // distinct modules in the transitive closure (std excluded, self excluded)
	ClosureSource   string   `json:"closureSource"`   // "go list" or "direct imports (go list failed)"

	ImportsProtocol bool     `json:"importsProtocol"` // transitively imports libforge commands/** or blobindex/**
	InitCalls       []string `json:"initCalls"`       // callee names inside func init() bodies, if any
	HasInit         bool     `json:"hasInit"`

	Shipped      bool     `json:"shipped"`      // reachable from a non-gen package main in some consumer or subject module
	ShippedBy    []string `json:"shippedBy"`    // modules whose binaries reach it
	Reachability string   `json:"reachability"` // "shipped", "test-only", "library-only" (imported by non-test code but no binary reaches it), "unreferenced"
}

type moduleInfo struct {
	Dir  string
	Path string
	SHA  string
}

var protocolPrefixes = []string{
	"github.com/fil-forge/libforge/commands",
	"github.com/fil-forge/libforge/blobindex",
	// The in-repo copy on the consolidation POC branch counts as protocol too.
	"github.com/fil-forge/forge/commands",
}

type listFlag []string

func (l *listFlag) String() string     { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error { *l = append(*l, strings.Split(v, ",")...); return nil }

func main() {
	var subjects, consumers listFlag
	out := flag.String("out", "", "markdown output path (default: stdout)")
	jsonOut := flag.String("json", "", "JSON output path (optional)")
	noReach := flag.Bool("no-reach", false, "skip reachability analysis (no `go list` in consumer modules)")
	flag.Var(&subjects, "subject", "subject module directory (repeatable or comma-separated)")
	flag.Var(&consumers, "consumer", "consumer module directory (repeatable or comma-separated)")
	flag.Parse()
	if len(subjects) == 0 {
		fatalf("at least one -subject is required")
	}

	var mods []moduleInfo
	for _, d := range subjects {
		mods = append(mods, mustModule(d))
	}
	var cons []moduleInfo
	for _, d := range consumers {
		cons = append(cons, mustModule(d))
	}

	// Every module we look at (subjects and consumers) is also a potential
	// importer of every subject package; guppy is both a subject and a consumer.
	importerMods := append(append([]moduleInfo{}, mods...), cons...)
	importerMods = dedupeModules(importerMods)

	var warnings []string
	var rows []*Package
	byImportPath := map[string]*Package{}
	for _, m := range mods {
		pkgs, err := scanModule(m)
		if err != nil {
			fatalf("scan %s: %v", m.Dir, err)
		}
		for _, p := range pkgs {
			byImportPath[p.ImportPath] = p
		}
		rows = append(rows, pkgs...)
		if w := enrichWithGoList(m, pkgs); w != "" {
			warnings = append(warnings, w)
		}
	}

	// Importers: static scan of every importer module's non-test and test files.
	for _, im := range importerMods {
		files, err := goFiles(im.Dir)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", im.Dir, err))
			continue
		}
		for _, f := range files {
			imps, _, err := fileImports(f)
			if err != nil {
				continue
			}
			isTest := strings.HasSuffix(f, "_test.go")
			seen := map[string]bool{}
			for _, ip := range imps {
				p, ok := byImportPath[ip]
				if !ok || seen[ip] || p.Module == im.Path && !isTest && sameModuleSelfImport(p, f, im) {
					continue
				}
				seen[ip] = true
				if isTest {
					p.TestImporters[im.Path]++
				} else {
					p.Importers[im.Path]++
				}
			}
		}
	}

	// Reachability from shipped binaries.
	if !*noReach {
		for _, im := range importerMods {
			reach, err := reachableFromMains(im)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("reachability skipped for %s: %v", im.Path, err))
				continue
			}
			for ip := range reach {
				if p, ok := byImportPath[ip]; ok {
					p.Shipped = true
					p.ShippedBy = append(p.ShippedBy, im.Path)
				}
			}
		}
	}
	for _, p := range rows {
		sort.Strings(p.ShippedBy)
		switch {
		case p.Shipped:
			p.Reachability = "shipped"
		case len(p.Importers) > 0:
			p.Reachability = "library-only"
		case len(p.TestImporters) > 0:
			p.Reachability = "test-only"
		default:
			p.Reachability = "unreferenced"
		}
		if *noReach {
			p.Reachability = "(reachability skipped)"
		}
	}

	report := render(mods, importerMods, rows, warnings, *noReach)
	if *out == "" {
		os.Stdout.WriteString(report)
	} else if err := os.WriteFile(*out, []byte(report), 0o644); err != nil {
		fatalf("write %s: %v", *out, err)
	}
	if *jsonOut != "" {
		b, err := json.MarshalIndent(struct {
			Subjects  []moduleInfo `json:"subjects"`
			Importers []moduleInfo `json:"importers"`
			Packages  []*Package   `json:"packages"`
			Warnings  []string     `json:"warnings"`
		}{mods, importerMods, rows, warnings}, "", "  ")
		if err != nil {
			fatalf("json: %v", err)
		}
		if err := os.WriteFile(*jsonOut, append(b, '\n'), 0o644); err != nil {
			fatalf("write %s: %v", *jsonOut, err)
		}
	}
}

// sameModuleSelfImport reports whether f (a file inside importer module im)
// importing p is an intra-module import of the subject itself. Intra-module
// imports are not "consumers" and are counted separately as the internal
// closure, so they are excluded from the importer table.
func sameModuleSelfImport(p *Package, f string, im moduleInfo) bool {
	return p.Module == im.Path
}

func mustModule(dir string) moduleInfo {
	dir = filepath.Clean(dir)
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		fatalf("%s: %v", dir, err)
	}
	path := ""
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "module ") {
			path = strings.TrimSpace(strings.TrimPrefix(line, "module "))
			break
		}
	}
	if path == "" {
		fatalf("%s/go.mod: no module line", dir)
	}
	sha := "?"
	if out, err := exec.Command("git", "-C", dir, "rev-parse", "--short", "HEAD").Output(); err == nil {
		sha = strings.TrimSpace(string(out))
	}
	return moduleInfo{Dir: dir, Path: path, SHA: sha}
}

func dedupeModules(ms []moduleInfo) []moduleInfo {
	seen := map[string]bool{}
	var out []moduleInfo
	for _, m := range ms {
		if seen[m.Dir] {
			continue
		}
		seen[m.Dir] = true
		out = append(out, m)
	}
	return out
}

// goFiles lists every .go file under a module root, skipping nested modules,
// testdata, vendor and hidden directories.
func goFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p == root {
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "vendor" {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil {
				return filepath.SkipDir // nested module
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

// fileImports parses only the import block; returns imports and the package name.
func fileImports(path string) ([]string, string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		return nil, "", err
	}
	var imps []string
	for _, s := range f.Imports {
		imps = append(imps, strings.Trim(s.Path.Value, `"`))
	}
	return imps, f.Name.Name, nil
}

func isGenerated(path string, head []byte) bool {
	base := filepath.Base(path)
	if strings.Contains(base, "_gen") || base == "cbor_gen.go" || base == "json_gen.go" {
		return true
	}
	// The convention from https://go.dev/s/generatedcode.
	return bytes.Contains(head, []byte("DO NOT EDIT"))
}

func countLines(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	n := bytes.Count(b, []byte{'\n'})
	if b[len(b)-1] != '\n' {
		n++
	}
	return n
}

// initCalls returns the callee expressions inside every func init() in a file.
func initCalls(path string) ([]string, bool, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, false, err
	}
	var calls []string
	has := false
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Recv != nil || fd.Name.Name != "init" || fd.Body == nil {
			continue
		}
		has = true
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				calls = append(calls, exprString(c.Fun))
			}
			return true
		})
	}
	return calls, has, nil
}

func exprString(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return exprString(x.X) + "." + x.Sel.Name
	case *ast.IndexExpr:
		return exprString(x.X)
	case *ast.CallExpr:
		return exprString(x.Fun) + "()"
	case *ast.ParenExpr:
		return exprString(x.X)
	case *ast.StarExpr:
		return "*" + exprString(x.X)
	default:
		return "?"
	}
}

// scanModule builds the static part of every package row in a module.
func scanModule(m moduleInfo) ([]*Package, error) {
	files, err := goFiles(m.Dir)
	if err != nil {
		return nil, err
	}
	byDir := map[string]*Package{}
	for _, f := range files {
		dir := filepath.Dir(f)
		rel, _ := filepath.Rel(m.Dir, dir)
		rel = filepath.ToSlash(rel)
		p := byDir[dir]
		if p == nil {
			ip := m.Path
			if rel != "." {
				ip = m.Path + "/" + rel
			}
			p = &Package{Module: m.Path, ImportPath: ip, Rel: rel, Importers: map[string]int{}, TestImporters: map[string]int{}}
			p.IsGen = rel == "gen" || strings.HasSuffix(rel, "/gen")
			byDir[dir] = p
		}
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		imps, name, err := fileImports(f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		isTest := strings.HasSuffix(f, "_test.go")
		if isTest {
			p.TestFiles++
			p.TestLOC += countLines(b)
			p.TestImports = append(p.TestImports, imps...)
			continue
		}
		p.Files++
		n := countLines(b)
		p.LOC += n
		head := b
		if len(head) > 2048 {
			head = head[:2048]
		}
		if isGenerated(f, head) {
			p.GenLOC += n
		}
		if p.Name == "" {
			p.Name = name
			p.IsMain = name == "main"
		}
		p.Imports = append(p.Imports, imps...)
		calls, has, err := initCalls(f)
		if err == nil && has {
			p.HasInit = true
			p.InitCalls = append(p.InitCalls, calls...)
		}
	}
	var pkgs []*Package
	for _, p := range byDir {
		p.HandLOC = p.LOC - p.GenLOC
		p.Imports = uniqSorted(p.Imports)
		// Test imports that are also non-test imports are not test-only.
		nonTest := map[string]bool{}
		for _, ip := range p.Imports {
			nonTest[ip] = true
		}
		var t []string
		for _, ip := range uniqSorted(p.TestImports) {
			if !nonTest[ip] {
				t = append(t, ip)
			}
		}
		p.TestImports = t
		p.InitCalls = uniqSorted(p.InitCalls)
		pkgs = append(pkgs, p)
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].ImportPath < pkgs[j].ImportPath })
	// Direct-import approximation first; go list refines it below.
	for _, p := range pkgs {
		p.ClosureSource = "direct imports (go list failed)"
		var ext []string
		seen := map[string]bool{}
		for _, ip := range p.Imports {
			if strings.HasPrefix(ip, m.Path+"/") || ip == m.Path {
				continue
			}
			if !strings.Contains(strings.SplitN(ip, "/", 2)[0], ".") {
				continue // standard library
			}
			modPath := guessModule(ip, m.Dir)
			if !seen[modPath] {
				seen[modPath] = true
				ext = append(ext, modPath)
			}
			if isProtocol(ip) && !isProtocol(p.ImportPath) {
				p.ImportsProtocol = true
			}
		}
		sort.Strings(ext)
		p.ExternalModules = ext
	}
	return pkgs, nil
}

// goListPkg is the subset of `go list -json` we consume.
type goListPkg struct {
	ImportPath string
	Name       string
	Deps       []string
	Module     *struct{ Path string }
	Incomplete bool
}

// enrichWithGoList fills transitive closure facts from `go list -json -deps ./...`
// run in the subject module. Returns a warning string if it could not.
func enrichWithGoList(m moduleInfo, pkgs []*Package) string {
	all, err := goList(m.Dir, "-deps", "./...")
	if err != nil {
		return fmt.Sprintf("%s: go list failed (%v); closure figures are direct-import approximations", m.Path, firstLine(err.Error()))
	}
	modOf := map[string]string{}
	depsOf := map[string][]string{}
	for _, p := range all {
		if p.Module != nil {
			modOf[p.ImportPath] = p.Module.Path
		}
		depsOf[p.ImportPath] = p.Deps
	}
	for _, p := range pkgs {
		deps, ok := depsOf[p.ImportPath]
		if !ok {
			continue
		}
		p.ClosureSource = "go list"
		internal := 0
		mods := map[string]bool{}
		proto := false
		// A protocol package's own subpackages do not make it "protocol-importing";
		// the column is about reaching the wire contract from outside it.
		selfProto := isProtocol(p.ImportPath)
		for _, d := range deps {
			if d == p.ImportPath {
				continue
			}
			if isProtocol(d) && !selfProto {
				proto = true
			}
			mp, isMod := modOf[d]
			if !isMod {
				continue // standard library
			}
			if mp == m.Path {
				internal++
				continue
			}
			mods[mp] = true
		}
		for _, d := range p.Imports {
			if isProtocol(d) && !selfProto {
				proto = true
			}
		}
		p.InternalClosure = internal
		p.ImportsProtocol = proto
		p.ExternalModules = p.ExternalModules[:0]
		for mp := range mods {
			p.ExternalModules = append(p.ExternalModules, mp)
		}
		sort.Strings(p.ExternalModules)
	}
	return ""
}

// reachableFromMains returns the import paths reachable from every shipped
// `package main` in module m — codegen drivers (…/gen) and examples excluded.
func reachableFromMains(m moduleInfo) (map[string]bool, error) {
	pkgs, err := goList(m.Dir, "./...")
	if err != nil {
		return nil, fmt.Errorf("go list: %s", firstLine(err.Error()))
	}
	var mains []string
	for _, p := range pkgs {
		// Codegen drivers and examples are main packages nobody ships.
		if p.Name == "main" && !strings.HasSuffix(p.ImportPath, "/gen") && !strings.Contains(p.ImportPath, "/examples") {
			mains = append(mains, p.ImportPath)
		}
	}
	reach := map[string]bool{}
	if len(mains) == 0 {
		return reach, nil
	}
	deps, err := goList(m.Dir, append([]string{"-deps"}, mains...)...)
	if err != nil {
		return nil, fmt.Errorf("go list -deps: %s", firstLine(err.Error()))
	}
	for _, p := range deps {
		reach[p.ImportPath] = true
	}
	return reach, nil
}

func goList(dir string, args ...string) ([]goListPkg, error) {
	cmd := exec.Command("go", append([]string{"list", "-e", "-json=ImportPath,Name,Deps,Module,Incomplete"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	var pkgs []goListPkg
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var p goListPkg
		if err := dec.Decode(&p); err != nil {
			return nil, err
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

// guessModule maps an import path to a module path using the module's go.mod
// require lines (longest prefix wins). Used only when go list is unavailable.
func guessModule(ip, modDir string) string {
	b, err := os.ReadFile(filepath.Join(modDir, "go.mod"))
	if err != nil {
		return ip
	}
	best := ""
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) < 2 || !strings.Contains(f[0], ".") || !strings.HasPrefix(f[1], "v") {
			continue
		}
		if (ip == f[0] || strings.HasPrefix(ip, f[0]+"/")) && len(f[0]) > len(best) {
			best = f[0]
		}
	}
	if best == "" {
		// Fall back to the first three path elements, the common shape of a module path.
		parts := strings.Split(ip, "/")
		if len(parts) > 3 {
			parts = parts[:3]
		}
		return strings.Join(parts, "/")
	}
	return best
}

func isProtocol(ip string) bool {
	for _, pfx := range protocolPrefixes {
		if ip == pfx || strings.HasPrefix(ip, pfx+"/") {
			return true
		}
	}
	return false
}

func uniqSorted(s []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range s {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "inventory: "+format+"\n", args...)
	os.Exit(1)
}

// shortMod abbreviates a module path for table cells.
func shortMod(p string) string {
	p = strings.TrimPrefix(p, "github.com/fil-forge/")
	return strings.TrimPrefix(p, "github.com/")
}

func render(subjects, importers []moduleInfo, rows []*Package, warnings []string, noReach bool) string {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }
	w("# Package inventory\n\n")
	w("Generated by `tools/inventory` — do not edit by hand; re-run it. Command line (from `tools/inventory`):\n\n```\nGOWORK=off go run . %s\n```\n\n", strings.Join(os.Args[1:], " \\\n  "))
	w("Checkouts scanned (module, directory, `git rev-parse --short HEAD`):\n\n| role | module | dir | commit |\n|---|---|---|---|\n")
	subj := map[string]bool{}
	for _, m := range subjects {
		subj[m.Dir] = true
		w("| subject | `%s` | `%s` | `%s` |\n", m.Path, m.Dir, m.SHA)
	}
	for _, m := range importers {
		if !subj[m.Dir] {
			w("| consumer | `%s` | `%s` | `%s` |\n", m.Path, m.Dir, m.SHA)
		}
	}
	w("\nColumns: **LOC** non-test lines (in parentheses: hand-written, i.e. excluding `*_gen*.go` / generated files); **importers** consumer modules and the number of non-test files in each importing the package (test-only importers in italics); **closure** transitive same-module packages / distinct external modules in the full transitive closure (standard library excluded; source noted per module); **proto** the closure includes `libforge/commands/**` or `blobindex/**`; **init** callees inside `func init()`; **reach** whether a shipped binary (a non-`gen` `package main` in any scanned module) links the package.\n\n")
	if len(warnings) > 0 {
		w("Warnings:\n\n")
		for _, x := range warnings {
			w("- %s\n", x)
		}
		w("\n")
	}
	if noReach {
		w("Reachability analysis was skipped (`-no-reach`).\n\n")
	}
	for _, m := range subjects {
		w("## `%s` (`%s`)\n\n", m.Path, m.SHA)
		var mine []*Package
		for _, p := range rows {
			if p.Module == m.Path {
				mine = append(mine, p)
			}
		}
		// Roll-up per top-level directory.
		type agg struct{ loc, hand, files, pkgs int }
		top := map[string]*agg{}
		var order []string
		for _, p := range mine {
			k := strings.SplitN(p.Rel, "/", 2)[0]
			if k == "." {
				k = "(root)"
			}
			a := top[k]
			if a == nil {
				a = &agg{}
				top[k] = a
				order = append(order, k)
			}
			a.loc += p.LOC
			a.hand += p.HandLOC
			a.files += p.Files
			a.pkgs++
		}
		sort.Strings(order)
		total := &agg{}
		w("### Roll-up by top-level directory\n\n| dir | packages | non-test files | LOC | hand-written LOC |\n|---|---:|---:|---:|---:|\n")
		for _, k := range order {
			a := top[k]
			total.loc += a.loc
			total.hand += a.hand
			total.files += a.files
			total.pkgs += a.pkgs
			w("| `%s` | %d | %d | %d | %d |\n", k, a.pkgs, a.files, a.loc, a.hand)
		}
		w("| **total** | %d | %d | %d | %d |\n\n", total.pkgs, total.files, total.loc, total.hand)
		w("### Packages\n\n| package | LOC (hand) | importers | closure int/ext | proto | init | reach |\n|---|---:|---|---:|:---:|---|---|\n")
		for _, p := range mine {
			var imps []string
			for _, mod := range sortedKeys(p.Importers) {
				imps = append(imps, fmt.Sprintf("%s:%d", shortMod(mod), p.Importers[mod]))
			}
			for _, mod := range sortedKeys(p.TestImporters) {
				imps = append(imps, fmt.Sprintf("*%s:%d*", shortMod(mod), p.TestImporters[mod]))
			}
			impCell := strings.Join(imps, ", ")
			if impCell == "" {
				impCell = "—"
			}
			initCell := "—"
			if p.HasInit {
				calls := p.InitCalls
				more := ""
				if len(calls) > 6 {
					more = fmt.Sprintf(" … (%d more)", len(calls)-6)
					calls = calls[:6]
				}
				initCell = "`" + strings.Join(calls, "`, `") + "`" + more
				if len(p.InitCalls) == 0 {
					initCell = "yes"
				}
			}
			proto := ""
			if p.ImportsProtocol {
				proto = "yes"
			}
			name := "`" + p.Rel + "`"
			if p.IsGen {
				name += " (codegen driver)"
			} else if p.IsMain {
				name += " (main)"
			}
			w("| %s | %d (%d) | %s | %d / %d | %s | %s | %s |\n", name, p.LOC, p.HandLOC, impCell, p.InternalClosure, len(p.ExternalModules), proto, initCell, p.Reachability)
		}
		w("\n")
		// Closure detail for the packages the decision hangs on: list modules for the largest closures.
		w("### External module closures (largest first)\n\n| package | external modules | modules |\n|---|---:|---|\n")
		sorted := append([]*Package{}, mine...)
		sort.Slice(sorted, func(i, j int) bool {
			if len(sorted[i].ExternalModules) != len(sorted[j].ExternalModules) {
				return len(sorted[i].ExternalModules) > len(sorted[j].ExternalModules)
			}
			return sorted[i].ImportPath < sorted[j].ImportPath
		})
		for i, p := range sorted {
			if i >= 12 || p.IsGen {
				continue
			}
			mods := p.ExternalModules
			cell := strings.Join(mapShort(mods), ", ")
			if len(mods) > 12 {
				cell = strings.Join(mapShort(mods[:12]), ", ") + fmt.Sprintf(", … (%d more)", len(mods)-12)
			}
			w("| `%s` | %d | %s |\n", p.Rel, len(mods), cell)
		}
		w("\n")
	}
	return b.String()
}

func mapShort(mods []string) []string {
	out := make([]string, len(mods))
	for i, m := range mods {
		out[i] = shortMod(m)
	}
	return out
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
