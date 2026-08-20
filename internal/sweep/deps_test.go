package sweep

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lopster568/loadline/internal/corpus"
	"github.com/lopster568/loadline/internal/report"
)

// pythonListing is real stdout from
// `uvx --from mcp-server-fetch python -c <pythonDepListing>`, trimmed to the
// entries the assertions read. The mcp entry is the one the 2026-08-18 fetch
// row could not report.
const pythonListing = `{"prefix": "/home/oni/.cache/uv/archive-v0/zgVBHfO2se1vLBdPHhJVB", "packages": [["anyio", "4.14.2"], ["mcp", "1.29.0"], ["mcp-server-fetch", "2026.8.18"], ["pydantic", "2.13.4"]]}`

// nodeListing is real stdout from
// `npx -y -p @modelcontextprotocol/server-filesystem node -e <nodeDepListing>`,
// trimmed the same way.
const nodeListing = `{"prefix":"C:\\Users\\Roshs\\AppData\\Local\\npm-cache\\_npx\\a3241bba59c344f5","packages":[["@modelcontextprotocol/sdk","1.30.0"],["@modelcontextprotocol/server-filesystem","2026.7.10"],["zod","4.4.3"]]}`

func TestParseDepListingPython(t *testing.T) {
	env, pkgs, err := parseDepListing([]byte(pythonListing + "\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if env != "/home/oni/.cache/uv/archive-v0/zgVBHfO2se1vLBdPHhJVB" {
		t.Errorf("env = %q, want the resolved environment prefix", env)
	}
	if len(pkgs) != 4 {
		t.Fatalf("packages = %d, want 4", len(pkgs))
	}
	sdk := findDep(pkgs, pypiSDK)
	if sdk == nil || sdk.Version != "1.29.0" {
		t.Errorf("sdk = %+v, want mcp 1.29.0", sdk)
	}
}

func TestParseDepListingNode(t *testing.T) {
	env, pkgs, err := parseDepListing([]byte(nodeListing + "\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !strings.Contains(env, "_npx") {
		t.Errorf("env = %q, want the npx cache directory", env)
	}
	sdk := findDep(pkgs, npmSDK)
	if sdk == nil || sdk.Version != "1.30.0" {
		t.Errorf("sdk = %+v, want @modelcontextprotocol/sdk 1.30.0", sdk)
	}
}

// A package runner writing its own notices onto the child's stdout must not
// cost the row its dependency record.
func TestParseDepListingIgnoresRunnerNotices(t *testing.T) {
	noisy := strings.Join([]string{
		"npm notice",
		"npm notice New major version of npm available! 10.9.3 -> 12.0.2",
		"CMD.EXE was started with the above path as the current directory.",
		"{ not json at all",
		nodeListing,
		"",
	}, "\n")
	_, pkgs, err := parseDepListing([]byte(noisy))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if findDep(pkgs, npmSDK) == nil {
		t.Error("the listing line was lost behind the runner's notices")
	}
}

func TestParseDepListingRejectsAbsentAndMalformed(t *testing.T) {
	if _, _, err := parseDepListing([]byte("npm notice\nnothing here\n")); err == nil {
		t.Error("output carrying no listing must be an error, not an empty dependency set")
	}
	malformed := `{"prefix": "/tmp/env", "packages": [["mcp"]]}`
	if _, _, err := parseDepListing([]byte(malformed)); err == nil {
		t.Error("a listing entry missing its version must be an error, not a package with an empty version")
	}
}

func TestFindDepNormalizesNames(t *testing.T) {
	pkgs := []report.DepPackage{
		{Name: "MCP", Version: "1.29.0"},
		{Name: "mcp_server_fetch", Version: "2026.8.18"},
	}
	if got := findDep(pkgs, "mcp"); got == nil || got.Version != "1.29.0" {
		t.Errorf("findDep(mcp) = %+v, PyPI names compare case-insensitively", got)
	}
	if got := findDep(pkgs, "mcp-server-fetch"); got == nil || got.Version != "2026.8.18" {
		t.Errorf("findDep(mcp-server-fetch) = %+v, PyPI folds - _ and . together", got)
	}
	if got := findDep(pkgs, "starlette"); got != nil {
		t.Errorf("findDep of an absent package = %+v, want nil", got)
	}
}

// The listing must resolve the launch's own requirement set. A --with
// constraint left out here would key a different cached environment and report
// versions the server never imported.
func TestPypiDepsPlanCarriesTheLaunchRequirements(t *testing.T) {
	s := corpus.Server{
		ID: "constrained", Transport: []string{"stdio"},
		Auth: corpus.Auth{Required: ptrBool(false)},
		Package: map[string]corpus.Pkg{"stdio": {
			Type: "pypi", Name: "mcp-server-fetch", Version: "2026.7.10", With: []string{"mcp<2"},
		}},
	}
	l, err := resolveLaunch(s, "/scratch")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	want := []string{"uvx", "--from", "mcp-server-fetch==2026.7.10", "--with", "mcp<2", "python", "-c", pythonDepListing}
	if fmt.Sprint(l.deps.argv) != fmt.Sprint(want) {
		t.Errorf("deps argv = %v, want %v", l.deps.argv, want)
	}
	if l.deps.method != methodUvx || l.deps.sdk != pypiSDK {
		t.Errorf("deps plan = %+v, want the uvx method and the PyPI SDK name", l.deps)
	}
}

func TestNpmDepsPlanUsesTheSameSpecAsTheLaunch(t *testing.T) {
	s := corpus.Server{
		ID: "fs", Transport: []string{"stdio"},
		Auth: corpus.Auth{Required: ptrBool(false)},
		Package: map[string]corpus.Pkg{"stdio": {
			Type: "npm", Name: "@modelcontextprotocol/server-filesystem", Args: []string{scratchToken},
		}},
	}
	l, err := resolveLaunch(s, "/scratch")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	want := []string{"npx", "-y", "-p", "@modelcontextprotocol/server-filesystem", "node", "-e", nodeDepListing}
	if fmt.Sprint(l.deps.argv) != fmt.Sprint(want) {
		t.Errorf("deps argv = %v, want %v", l.deps.argv, want)
	}
	if l.deps.sdk != npmSDK {
		t.Errorf("deps sdk = %q, want %q", l.deps.sdk, npmSDK)
	}
}

// An acquisition kind that resolves nothing locally records why, rather than a
// bare empty field a reader cannot tell apart from a listing that failed.
func TestNonResolvingAcquisitionsRecordAReason(t *testing.T) {
	cases := []struct {
		name   string
		server corpus.Server
		method string
	}{
		{
			name: "docker",
			server: corpus.Server{
				ID: "pg", Transport: []string{"stdio"}, Auth: corpus.Auth{Required: ptrBool(false)},
				Package: map[string]corpus.Pkg{"stdio": {Type: "docker", Image: "example/server:1.2.3"}},
			},
			method: methodImage,
		},
		{
			name: "remote",
			server: corpus.Server{
				ID: "remote", Transport: []string{"remote"}, Auth: corpus.Auth{Required: ptrBool(false)},
				Endpoint: ptrString("https://example.test/mcp"),
			},
			method: methodNotApplicable,
		},
		{
			name: "binary",
			server: corpus.Server{
				ID: "bin", Transport: []string{"stdio"}, Auth: corpus.Auth{Required: ptrBool(false)},
				Package: map[string]corpus.Pkg{"stdio": {Type: "binary", Command: "some-server"}},
			},
			method: methodNotApplicable,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l, err := resolveLaunch(tc.server, "/scratch")
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if len(l.deps.argv) != 0 {
				t.Errorf("deps argv = %v, want nothing to run", l.deps.argv)
			}
			rec := resolveDeps(t.Context(), l.deps, time.Second)
			if rec.Method != tc.method {
				t.Errorf("method = %q, want %q", rec.Method, tc.method)
			}
			if rec.Note == "" {
				t.Error("an acquisition with no listing must say why")
			}
			if rec.SDK != nil || rec.Packages != nil || rec.Error != "" {
				t.Errorf("record = %+v, want no listing and no error", rec)
			}
		})
	}
}

// A listing that will not run is data on the row. The acquisition it observes
// is measured either way.
func TestResolveDepsRecordsAFailedListingRatherThanFailing(t *testing.T) {
	p := depsPlan{
		method: methodUvx,
		argv:   []string{"loadline-no-such-resolver", "--list"},
		sdk:    pypiSDK,
	}
	rec := resolveDeps(t.Context(), p, time.Second)
	if rec.Error == "" {
		t.Fatal("a listing that could not run must record the error")
	}
	if rec.SDK != nil || rec.Packages != nil {
		t.Errorf("record = %+v, want no dependency set behind a failed listing", rec)
	}
	if fmt.Sprint(rec.Command) != fmt.Sprint(p.argv) {
		t.Errorf("command = %v, want the argv that failed so it can be rerun", rec.Command)
	}
}

// A listing that will not finish is bounded by its own budget and spends none
// of the budget it was handed. resolveDeps is instrumentation on the
// acquisition, and sweep.go starts the server budget only after it returns, so
// the untouched window asserted here is the window the dial that follows gets:
// the same one the row would have had if no listing existed at all.
func TestResolveDepsSpendsItsOwnBudgetAndRecordsTheTimeout(t *testing.T) {
	p := depsPlan{method: methodUvx, argv: []string{os.Args[0], fakeSleepFlag}, sdk: pypiSDK}

	// Stands in for the sweep context runOne hands the listing.
	parent, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	rec := resolveDeps(parent, p, 250*time.Millisecond)
	elapsed := time.Since(start)

	if rec.Error == "" {
		t.Error("a listing that ran out its own budget must be recorded as data on the row")
	}
	if rec.SDK != nil || rec.Packages != nil {
		t.Errorf("record = %+v, want no dependency set behind a listing that never finished", rec)
	}
	if elapsed > 5*time.Second {
		t.Errorf("a 250ms listing budget took %s: the listing is running on someone else's deadline", elapsed)
	}
	if err := parent.Err(); err != nil {
		t.Errorf("handed-in context = %v after the listing, want it untouched", err)
	}
	deadline, ok := parent.Deadline()
	if !ok || time.Until(deadline) < 25*time.Second {
		t.Error("the listing spent the budget it was handed; the measurement's own window must start unspent")
	}
}
