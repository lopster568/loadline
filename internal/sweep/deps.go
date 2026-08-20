package sweep

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/lopster568/loadline/internal/report"
)

// Methods a dependency listing can come from. The value is recorded on the row
// so a reader knows which mechanism produced the list before reading the list.
const (
	methodUvx           = "uvx_importlib_metadata"
	methodNpx           = "npx_node_modules_walk"
	methodImage         = "container_image"
	methodNotApplicable = "not_applicable"
)

// The MCP SDK package on each registry. It is lifted out of the resolved set
// into its own field because a server that dies at import usually died on this
// package, and the reader of that row should not have to scan a 130-entry list
// to find the version that produced the failure.
const (
	pypiSDK = "mcp"
	npmSDK  = "@modelcontextprotocol/sdk"
)

// pythonDepListing prints every distribution in the environment the uvx tool
// actually runs from, read with importlib.metadata from inside that
// environment rather than from a second resolve outside it. sys.prefix is
// printed alongside so the environment is identifiable: it is the directory a
// server traceback cites, so a reader can confirm the list and the failure came
// from the same place.
//
// The script is one line because it is passed through package runners that
// re-quote their arguments, and a newline does not survive every one of them.
const pythonDepListing = `import json,sys,importlib.metadata as m; ps=sorted({(d.metadata["Name"], d.version) for d in m.distributions() if d.metadata["Name"] and d.version}); print(json.dumps({"prefix": sys.prefix, "packages": [[n,v] for n,v in ps]}))`

// nodeDepListing walks the node_modules tree npx installed for this package
// spec and prints every package.json name and version in it. The tree is
// located from PATH: npx puts <prefix>/node_modules/.bin at the front of the
// child environment, which is the only handle a child process gets on the
// cache directory npx chose. When that handle is absent the script exits
// non-zero rather than falling back to the working directory: a walk of some
// unrelated node_modules tree, or of a directory that has none, would publish
// either the wrong versions or an empty resolved set under a null sdk, and
// both read as answers rather than as the failure they are. Same one-line
// constraint as pythonDepListing.
const nodeDepListing = `const fs=require('fs'),path=require('path'); const dir=(process.env.PATH||'').split(path.delimiter).find(p=>p.indexOf('_npx')!==-1); if(!dir){console.error('no npx cache directory on PATH: this listing would have walked an unrelated node_modules tree');process.exit(1)} const root=path.resolve(dir,'..','..'); const out=[],seen={}; function names(d){try{return fs.readdirSync(d)}catch(e){return[]}} function readPkg(p){try{const j=JSON.parse(fs.readFileSync(path.join(p,'package.json'),'utf8'));if(j.name&&j.version){const k=j.name+'@'+j.version;if(!seen[k]){seen[k]=1;out.push([j.name,j.version])}}}catch(e){} scan(path.join(p,'node_modules'))} function scan(nm){for(const n of names(nm)){if(n[0]==='.')continue;const p=path.join(nm,n);if(n[0]==='@'){for(const c of names(p)){if(c[0]==='.')continue;readPkg(path.join(p,c))}continue}readPkg(p)}} scan(path.join(root,'node_modules')); out.sort(); console.log(JSON.stringify({prefix:root,packages:out}));`

// depsPlan is how one acquisition's resolved dependency set is to be read. An
// empty argv means there is nothing to run, in which case note says why.
type depsPlan struct {
	method string
	argv   []string
	sdk    string
	note   string
}

// resolveDeps runs the listing and turns whatever happened into a record. It
// has no failure path that reaches the caller: a listing that will not run, or
// runs and cannot be parsed, publishes the error in the field. The dependency
// set is instrumentation on the acquisition, and instrumentation must not be
// able to fail a measurement it only observes.
func resolveDeps(ctx context.Context, p depsPlan, timeout time.Duration) *report.ResolvedDeps {
	out := &report.ResolvedDeps{Method: p.method, Note: p.note}
	if len(p.argv) == 0 {
		return out
	}
	out.Command = append([]string(nil), p.argv...)

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, p.argv[0], p.argv[1:]...)
	// The same environment the launch gets, so the listing reads the same
	// resolver caches the server process will be started from.
	cmd.Env = inheritedEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		out.Error = err.Error()
		if s := strings.TrimSpace(stderr.String()); s != "" {
			out.Error += ": " + firstLines(s, 3)
		}
		return out
	}

	env, pkgs, err := parseDepListing(stdout.Bytes())
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.Env = env
	out.Packages = pkgs
	out.SDK = findDep(pkgs, p.sdk)
	return out
}

// depListing is the JSON both helper scripts print.
type depListing struct {
	Prefix   string     `json:"prefix"`
	Packages [][]string `json:"packages"`
}

// parseDepListing reads the last line of stdout that parses as a listing.
// Package runners write their own notices into the child's output (npm update
// notices, interop warnings on WSL), and a notice that lands on stdout rather
// than stderr must not cost the row its dependency record over a line the
// harness never asked for.
func parseDepListing(stdout []byte) (string, []report.DepPackage, error) {
	lines := strings.Split(string(stdout), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var listing depListing
		if err := json.Unmarshal([]byte(line), &listing); err != nil {
			continue
		}
		if listing.Prefix == "" && listing.Packages == nil {
			continue
		}
		pkgs := make([]report.DepPackage, 0, len(listing.Packages))
		for _, pair := range listing.Packages {
			if len(pair) != 2 || pair[0] == "" || pair[1] == "" {
				return "", nil, fmt.Errorf("dependency listing carried a malformed entry: %v", pair)
			}
			pkgs = append(pkgs, report.DepPackage{Name: pair[0], Version: pair[1]})
		}
		return listing.Prefix, pkgs, nil
	}
	return "", nil, fmt.Errorf("no dependency listing found in %d lines of output", len(lines))
}

// findDep returns the named package from the resolved set, or nil when the set
// does not carry it. Names are compared under the PyPI normalization rule
// (lowercase, runs of separators folded to a hyphen), which npm names pass
// through unchanged.
func findDep(pkgs []report.DepPackage, name string) *report.DepPackage {
	if name == "" {
		return nil
	}
	want := normalizeDepName(name)
	for _, p := range pkgs {
		if normalizeDepName(p.Name) == want {
			hit := p
			return &hit
		}
	}
	return nil
}

func normalizeDepName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastSep := false
	for _, r := range lower {
		if r == '-' || r == '_' || r == '.' {
			if !lastSep {
				b.WriteRune('-')
			}
			lastSep = true
			continue
		}
		lastSep = false
		b.WriteRune(r)
	}
	return b.String()
}

// pypiDepsPlan lists the resolved environment uvx runs the tool from. The
// requirement set given here is the launch's requirement set: same package
// spec, same --with constraints, in the same order, because uv keys its cached
// environment on that set. A listing built from a different set would resolve
// its own environment and report versions the server never imported.
func pypiDepsPlan(spec string, with []string) depsPlan {
	argv := []string{"uvx", "--from", spec}
	for _, w := range with {
		argv = append(argv, "--with", w)
	}
	argv = append(argv, "python", "-c", pythonDepListing)
	return depsPlan{method: methodUvx, argv: argv, sdk: pypiSDK}
}

// npmDepsPlan lists the tree npx installed for this package spec. npx keys its
// cache directory on the package spec set alone, so `-p <spec> node` lands in
// the same directory `npx <spec>` runs the server from.
func npmDepsPlan(spec string) depsPlan {
	return depsPlan{
		method: methodNpx,
		argv:   []string{"npx", "-y", "-p", spec, "node", "-e", nodeDepListing},
		sdk:    npmSDK,
	}
}
