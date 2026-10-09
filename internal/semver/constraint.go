// Package semver — constraint grammar.
//
// A Constraint is a boolean predicate over semantic versions. It supports:
//
//   latest             → highest stable (filters out pre-releases)
//   latest-prerelease  → highest including pre-releases
//   exact              → "1.24.0"
//   caret              → "^1.24.0"  (>=1.24.0, <2.0.0)
//   tilde              → "~1.24.0"  (>=1.24.0, <1.25.0)
//   tilde (partial)    → "~1.24"    (>=1.24.0, <1.25.0)
//   comparators        → ">=1.24.0", ">1.24.0", "<=1.26.0", "<1.26.0", "=1.24.0"
//   AND of comparators → ">=1.24.0, <1.26.0"  or  ">=1.24.0 <1.26.0"
//
// Partial versions in constraints are allowed. "^1.24" is treated as "^1.24.0";
// "~1" is treated as ">=1.0.0, <2.0.0".
//
// A Constraint is created with ParseConstraint. The returned value exposes
// a Match method and the Raw text it was created from.

package semver

import (
	"fmt"
	"strconv"
	"strings"
)

// Constraint is a predicate on semver.Version. It is produced by
// ParseConstraint and may represent a conjunction (AND) of simple
// comparators or a special selector (latest / latest-prerelease).
type Constraint struct {
	Raw   string
	nodes []constraintNode
}

// constraintNode is a single predicate in a conjunction. Exactly one of
// cmp or special is populated.
type constraintNode struct {
	cmp     *comparator
	special specialKind
}

type specialKind int

const (
	specNone specialKind = iota
	specLatest
	specLatestPrerelease
)

type comparator struct {
	op  string  // one of: =, >, >=, <, <=
	ver Version // right-hand version
	// includePre controls whether the right-hand version's "zone" allows
	// pre-releases of versions outside the base range. For now we follow a
	// simple rule: comparators do not match pre-releases of versions that
	// are not explicitly referenced by the constraint, unless the comparator
	// RHS itself is a pre-release.
	includePre bool
}

// ParseConstraint parses the given string into a Constraint.
//
// It accepts the full grammar documented on the Constraint type. Empty
// strings are rejected.
func ParseConstraint(s string) (Constraint, error) {
	raw := s
	s = strings.TrimSpace(s)
	if s == "" {
		return Constraint{}, fmt.Errorf("empty constraint")
	}

	// Special-purpose selectors.
	switch strings.ToLower(s) {
	case "latest":
		return Constraint{
			Raw:   raw,
			nodes: []constraintNode{{special: specLatest}},
		}, nil
	case "latest-prerelease":
		return Constraint{
			Raw:   raw,
			nodes: []constraintNode{{special: specLatestPrerelease}},
		}, nil
	}

	// Split on commas or runs of whitespace into individual clauses.
	parts := splitConstraintClauses(s)
	if len(parts) == 0 {
		return Constraint{}, fmt.Errorf("empty constraint")
	}

	nodes := make([]constraintNode, 0, len(parts))
	for _, p := range parts {
		expanded, err := expandClause(p)
		if err != nil {
			return Constraint{}, err
		}
		for _, cmp := range expanded {
			cmpCopy := cmp
			nodes = append(nodes, constraintNode{cmp: &cmpCopy})
		}
	}

	return Constraint{Raw: raw, nodes: nodes}, nil
}

// splitConstraintClauses splits a constraint string on commas and whitespace
// while keeping operator+operand pairs together, e.g. ">= 1.24.0" collapses
// into ">=1.24.0" so it becomes a single token.
func splitConstraintClauses(s string) []string {
	var sb strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		sb.WriteRune(r)
		if r == '>' || r == '<' || r == '=' || r == '~' || r == '^' {
			// Swallow any immediate whitespace so the operator sticks to its
			// operand (e.g. ">= 1.24.0" → ">=1.24.0").
			for i+1 < len(runes) && (runes[i+1] == ' ' || runes[i+1] == '\t') {
				i++
			}
		}
	}
	normalized := strings.ReplaceAll(sb.String(), ",", " ")
	return strings.Fields(normalized)
}

// expandClause converts a single clause (e.g. "^1.24", ">=1.24.0", "1.24.0")
// into one or more comparators that together express it.
func expandClause(clause string) ([]comparator, error) {
	if clause == "" {
		return nil, fmt.Errorf("empty clause")
	}

	// Caret: ^X[.Y[.Z[-pre]]]  →  [>=X.Y.Z, <(X+1).0.0]  (for X > 0)
	//                              [>=0.Y.Z, <0.(Y+1).0] (for X == 0, Y > 0 — standard semver)
	// We implement the "major-anchored" variant for X >= 1 and the
	// "minor-anchored" variant for X == 0 to mirror the common SemVer rule.
	if strings.HasPrefix(clause, "^") {
		rest := strings.TrimPrefix(clause, "^")
		base, err := parsePartialVersion(rest)
		if err != nil {
			return nil, fmt.Errorf("invalid caret constraint %q: %w", clause, err)
		}
		lo := base.toFullZero()
		var hi Version
		if lo.Major > 0 {
			hi = Version{Major: lo.Major + 1}
		} else if lo.Minor > 0 {
			hi = Version{Major: 0, Minor: lo.Minor + 1}
		} else {
			// ^0.0.z → >=0.0.z, <0.0.(z+1)
			hi = Version{Major: 0, Minor: 0, Patch: lo.Patch + 1}
		}
		return []comparator{
			{op: ">=", ver: lo},
			{op: "<", ver: hi},
		}, nil
	}

	// Tilde: ~X[.Y[.Z[-pre]]]
	// ~X.Y.Z and ~X.Y both expand to [>=X.Y.0, <X.(Y+1).0]
	// ~X expands to [>=X.0.0, <(X+1).0.0]
	if strings.HasPrefix(clause, "~") {
		rest := strings.TrimPrefix(clause, "~")
		base, err := parsePartialVersion(rest)
		if err != nil {
			return nil, fmt.Errorf("invalid tilde constraint %q: %w", clause, err)
		}
		lo := base.toFullZero()
		var hi Version
		if base.hasMinor {
			hi = Version{Major: lo.Major, Minor: lo.Minor + 1}
		} else {
			hi = Version{Major: lo.Major + 1}
		}
		return []comparator{
			{op: ">=", ver: lo},
			{op: "<", ver: hi},
		}, nil
	}

	// Explicit comparator operators.
	for _, op := range []string{">=", "<=", ">", "<", "="} {
		if strings.HasPrefix(clause, op) {
			rest := strings.TrimPrefix(clause, op)
			base, err := parsePartialVersion(rest)
			if err != nil {
				return nil, fmt.Errorf("invalid constraint %q: %w", clause, err)
			}
			v := base.toFullZero()
			return []comparator{{op: op, ver: v, includePre: v.PreRelease != ""}}, nil
		}
	}

	// Bare version → exact equality.
	base, err := parsePartialVersion(clause)
	if err != nil {
		return nil, fmt.Errorf("invalid version %q: %w", clause, err)
	}
	if !base.hasMinor || !base.hasPatch {
		// Treat "1.24" as "~1.24" and "1" as "~1" for ergonomic parity
		// with the tilde form when used bare in a constraint position.
		lo := base.toFullZero()
		var hi Version
		if base.hasMinor {
			hi = Version{Major: lo.Major, Minor: lo.Minor + 1}
		} else {
			hi = Version{Major: lo.Major + 1}
		}
		return []comparator{
			{op: ">=", ver: lo},
			{op: "<", ver: hi},
		}, nil
	}
	v := base.toFullZero()
	return []comparator{{op: "=", ver: v, includePre: v.PreRelease != ""}}, nil
}

// partialVersion is an intermediate representation of a possibly partial
// version string like "1", "1.24", "1.24.0", or "1.24.0-alpha".
type partialVersion struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
	hasMinor   bool
	hasPatch   bool
}

func (p partialVersion) toFullZero() Version {
	return Version{
		Major:      p.Major,
		Minor:      p.Minor,
		Patch:      p.Patch,
		PreRelease: p.PreRelease,
		Original:   p.String(),
	}
}

func (p partialVersion) String() string {
	s := strconv.Itoa(p.Major)
	if p.hasMinor {
		s += "." + strconv.Itoa(p.Minor)
	}
	if p.hasPatch {
		s += "." + strconv.Itoa(p.Patch)
	}
	if p.PreRelease != "" {
		s += "-" + p.PreRelease
	}
	return s
}

// parsePartialVersion parses a version string with an optional "v" prefix.
// It accepts "1", "1.2", "1.2.3", and any of these suffixed with "-pre".
func parsePartialVersion(s string) (partialVersion, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return partialVersion{}, fmt.Errorf("empty version")
	}
	s = strings.TrimPrefix(s, "v")

	var pre string
	if idx := strings.Index(s, "-"); idx >= 0 {
		pre = s[idx+1:]
		s = s[:idx]
	}

	nums := strings.Split(s, ".")
	if len(nums) < 1 || len(nums) > 3 {
		return partialVersion{}, fmt.Errorf("version must have 1 to 3 dotted components, got %q", s)
	}

	var p partialVersion
	var err error
	p.Major, err = strconv.Atoi(nums[0])
	if err != nil {
		return partialVersion{}, fmt.Errorf("invalid major component %q: %w", nums[0], err)
	}
	if len(nums) >= 2 {
		p.Minor, err = strconv.Atoi(nums[1])
		if err != nil {
			return partialVersion{}, fmt.Errorf("invalid minor component %q: %w", nums[1], err)
		}
		p.hasMinor = true
	}
	if len(nums) == 3 {
		p.Patch, err = strconv.Atoi(nums[2])
		if err != nil {
			return partialVersion{}, fmt.Errorf("invalid patch component %q: %w", nums[2], err)
		}
		p.hasPatch = true
	}
	p.PreRelease = pre
	return p, nil
}

// IsConstraint reports whether s should be treated as a constraint rather than
// a plain exact version. Plain exact versions are of the form X.Y.Z or
// X.Y.Z-pre (with an optional leading "v").
//
// It is intentionally permissive about pre-release syntax (any trailing
// -<text>) because the semver.Parse helper is similarly permissive.
func IsConstraint(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	// Any explicit operator or special selector makes it a constraint.
	switch strings.ToLower(s) {
	case "latest", "latest-prerelease":
		return true
	}
	if strings.ContainsAny(s, "^~<>=,") {
		return true
	}
	// Space-separated → multi-clause constraint.
	if strings.ContainsAny(s, " \t") {
		return true
	}
	// Partial version like "1" or "1.24" is a constraint (resolves to range).
	stripped := strings.TrimPrefix(s, "v")
	var core = stripped
	if idx := strings.Index(stripped, "-"); idx >= 0 {
		core = stripped[:idx]
	}
	nums := strings.Split(core, ".")
	if len(nums) != 3 {
		return true
	}
	for _, n := range nums {
		if _, err := strconv.Atoi(n); err != nil {
			// Not a valid plain exact version — let the caller treat it as a
			// constraint and surface a parse error at that point.
			return true
		}
	}
	return false
}

// Match reports whether v satisfies the constraint.
//
// Semantics for pre-release versions: per the common SemVer convention, a
// version with a pre-release tag (e.g. 1.25.0-alpha) does not satisfy a range
// unless at least one bound of the range explicitly references a pre-release
// of the same (Major, Minor, Patch) tuple. The special selector
// "latest-prerelease" opts into pre-releases wholesale.
func (c Constraint) Match(v Version) bool {
	if len(c.nodes) == 0 {
		return false
	}

	// Handle special selectors: they can only appear alone.
	for _, n := range c.nodes {
		switch n.special {
		case specLatest:
			return v.PreRelease == ""
		case specLatestPrerelease:
			return true
		}
	}

	// Pre-release gating: a version is only eligible for ordinary comparator
	// matching if either (a) it has no pre-release tag, or (b) at least one
	// comparator references the same (Major, Minor, Patch) with a pre-release.
	if v.PreRelease != "" && !c.allowsPrereleaseOf(v) {
		return false
	}

	for _, n := range c.nodes {
		if n.cmp == nil {
			return false
		}
		if !n.cmp.match(v) {
			return false
		}
	}
	return true
}

func (c Constraint) allowsPrereleaseOf(v Version) bool {
	for _, n := range c.nodes {
		if n.cmp == nil {
			continue
		}
		if n.cmp.includePre &&
			n.cmp.ver.Major == v.Major &&
			n.cmp.ver.Minor == v.Minor &&
			n.cmp.ver.Patch == v.Patch {
			return true
		}
	}
	return false
}

func (cm comparator) match(v Version) bool {
	switch cm.op {
	case "=":
		return !Less(v, cm.ver) && !Less(cm.ver, v)
	case ">":
		return Less(cm.ver, v)
	case ">=":
		return !Less(v, cm.ver)
	case "<":
		return Less(v, cm.ver)
	case "<=":
		return !Less(cm.ver, v)
	}
	return false
}

// HighestMatching returns the newest version in candidates (treated as a list
// of version strings) that satisfies the constraint. Unparseable entries are
// skipped. Returns ("", false) if no candidate matches.
func HighestMatching(c Constraint, candidates []string) (string, bool) {
	sorted := SortDescending(candidates)
	for _, s := range sorted {
		v := Parse(s)
		// Skip entries that failed to parse (Original set but zero core and
		// no explicit zero version was supplied).
		if v.Major == 0 && v.Minor == 0 && v.Patch == 0 && v.PreRelease == "" &&
			!looksLikeZeroVersion(s) {
			continue
		}
		if c.Match(v) {
			return s, true
		}
	}
	return "", false
}

// looksLikeZeroVersion returns true if s is a legitimate representation of
// 0.0.0 (so we don't accidentally drop it from HighestMatching).
func looksLikeZeroVersion(s string) bool {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if idx := strings.Index(s, "-"); idx >= 0 {
		s = s[:idx]
	}
	return s == "0.0.0"
}
