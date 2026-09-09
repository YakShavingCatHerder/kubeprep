package terminal

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func TestStyleScenarioMarkupHidesFenceMarkers(t *testing.T) {
	got := styleScenarioMarkup("Start with:\n```\nkubectl get pod nginx\n```\nThen compare.")
	if strings.Contains(got, "```") {
		t.Fatalf("fence markers still visible:\n%s", got)
	}
	if !strings.Contains(got, "kubectl get pod nginx") {
		t.Fatalf("command missing:\n%s", got)
	}
	if !strings.Contains(got, "Start with:") || !strings.Contains(got, "Then compare.") {
		t.Fatalf("prose missing:\n%s", got)
	}
	if !strings.Contains(got, "\x1b") {
		t.Fatal("expected ANSI styling on the fenced command")
	}
	if !strings.Contains(got, "│ ") {
		t.Fatal("expected a gutter on the fenced command")
	}
	code := scenarioCodeStyle.Render("kubectl get pod nginx")
	if strings.Contains(code, "48;") {
		t.Fatal("fenced commands should not set a background")
	}
}

func TestStyleScenarioMarkupAcceptsLanguageTag(t *testing.T) {
	got := styleScenarioMarkup("```kubectl\nkubectl get pod nginx -o wide\n```")
	if strings.Contains(got, "```") || strings.Contains(got, "kubectl\nkubectl") {
		t.Fatalf("language tag leaked:\n%s", got)
	}
	if !strings.Contains(got, "kubectl get pod nginx -o wide") {
		t.Fatalf("command missing:\n%s", got)
	}
}

func TestStyleScenarioMarkupHighlightsInlineCode(t *testing.T) {
	got := styleScenarioMarkup("Pay attention to `READY` and `STATUS`.")
	if strings.Contains(got, "`READY`") || strings.Contains(got, "`STATUS`") {
		t.Fatalf("backticks still visible:\n%s", got)
	}
	if !strings.Contains(got, "READY") || !strings.Contains(got, "STATUS") {
		t.Fatalf("tokens missing:\n%s", got)
	}
	if !strings.Contains(got, "\x1b") {
		t.Fatal("expected ANSI styling on inline code")
	}
	inline := scenarioInlineStyle.Render("READY")
	if strings.Contains(inline, "48;") {
		t.Fatal("inline code should not set a background")
	}
}

func TestStyleScenarioMarkupDistinguishesCommandsFromKeywords(t *testing.T) {
	if scenarioCodeStyle.Render("READY") == scenarioInlineStyle.Render("READY") {
		t.Fatal("command and keyword styles should differ")
	}
	got := styleScenarioMarkup("See `READY` then:\n```\nkubectl get pod nginx\n```")
	if !strings.Contains(got, "│ ") {
		t.Fatal("commands should keep the gutter")
	}
	ready := strings.Index(got, "READY")
	gutter := strings.Index(got, "│ ")
	if ready < 0 || gutter < 0 {
		t.Fatalf("missing pieces:\n%s", got)
	}
	if ready > gutter {
		t.Fatal("expected the keyword before the fenced command in this sample")
	}
}

func TestStyleScenarioMarkupLeavesUnmatchedBacktick(t *testing.T) {
	got := styleScenarioMarkup("Leave this `alone")
	if !strings.Contains(got, "`alone") {
		t.Fatalf("unmatched backtick should stay visible:\n%s", got)
	}
}

func TestStyleScenarioMarkupDimsProseAndKeepsLabels(t *testing.T) {
	if styleScenarioMarkup("Inspect the cluster.") == "Inspect the cluster." {
		t.Fatal("prose should be dimmed")
	}
	if styleScenarioMarkup("Inspect the cluster.") == scenarioInlineStyle.Render("Inspect the cluster.") {
		t.Fatal("prose should not use the keyword style")
	}
	label := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110")).Render("OBJECTIVE")
	got := styleScenarioMarkup(label + "\nInspect the cluster.")
	if !strings.Contains(got, label) {
		t.Fatalf("section labels should stay bright:\n%s", got)
	}
}

func TestStyleScenarioMarkupSkipsInlineInsideFence(t *testing.T) {
	got := styleScenarioMarkup("```\necho `READY`\n```")
	if !strings.Contains(got, "`READY`") {
		t.Fatalf("backticks inside a fence should stay literal:\n%s", got)
	}
}

func TestPaginateScenarioKeepsFencesAndPageBreaks(t *testing.T) {
	text := "first page\n```\nkubectl get pod nginx\n```\n::page::\nsecond page `READY`"
	pages := paginateScenario(text, 40, 20)
	if len(pages) != 2 {
		t.Fatalf("pages = %d, want 2", len(pages))
	}
	first := strings.Join(pages[0], "\n")
	second := strings.Join(pages[1], "\n")
	if !strings.Contains(first, "kubectl get pod nginx") || strings.Contains(first, "second page") {
		t.Fatalf("page 1 = %q", first)
	}
	if !strings.Contains(second, "READY") || strings.Contains(second, "first page") {
		t.Fatalf("page 2 = %q", second)
	}
	if strings.Contains(first, "```") || strings.Contains(second, "`READY`") {
		t.Fatalf("markup leaked into pages:\n1=%q\n2=%q", first, second)
	}
}

func TestWrapScenarioLinesRespectsWidthWithCode(t *testing.T) {
	text := "```\n" + strings.Repeat("kubectl get pod nginx ", 8) + "\n```"
	for _, line := range wrapScenarioLines(text, 24) {
		if got := lipgloss.Width(line); got > 24 {
			t.Fatalf("line width = %d, want <= 24: %q", got, line)
		}
	}
}
