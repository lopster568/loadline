package sweep

import (
	"fmt"
	"os"
	"strings"

	"github.com/lopster568/loadline/internal/corpus"
)

// launch is a resolved acquisition plan for one server.
type launch struct {
	transport string
	source    string
	command   string
	args      []string
	// recorded is the argument vector before scratch substitution, so the run
	// record stays reproducible instead of citing one run's temp directory.
	recorded []string
	endpoint string
	pinned   bool
}

// scratchToken is substituted with the per-run scratch directory in launch
// arguments that need a filesystem root. A corpus entry writes it into its
// package args; nothing about which server needs one is known to this file.
const scratchToken = "{{SCRATCH}}"

// resolveLaunch picks a transport and builds the acquisition plan purely from
// the corpus entry: no server is special-cased here, so a launch plan is always
// reviewable in servers.yaml rather than half in Go. stdio wins over remote
// when both are declared, because a package version is a stronger pin than a
// self-reported serverInfo (methodology 1.1). An entry whose package block is
// still a "TODO verify at onboarding" stub stays terminally unresolved.
func resolveLaunch(s corpus.Server, scratch string) (launch, error) {
	if s.HasTransport("stdio") || len(s.Package) > 0 {
		if l, ok := stdioLaunch(s); ok {
			l.recorded = append([]string(nil), l.args...)
			l.args = substitute(l.args, scratch)
			return l, nil
		}
	}
	if s.Endpoint != nil && *s.Endpoint != "" {
		if s.HasTransport("remote") || len(s.Transport) == 0 {
			return launch{
				transport: "remote",
				source:    "endpoint:" + *s.Endpoint,
				endpoint:  *s.Endpoint,
			}, nil
		}
	}
	if s.HasTransport("sse") && !s.HasTransport("remote") && !s.HasTransport("stdio") {
		return launch{}, fmt.Errorf("only the deprecated HTTP+SSE transport is declared, which the harness does not support")
	}
	return launch{}, fmt.Errorf("acquisition spec unresolved in the corpus")
}

func stdioLaunch(s corpus.Server) (launch, bool) {
	pkg, ok := s.Package["stdio"]
	if !ok {
		return launch{}, false
	}
	l := launch{transport: "stdio"}
	switch pkg.Type {
	case "npm":
		if pkg.Name == "" {
			return launch{}, false
		}
		spec := pkg.Name
		if pkg.Version != "" {
			spec += "@" + pkg.Version
		}
		l.source = "npm:" + spec
		l.command = "npx"
		l.args = append([]string{"-y", spec}, pkg.Args...)
		l.pinned = pkg.Version != ""
	case "pypi", "uv", "uvx":
		if pkg.Name == "" {
			return launch{}, false
		}
		spec := pkg.Name
		if pkg.Version != "" {
			spec += "==" + pkg.Version
		}
		l.source = "pypi:" + spec
		l.command = "uvx"
		for _, w := range pkg.With {
			l.args = append(l.args, "--with", w)
			l.source += " --with " + w
		}
		l.args = append(l.args, spec)
		l.args = append(l.args, pkg.Args...)
		l.pinned = pkg.Version != ""
	case "docker":
		if pkg.Image == "" {
			return launch{}, false
		}
		l.source = "docker:" + pkg.Image
		l.command = "docker"
		l.args = append([]string{"run", "--rm", "-i", pkg.Image}, pkg.Args...)
		l.pinned = strings.Contains(pkg.Image, "@sha256:")
	case "binary":
		if pkg.Command == "" {
			return launch{}, false
		}
		l.source = "binary:" + pkg.Command
		l.command = pkg.Command
		l.args = append([]string(nil), pkg.Args...)
		l.pinned = pkg.Version != ""
	default:
		return launch{}, false
	}
	return l, true
}

func substitute(args []string, scratch string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = strings.ReplaceAll(a, scratchToken, scratch)
	}
	return out
}

// credential resolves the corpus credential declaration into an env var name
// and value. An empty name with a nil error means no credential is needed.
func credential(s corpus.Server) (name, value string, err error) {
	if !s.AuthRequired() {
		if s.Auth.TokenEnv != nil && *s.Auth.TokenEnv != "" {
			return *s.Auth.TokenEnv, os.Getenv(*s.Auth.TokenEnv), nil
		}
		return "", "", nil
	}
	if s.Auth.TokenEnv == nil || *s.Auth.TokenEnv == "" {
		return "", "", fmt.Errorf("credential required but the corpus declares no env var convention")
	}
	name = *s.Auth.TokenEnv
	value = os.Getenv(name)
	if value == "" {
		return name, "", fmt.Errorf("credential required but %s is not set", name)
	}
	return name, value, nil
}
