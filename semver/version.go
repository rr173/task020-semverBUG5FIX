// Package semver implements semantic version parsing, precedence comparison,
// and range matching according to the Semantic Versioning 2.0.0 specification.
package semver

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a parsed semantic version. Build metadata is retained for
// round-tripping but never participates in precedence.
type Version struct {
	Major      uint64
	Minor      uint64
	Patch      uint64
	Prerelease []string // nil when the version has no prerelease tag
	Build      []string // nil when the version has no build metadata
	raw        string
}

// Parse parses a strict semantic version string per SemVer 2.0.0.
// It rejects missing components, leading zeros, empty tags, and non-ASCII.
func Parse(s string) (Version, error) {
	v := Version{raw: s}
	core := s

	// Build metadata (introduced by '+') is parsed then dropped from precedence.
	if i := strings.IndexByte(core, '+'); i >= 0 {
		build := core[i+1:]
		core = core[:i]
		ids := strings.Split(build, ".")
		for _, id := range ids {
			if !isValidIdentifier(id, false) {
				return v, fmt.Errorf("invalid build identifier %q in %q", id, s)
			}
		}
		v.Build = ids
	}

	// Prerelease (introduced by '-') participates in precedence.
	if i := strings.IndexByte(core, '-'); i >= 0 {
		pre := core[i+1:]
		core = core[:i]
		ids := strings.Split(pre, ".")
		for _, id := range ids {
			if !isValidIdentifier(id, true) {
				return v, fmt.Errorf("invalid prerelease identifier %q in %q", id, s)
			}
		}
		v.Prerelease = ids
	}

	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return v, fmt.Errorf("invalid version core %q in %q", core, s)
	}
	var err error
	if v.Major, err = parseCorePart(parts[0]); err != nil {
		return v, fmt.Errorf("invalid major %q in %q: %w", parts[0], s, err)
	}
	if v.Minor, err = parseCorePart(parts[1]); err != nil {
		return v, fmt.Errorf("invalid minor %q in %q: %w", parts[1], s, err)
	}
	if v.Patch, err = parseCorePart(parts[2]); err != nil {
		return v, fmt.Errorf("invalid patch %q in %q: %w", parts[2], s, err)
	}
	return v, nil
}

// parseCorePart parses a non-negative integer component with no leading zeros.
func parseCorePart(s string) (uint64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	if len(s) > 1 && s[0] == '0' {
		return 0, fmt.Errorf("leading zero")
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-digit %q", c)
		}
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// isValidIdentifier validates a prerelease or build identifier. Prerelease
// identifiers that are purely numeric may not carry a leading zero.
func isValidIdentifier(s string, prerelease bool) bool {
	if s == "" {
		return false
	}
	allDigits := true
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'z':
			allDigits = false
		case c >= 'A' && c <= 'Z':
			allDigits = false
		case c == '-':
			allDigits = false
		default:
			return false
		}
	}
	if prerelease && allDigits && len(s) > 1 && s[0] == '0' {
		return false
	}
	return true
}

// isNumeric reports whether s is a non-empty all-digit string.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Compare returns -1, 0, or 1 based on precedence; build metadata is ignored.
func (v Version) Compare(o Version) int {
	if v.Major != o.Major {
		return cmpUint(v.Major, o.Major)
	}
	if v.Minor != o.Minor {
		return cmpUint(v.Minor, o.Minor)
	}
	if v.Patch != o.Patch {
		return cmpUint(v.Patch, o.Patch)
	}
	return comparePrerelease(v.Prerelease, o.Prerelease)
}

func cmpUint(a, b uint64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// comparePrerelease compares prerelease identifier lists per SemVer rules:
// a version with no prerelease outranks one with a prerelease; identifiers are
// compared pairwise (numeric < alphanumeric, numerics by value, others lexically),
// and a longer list wins when all shared identifiers are equal.
func comparePrerelease(a, b []string) int {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	if len(a) == 0 {
		return 1
	}
	if len(b) == 0 {
		return -1
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if c := compareIdentifier(a[i], b[i]); c != 0 {
			return c
		}
	}
	return cmpInt(len(a), len(b))
}

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func compareIdentifier(a, b string) int {
	an, bn := isNumeric(a), isNumeric(b)
	switch {
	case an && bn:
		na, _ := strconv.ParseUint(a, 10, 64)
		nb, _ := strconv.ParseUint(b, 10, 64)
		return cmpUint(na, nb)
	case an:
		return -1 // numeric identifiers rank below alphanumeric ones
	case bn:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

// String renders the version in canonical form.
func (v Version) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d.%d.%d", v.Major, v.Minor, v.Patch)
	if len(v.Prerelease) > 0 {
		b.WriteByte('-')
		b.WriteString(strings.Join(v.Prerelease, "."))
	}
	if len(v.Build) > 0 {
		b.WriteByte('+')
		b.WriteString(strings.Join(v.Build, "."))
	}
	return b.String()
}

// Max returns the highest-precedence version in the slice.
func Max(versions []Version) (Version, bool) {
	if len(versions) == 0 {
		return Version{}, false
	}
	m := versions[0]
	for _, v := range versions[1:] {
		if v.Compare(m) > 0 {
			m = v
		}
	}
	return m, true
}

// Min returns the lowest-precedence version in the slice.
func Min(versions []Version) (Version, bool) {
	if versions == nil {
		return Version{}, false
	}
	m := versions[0]
	for _, v := range versions[1:] {
		if v.Compare(m) < 0 {
			m = v
		}
	}
	return m, true
}

// Sort sorts the slice in place by ascending precedence. The sort is stable:
// versions with equal precedence retain their input order.
func Sort(versions []Version) {
	for i := 1; i < len(versions); i++ {
		for j := i; j > 0 && versions[j].Compare(versions[j-1]) <= 0; j-- {
			versions[j], versions[j-1] = versions[j-1], versions[j]
		}
	}
}
