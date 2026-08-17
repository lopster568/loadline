// Package corpus reads the ratified server list.
package corpus

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// File is the top level of servers.yaml.
type File struct {
	Servers  []Server   `yaml:"servers"`
	Excluded []Excluded `yaml:"excluded"`
}

// Server is one corpus entry.
type Server struct {
	ID             string           `yaml:"id"`
	Name           string           `yaml:"name"`
	Category       string           `yaml:"category"`
	MaintainerType string           `yaml:"maintainer_type"`
	Repo           *string          `yaml:"repo"`
	Endpoint       *string          `yaml:"endpoint"`
	Transport      []string         `yaml:"transport"`
	Auth           Auth             `yaml:"auth"`
	Package        map[string]Pkg   `yaml:"package"`
	Flags          []map[string]any `yaml:"flags"`
}

// Auth is the credential declaration for a server.
type Auth struct {
	Required *bool   `yaml:"required"`
	TokenEnv *string `yaml:"token_env"`
}

// Pkg is one distribution channel. Args and Version are absent from the
// current servers.yaml but are read here so the corpus can carry a pin and a
// launch argument vector without a harness change.
type Pkg struct {
	Type    string   `yaml:"type"`
	Name    string   `yaml:"name"`
	Version string   `yaml:"version"`
	Image   string   `yaml:"image"`
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
	// With pins transitive dependencies a package leaves unbounded. It is
	// recorded in the run record like any other part of the pin, so a
	// constrained acquisition is never invisible in a published row.
	With []string `yaml:"with"`
}

// Excluded is one entry dropped under the partial-surface rule.
type Excluded struct {
	ID        string `yaml:"id"`
	Name      string `yaml:"name"`
	Reason    string `yaml:"reason"`
	Rationale string `yaml:"rationale"`
}

// AuthRequired reports whether the entry declares a credential requirement.
// A null value in the corpus means unresearched, which is treated as required
// so the harness never silently publishes an unauthenticated partial surface.
func (s Server) AuthRequired() bool {
	if s.Auth.Required == nil {
		return true
	}
	return *s.Auth.Required
}

// HasTransport reports whether the entry declares the named transport.
func (s Server) HasTransport(name string) bool {
	for _, t := range s.Transport {
		if t == name {
			return true
		}
	}
	return false
}

// Notes flattens the freeform flags block into readable strings. Keys within
// one flag entry are sorted, because a flag with several keys would otherwise
// emit corpus_notes in Go's randomized map order and make two runs of an
// unchanged corpus produce different published rows.
func (s Server) Notes() []string {
	var out []string
	for _, f := range s.Flags {
		keys := make([]string, 0, len(f))
		for k := range f {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out = append(out, fmt.Sprintf("%s: %v", k, f[k]))
		}
	}
	return out
}

// Load reads and parses a servers.yaml file.
func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for i, s := range f.Servers {
		if s.ID == "" {
			return nil, fmt.Errorf("%s: server %d has no id", path, i)
		}
	}
	return &f, nil
}
