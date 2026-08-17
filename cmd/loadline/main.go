// Command loadline runs the Tier 1 static sweep defined in
// docs/methodology-v0.md and writes the run artifacts the site reads.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lopster568/loadline/internal/corpus"
	"github.com/lopster568/loadline/internal/report"
	"github.com/lopster568/loadline/internal/sweep"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "scan":
		if err := runScan(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "loadline:", err)
			os.Exit(1)
		}
	case "version":
		fmt.Printf("loadline %s (methodology %s, schema %s)\n",
			report.HarnessVersion, report.MethodologyVersion, report.SchemaVersion)
	case "-h", "--help", "help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `loadline - Tier 1 MCP context-footprint measurement harness

Usage:
  loadline scan --servers servers.yaml [--only id1,id2] [--out data/]
  loadline version

scan flags:
  --servers path    corpus file (default servers.yaml)
  --out dir         output root; writes <dir>/runs/<date>/<id>.json and <dir>/latest.json (default data)
  --only ids        comma-separated server ids to sweep
  --skip ids        comma-separated server ids to omit
  --step-timeout d  per-step budget (default 60s)
  --server-timeout d overall per-server budget (default 4m)
  --claude-model id pinned Claude model for count_tokens
  --gemini-model id pinned Gemini model for countTokens
  --dry-run         resolve the corpus and print the plan without contacting servers
`)
}

func runScan(argv []string) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	serversPath := fs.String("servers", "servers.yaml", "corpus file")
	outDir := fs.String("out", "data", "output root")
	only := fs.String("only", "", "comma-separated server ids to sweep")
	skip := fs.String("skip", "", "comma-separated server ids to omit")
	stepTimeout := fs.Duration("step-timeout", 60*time.Second, "per-step budget")
	serverTimeout := fs.Duration("server-timeout", 4*time.Minute, "overall per-server budget")
	claudeModel := fs.String("claude-model", "", "pinned Claude model")
	geminiModel := fs.String("gemini-model", "", "pinned Gemini model")
	dryRun := fs.Bool("dry-run", false, "resolve the corpus without contacting servers")
	if err := fs.Parse(argv); err != nil {
		return err
	}

	file, err := corpus.Load(*serversPath)
	if err != nil {
		return err
	}
	selected := filter(file.Servers, splitIDs(*only), splitIDs(*skip))
	if len(selected) == 0 {
		return fmt.Errorf("no servers selected from %s", *serversPath)
	}

	if *dryRun {
		for _, s := range selected {
			fmt.Printf("%-12s %-22s transport=%v auth_required=%v\n",
				s.ID, s.Category, s.Transport, s.AuthRequired())
		}
		return nil
	}

	scratch, err := os.MkdirTemp("", "loadline-scratch-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)

	cfg := sweep.DefaultConfig()
	cfg.StepTimeout = *stepTimeout
	cfg.ServerTimeout = *serverTimeout
	cfg.ScratchDir = scratch
	if *claudeModel != "" {
		cfg.ClaudeModel = *claudeModel
	}
	if *geminiModel != "" {
		cfg.GeminiModel = *geminiModel
	}
	cfg.Logf = func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runner := sweep.NewRunner(cfg)
	doc := runner.Run(ctx, selected)

	// An aborted sweep publishes the rows that completed plus a run-level
	// marker, and nothing for the servers it never reached. Methodology 7 only
	// classifies server-attributable failures, so writing a "timeout" row for a
	// server the operator interrupted would publish a claim about that server
	// the run never earned. A completed row is still a real measurement, so it
	// is kept; if nothing completed there is nothing honest to write, and the
	// previous artifacts are left untouched.
	if doc.Run.Aborted {
		fmt.Fprintln(os.Stderr, "loadline: sweep aborted; publishing only the servers that completed")
		if len(doc.Servers) == 0 {
			return fmt.Errorf("sweep aborted before any server completed; no report written")
		}
	}

	written, err := report.Write(*outDir, doc)
	if err != nil {
		return err
	}

	for _, s := range doc.Servers {
		naive := -1
		if s.Modes != nil {
			naive = s.Modes.Naive.Tokens
		}
		grade, retr := "-", "-"
		if s.Hygiene != nil {
			grade = fmt.Sprintf("%s(%.0f)", s.Hygiene.Grade, s.Hygiene.Score)
		}
		if s.Retrievability != nil {
			retr = fmt.Sprintf("%.2f", s.Retrievability.Top3Fraction)
		}
		line := fmt.Sprintf("%-12s %-16s tools=%-4d naive=%-7s hygiene=%-7s retr@3=%-5s rev=%s",
			s.ID, s.Status, s.ToolCount, naiveField(naive), grade, retr, s.ProtocolRevision)
		if s.Error != "" {
			line += "  " + firstLine(s.Error)
		}
		fmt.Println(line)
	}
	fmt.Printf("wrote %d artifacts under %s\n", len(written), *outDir)
	return nil
}

func filter(all []corpus.Server, only, skip map[string]bool) []corpus.Server {
	var out []corpus.Server
	for _, s := range all {
		if len(only) > 0 && !only[s.ID] {
			continue
		}
		if skip[s.ID] {
			continue
		}
		out = append(out, s)
	}
	return out
}

func splitIDs(csv string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(csv, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out[part] = true
		}
	}
	return out
}

// naiveField renders the console naive figure, printing "-" rather than 0 for
// a row with no mode block so the terminal line cannot be read as a measurement.
func naiveField(n int) string {
	if n < 0 {
		return "-"
	}
	return strconv.Itoa(n)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	if len(s) > 160 {
		return s[:160] + "..."
	}
	return s
}
