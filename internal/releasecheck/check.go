package releasecheck

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Version is a vMAJOR.MINOR.PATCH release identifier.
type Version struct {
	Major int
	Minor int
	Patch int
}

func (v Version) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v Version) less(other Version) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	return v.Patch < other.Patch
}

func (v Version) nextPatch() Version {
	return Version{v.Major, v.Minor, v.Patch + 1}
}

func (v Version) nextMinor() Version {
	return Version{v.Major, v.Minor + 1, 0}
}

func (v Version) nextMajor() Version {
	return Version{v.Major + 1, 0, 0}
}

func (v Version) allowedNext() []Version {
	return []Version{v.nextPatch(), v.nextMinor(), v.nextMajor()}
}

// Parse accepts only vMAJOR.MINOR.PATCH. It rejects a missing v, extra dots
// (v.0.1.3), prerelease suffixes, and build metadata.
func Parse(tag string) (Version, error) {
	if !strings.HasPrefix(tag, "v") {
		return Version{}, fmt.Errorf("tag %q must look like v0.1.0", tag)
	}
	parts := strings.Split(tag[1:], ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("tag %q must look like v0.1.0", tag)
	}
	nums := make([]int, 3)
	for i, part := range parts {
		if part == "" || !isPlainDigits(part) {
			return Version{}, fmt.Errorf("tag %q must look like v0.1.0", tag)
		}
		if len(part) > 1 && part[0] == '0' {
			return Version{}, fmt.Errorf("tag %q must look like v0.1.0", tag)
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return Version{}, fmt.Errorf("tag %q must look like v0.1.0", tag)
		}
		nums[i] = n
	}
	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2]}, nil
}

func isPlainDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Latest returns the highest vMAJOR.MINOR.PATCH tag. Candidate is ignored so a
// tag-push checkout (which already contains the new tag) still sees the previous
// release as current.
func Latest(tags []string, candidate string) (Version, bool, error) {
	var found []Version
	for _, tag := range tags {
		if tag == candidate {
			continue
		}
		v, err := Parse(tag)
		if err != nil {
			continue
		}
		found = append(found, v)
	}
	if len(found) == 0 {
		return Version{}, false, nil
	}
	sort.Slice(found, func(i, j int) bool { return found[i].less(found[j]) })
	return found[len(found)-1], true, nil
}

// Validate reports whether candidate is the next allowed release.
//
// With no prior release, only v0.1.0 is allowed.
// Otherwise the tag must be exactly one of: next patch, next minor (.0), or
// next major (.0.0). That blocks going backwards and skipping (v0.1.4 → v0.1.6).
func Validate(candidate string, existing []string) error {
	want, err := Parse(candidate)
	if err != nil {
		return err
	}
	latest, ok, err := Latest(existing, candidate)
	if err != nil {
		return err
	}
	if !ok {
		first := Version{Major: 0, Minor: 1, Patch: 0}
		if want != first {
			return fmt.Errorf("first release must be %s, got %s", first, want)
		}
		return nil
	}
	allowed := latest.allowedNext()
	for _, next := range allowed {
		if want == next {
			return nil
		}
	}
	return fmt.Errorf("tag %s is not the next release after %s (allowed: %s, %s, %s)",
		want, latest, allowed[0], allowed[1], allowed[2])
}
