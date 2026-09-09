package terminal

import (
	"strings"
	"testing"
)

func TestClampStoryPage(t *testing.T) {
	if got := clampStoryPage(-1, 3); got != 0 {
		t.Fatalf("negative page = %d", got)
	}
	if got := clampStoryPage(8, 3); got != 2 {
		t.Fatalf("overshoot = %d, want 2", got)
	}
}

func TestPaginateScenarioReservesHintRow(t *testing.T) {
	text := strings.Repeat("line\n", 10)
	pages := paginateScenario(text, 20, 4)
	if len(pages) != 4 {
		t.Fatalf("pages = %d, want 4", len(pages))
	}
	if len(pages[0]) != 3 {
		t.Fatalf("first page lines = %d, want 3 so the page cue fits", len(pages[0]))
	}
	if len(pages[3]) != 1 {
		t.Fatalf("last page lines = %d, want 1", len(pages[3]))
	}
}

func TestAuthorPageBreakStartsANewPage(t *testing.T) {
	text := "FIRST-PAGE unique\n\n::page::\n\nSECOND-PAGE unique"
	pages := paginateScenario(text, 40, 20)
	if len(pages) != 2 {
		t.Fatalf("pages = %d, want 2", len(pages))
	}
	first := strings.Join(pages[0], "\n")
	second := strings.Join(pages[1], "\n")
	if !strings.Contains(first, "FIRST-PAGE") || strings.Contains(first, "SECOND-PAGE") {
		t.Fatalf("page 1 = %q", first)
	}
	if !strings.Contains(second, "SECOND-PAGE") || strings.Contains(second, "FIRST-PAGE") {
		t.Fatalf("page 2 = %q", second)
	}
	if strings.Contains(first, pageBreakLine) || strings.Contains(second, pageBreakLine) {
		t.Fatal("page-break marker should not be shown")
	}
}

func TestAuthorPageBreakIgnoresSurroundingWhitespace(t *testing.T) {
	pages := paginateScenario("alpha\n  ::page::  \nbeta", 40, 20)
	if len(pages) != 2 {
		t.Fatalf("pages = %d, want 2", len(pages))
	}
}

func TestLongForcedPageStillAutoSplits(t *testing.T) {
	text := "short\n::page::\n" + strings.Repeat("The API server persists the object.\n", 20)
	pages := paginateScenario(text, 24, 6)
	if len(pages) < 3 {
		t.Fatalf("pages = %d, want auto-split after the author break", len(pages))
	}
	if !strings.Contains(strings.Join(pages[0], "\n"), "short") {
		t.Fatalf("first page = %q", pages[0])
	}
}
