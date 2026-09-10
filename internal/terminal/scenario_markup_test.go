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
	got := styleScenarioMarkup("Start with:\n```kubectl\nkubectl get pod nginx\n```\nThen compare.")
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
	if !strings.Contains(got, scenarioPromptStyle.Render(scenarioPrompt)) {
		t.Fatal("expected a prompt on the fenced command")
	}
	if strings.Contains(got, "│ ") {
		t.Fatal("commands should not use a box-drawing gutter")
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
	got := styleScenarioMarkup("See `READY` then:\n```kubectl\nkubectl get pod nginx\n```")
	prompt := scenarioPromptStyle.Render(scenarioPrompt)
	if !strings.Contains(got, prompt) {
		t.Fatal("commands should show a prompt")
	}
	ready := strings.Index(got, "READY")
	dollar := strings.Index(got, prompt)
	if ready < 0 || dollar < 0 {
		t.Fatalf("missing pieces:\n%s", got)
	}
	if ready > dollar {
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
	label := scenarioSectionStyle.Render("OBJECTIVE")
	got := styleScenarioMarkup(label + "\nInspect the cluster.")
	if !strings.Contains(got, label) {
		t.Fatalf("precolored labels should not be restyled:\n%s", got)
	}
}

func TestStyleScenarioMarkupCommandsDifferFromDiagrams(t *testing.T) {
	command := styleScenarioMarkup("```kubectl\nkubectl get pod nginx\n```")
	diagram := styleScenarioMarkup("```text\nkubectl\n  -> API server\n```")
	if command == diagram {
		t.Fatal("command fences and diagram fences should render differently")
	}
	prompt := scenarioPromptStyle.Render(scenarioPrompt)
	if !strings.Contains(command, prompt) {
		t.Fatalf("kubectl fence should show a prompt:\n%s", command)
	}
	if strings.Contains(diagram, prompt) {
		t.Fatalf("text fence should not show a prompt:\n%s", diagram)
	}
	if !strings.Contains(command, scenarioCodeStyle.Render("kubectl get pod nginx")) {
		t.Fatalf("kubectl fence should use the command style:\n%s", command)
	}
	if strings.Contains(diagram, scenarioCodeStyle.Render("kubectl")) {
		t.Fatalf("text fence should not use the command style:\n%s", diagram)
	}
	if strings.Contains(diagram, "->") {
		t.Fatalf("arrow chains should not keep authoring arrows:\n%s", diagram)
	}
	if !strings.Contains(diagram, "▼") || !strings.Contains(diagram, "API server") {
		t.Fatalf("arrow chains should render as a vertical flow:\n%s", diagram)
	}
	unlabeled := styleScenarioMarkup("```\nscheduling the Pod\n```")
	if strings.Contains(unlabeled, scenarioCodeStyle.Render("scheduling the Pod")) {
		t.Fatalf("unlabeled fences should be diagrams, not commands:\n%s", unlabeled)
	}
	if !strings.Contains(unlabeled, scenarioDiagramStyle.Render(scenarioDiagramIndent+"scheduling the Pod")) {
		t.Fatalf("unlabeled fences should be indented diagrams:\n%s", unlabeled)
	}
}

func TestStyleScenarioMarkupRendersCenteredFlowPanel(t *testing.T) {
	src := "```text\nyou\n  -> kubectl\n  -> Kubernetes API\n  -> Kubernetes cluster\n```"
	got := styleScenarioMarkupAtWidth(src, 40)
	if strings.Contains(got, "```") || strings.Contains(got, "->") {
		t.Fatalf("flow leaked authoring chrome:\n%s", got)
	}
	for _, node := range []string{"you", "kubectl", "Kubernetes API", "Kubernetes cluster"} {
		if !strings.Contains(got, node) {
			t.Fatalf("missing node %q:\n%s", node, got)
		}
	}
	if strings.Count(got, "▼") != 3 {
		t.Fatalf("want 3 connectors, got:\n%s", got)
	}
	if strings.Contains(got, "48;") {
		t.Fatalf("flow should not paint a background box:\n%s", got)
	}
	if !flowNodeStyle.GetItalic() {
		t.Fatal("flow nodes should be italic")
	}
	if flowNodeStyle.Render("kubectl") == scenarioInlineStyle.Render("kubectl") {
		t.Fatal("flow nodes should not match keyword style")
	}
	if flowNodeStyle.Render("kubectl") == scenarioCodeStyle.Render("kubectl") {
		t.Fatal("flow nodes should not match command style")
	}
	if strings.Contains(got, scenarioCodeStyle.Render("kubectl")) {
		t.Fatalf("flow nodes should not use the command style:\n%s", got)
	}
	var nodeLine string
	for _, line := range strings.Split(got, "\n") {
		if w := lipgloss.Width(line); w > 40 {
			t.Fatalf("flow line width = %d, want <= 40: %q", w, line)
		}
		if strings.Contains(line, "you") {
			nodeLine = line
		}
	}
	if nodeLine == "" {
		t.Fatalf("you node missing:\n%s", got)
	}
	if lipgloss.Width(nodeLine) != 40 {
		t.Fatalf("flow panel should span the story column, width = %d:\n%s", lipgloss.Width(nodeLine), got)
	}
}

func TestStyleScenarioMarkupDoesNotPromoteGlossOrYAML(t *testing.T) {
	gloss := styleScenarioMarkup("```text\nkubectl\n  Use the Kubernetes command-line client.\nget\n  Ask Kubernetes to show something.\n```")
	if strings.Contains(gloss, "▼") || strings.Contains(gloss, "48;") {
		t.Fatalf("glossary should stay an indented diagram:\n%s", gloss)
	}
	yamlFence := styleScenarioMarkup("```yaml\napiVersion: v1\nkind: Pod\n```")
	if strings.Contains(yamlFence, "▼") || strings.Contains(yamlFence, "48;") {
		t.Fatalf("yaml fences should not become flows:\n%s", yamlFence)
	}
	shape := styleScenarioMarkup("```text\nkubectl <verb> <resource>\n```")
	if strings.Contains(shape, "▼") {
		t.Fatalf("one-line shapes should not become flows:\n%s", shape)
	}
}

func TestStyleScenarioMarkupRendersPipeFlow(t *testing.T) {
	got := styleScenarioMarkup("```text\nYAML manifest\n|\nv\nkubectl\n```")
	if strings.Contains(got, "->") || !strings.Contains(got, "▼") {
		t.Fatalf("pipe/v chains should render as a flow:\n%s", got)
	}
	if !strings.Contains(got, "YAML manifest") || !strings.Contains(got, "kubectl") {
		t.Fatalf("pipe/v nodes missing:\n%s", got)
	}
}

func TestWrapScenarioLinesKeepsFlowWithinWidth(t *testing.T) {
	text := "```text\nyou\n  -> kubectl\n  -> Kubernetes API\n```"
	for _, line := range wrapScenarioLines(text, 36) {
		if got := lipgloss.Width(line); got > 36 {
			t.Fatalf("line width = %d, want <= 36: %q", got, line)
		}
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
