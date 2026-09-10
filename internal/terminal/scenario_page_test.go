package terminal

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestClampStoryPage(t *testing.T) {
	if got := clampStoryPage(-1, 3); got != 0 {
		t.Fatalf("negative page = %d", got)
	}
	if got := clampStoryPage(8, 3); got != 2 {
		t.Fatalf("overshoot = %d, want 2", got)
	}
}

func TestPaginateScenarioFillsViewport(t *testing.T) {
	text := strings.Repeat("line\n", 10)
	pages := paginateScenario(text, 20, 4)
	if len(pages) != 3 {
		t.Fatalf("pages = %d, want 3", len(pages))
	}
	if len(pages[0]) != 4 {
		t.Fatalf("first page lines = %d, want 4", len(pages[0]))
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

func TestScenarioPageCueOmitsSinglePage(t *testing.T) {
	if got := scenarioPageCue(0, 1); got != "" {
		t.Fatalf("single page cue = %q", got)
	}
	if got := scenarioPageCue(1, 5); got != "2/5" {
		t.Fatalf("cue = %q, want 2/5", got)
	}
}

func TestFitCellsTruncatesWithEllipsis(t *testing.T) {
	got := fitCells("SCENARIO · A Very Long Lab Title", 12)
	if lipgloss.Width(got) > 12 {
		t.Fatalf("width = %d, want <= 12: %q", lipgloss.Width(got), got)
	}
	if !strings.Contains(got, "…") {
		t.Fatalf("expected ellipsis, got %q", got)
	}
}

func TestSplitScenarioCaptionReservesOneRow(t *testing.T) {
	inner, cap, story := scenarioPaneRegions(pane{Width: 40, Height: 20})
	if inner.Width != 38 || inner.Height != 18 {
		t.Fatalf("inner = %+v", inner)
	}
	if cap != 1 || story.Width != 34 || story.Height != 15 {
		t.Fatalf("caption=%d story=%+v", cap, story)
	}
}
