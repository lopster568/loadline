package canon

import (
	"fmt"
	"strings"
	"time"
)

// RefPolicy bounds $ref resolution. Methodology 2 rule 4 requires resolution to
// be bounded in depth, subschema count, and time.
type RefPolicy struct {
	MaxDepth      int
	MaxSubschemas int
	Budget        time.Duration
}

// DefaultRefPolicy is the v0 bound set.
func DefaultRefPolicy() RefPolicy {
	return RefPolicy{MaxDepth: 32, MaxSubschemas: 20000, Budget: 5 * time.Second}
}

// Flag names recorded against a tool whose schema could not be resolved
// cleanly. External is fatal for the tool under methodology 2 rule 3; the
// others leave the tool counted but marked.
const (
	FlagRecursiveRef = "recursive_ref_unresolved"
	FlagExternalRef  = "external_ref_unresolvable"
	FlagBoundHit     = "ref_resolution_bound_exceeded"
	FlagBadPointer   = "internal_ref_unresolvable"
)

type refOutcome struct {
	flags map[string]bool
}

func (o *refOutcome) flag(name string) {
	if o.flags == nil {
		o.flags = map[string]bool{}
	}
	o.flags[name] = true
}

func (o *refOutcome) has(name string) bool { return o.flags[name] }

func (o *refOutcome) list() []string {
	if len(o.flags) == 0 {
		return nil
	}
	order := []string{FlagExternalRef, FlagRecursiveRef, FlagBoundHit, FlagBadPointer}
	var out []string
	for _, n := range order {
		if o.flags[n] {
			out = append(out, n)
		}
	}
	return out
}

type refResolver struct {
	root     *Value
	policy   RefPolicy
	deadline time.Time
	visited  int
	out      *refOutcome
}

// ResolveRefs inlines same-document $refs in a schema per methodology 2 and
// returns the rewritten schema plus any flags raised. The input is not
// mutated.
func ResolveRefs(schema *Value, policy RefPolicy) (*Value, []string) {
	out := &refOutcome{}
	if schema == nil {
		return nil, nil
	}
	root := schema.Clone()
	r := &refResolver{root: root, policy: policy, deadline: time.Now().Add(policy.Budget), out: out}
	resolved := r.walk(root, 0, map[string]bool{})
	if !out.has(FlagRecursiveRef) && !out.has(FlagBoundHit) {
		resolved.Delete("$defs")
		resolved.Delete("definitions")
	}
	return resolved, out.list()
}

func (r *refResolver) walk(v *Value, depth int, active map[string]bool) *Value {
	if v == nil {
		return nil
	}
	r.visited++
	if r.visited > r.policy.MaxSubschemas || depth > r.policy.MaxDepth || time.Now().After(r.deadline) {
		r.out.flag(FlagBoundHit)
		return v
	}
	switch v.Kind {
	case KindArray:
		for i, e := range v.Arr {
			v.Arr[i] = r.walk(e, depth+1, active)
		}
		return v
	case KindObject:
		if ref, ok := v.Get("$ref"); ok && ref.Kind == KindString {
			return r.inline(v, ref.Str, depth, active)
		}
		for _, k := range v.Keys {
			v.Fields[k] = r.walk(v.Fields[k], depth+1, active)
		}
		return v
	default:
		return v
	}
}

func (r *refResolver) inline(node *Value, ref string, depth int, active map[string]bool) *Value {
	if !strings.HasPrefix(ref, "#") {
		// Methodology 2 rules 2 and 3: network and other external pointers are
		// never dereferenced and are not counted permissively.
		r.out.flag(FlagExternalRef)
		return node
	}
	if active[ref] {
		r.out.flag(FlagRecursiveRef)
		return node
	}
	target, err := resolvePointer(r.root, ref)
	if err != nil {
		r.out.flag(FlagBadPointer)
		return node
	}

	nested := map[string]bool{ref: true}
	for k := range active {
		nested[k] = true
	}
	expanded := r.walk(target.Clone(), depth+1, nested)

	// JSON Schema 2020-12 applies $ref alongside its siblings, so the target's
	// members are spliced in and any key the caller already set wins.
	merged := &Value{Kind: KindObject, Fields: map[string]*Value{}}
	if expanded.Kind == KindObject {
		for _, k := range expanded.Keys {
			merged.Set(k, expanded.Fields[k])
		}
	} else {
		return expanded
	}
	for _, k := range node.Keys {
		if k == "$ref" {
			continue
		}
		merged.Set(k, r.walk(node.Fields[k], depth+1, active))
	}
	return merged
}

func resolvePointer(root *Value, ref string) (*Value, error) {
	ptr := strings.TrimPrefix(ref, "#")
	if ptr == "" {
		return root, nil
	}
	if !strings.HasPrefix(ptr, "/") {
		return nil, fmt.Errorf("non-pointer fragment %q", ref)
	}
	cur := root
	for _, seg := range strings.Split(strings.TrimPrefix(ptr, "/"), "/") {
		seg = strings.ReplaceAll(seg, "~1", "/")
		seg = strings.ReplaceAll(seg, "~0", "~")
		next, ok := cur.Get(seg)
		if !ok {
			return nil, fmt.Errorf("pointer segment %q not found", seg)
		}
		cur = next
	}
	return cur, nil
}
