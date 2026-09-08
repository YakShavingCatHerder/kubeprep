package releasecheck

import (
	"strings"
	"testing"
)

func TestParseRejectsMalformedTags(t *testing.T) {
	for _, tag := range []string{"0.1.0", "v.0.1.3", "v0.1", "v0.1.4-rc.1", "v0.1.4+build", "v01.2.3", "vv0.1.0", ""} {
		if _, err := Parse(tag); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", tag)
		}
	}
}

func TestParseAcceptsReleaseTags(t *testing.T) {
	got, err := Parse("v0.1.4")
	if err != nil {
		t.Fatal(err)
	}
	if got != (Version{0, 1, 4}) {
		t.Fatalf("Parse(v0.1.4) = %v", got)
	}
}

func TestValidateFirstRelease(t *testing.T) {
	if err := Validate("v0.1.0", nil); err != nil {
		t.Fatal(err)
	}
	if err := Validate("v0.1.1", nil); err == nil {
		t.Fatal("first tag v0.1.1 should fail")
	}
}

func TestValidateNextBumps(t *testing.T) {
	existing := []string{"v0.1.0", "v0.1.3", "v0.1.4", "not-a-tag", "v0.1.4-rc.1"}
	cases := []struct {
		tag string
		ok  bool
	}{
		{"v0.1.5", true},
		{"v0.2.0", true},
		{"v1.0.0", true},
		{"v0.1.4", true}, // candidate is excluded; previous is v0.1.3
		{"v0.1.3", false},
		{"v0.1.6", false},
		{"v0.3.0", false},
		{"v2.0.0", false},
		{"v.0.1.5", false},
	}
	for _, tc := range cases {
		err := Validate(tc.tag, existing)
		if tc.ok && err != nil {
			t.Errorf("Validate(%s) = %v, want nil", tc.tag, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("Validate(%s) succeeded, want error", tc.tag)
		}
	}
}

func TestLatestIgnoresCandidateAlreadyOnTheRepo(t *testing.T) {
	latest, ok, err := Latest([]string{"v0.1.4", "v0.1.5"}, "v0.1.5")
	if err != nil || !ok {
		t.Fatalf("Latest() ok=%v err=%v", ok, err)
	}
	if latest.String() != "v0.1.4" {
		t.Fatalf("latest = %s, want v0.1.4", latest)
	}
}

func TestValidateErrorMentionsAllowedTags(t *testing.T) {
	err := Validate("v0.1.6", []string{"v0.1.4"})
	if err == nil {
		t.Fatal("expected skip error")
	}
	if !strings.Contains(err.Error(), "v0.1.5") || !strings.Contains(err.Error(), "v0.2.0") {
		t.Fatalf("error = %v, want allowed next versions", err)
	}
}
