package terminal

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	fenceMarker           = "```"
	scenarioPrompt        = "$ "
	scenarioDiagramIndent = "  "
	flowPadX              = 2
	flowPadY              = 1
)

var (
	// Fenced commands: a display-only prompt, then green. The $ is chrome,
	// not something to type and not something authors put in the YAML.
	scenarioPromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	scenarioCodeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	// Diagrams and YAML samples: indented reading matter, no prompt.
	scenarioDiagramStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	// Inline tokens: bold cyan, so READY/STATUS stay in the sentence.
	scenarioInlineStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110"))
	// Body section labels: same brightness as prose, bold, not keyword cyan.
	scenarioSectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	// Body copy sits one step down so commands and keywords carry the hierarchy.
	scenarioProseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	// Flow nodes: italic teal, not keyword cyan and not command green.
	flowNodeStyle      = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("73"))
	flowConnectorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	paneBorderStyle    = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("238"))
)

func isCommandFence(lang string) bool {
	switch lang {
	case "kubectl", "sh", "bash", "shell", "zsh", "console":
		return true
	default:
		return false
	}
}

func isYAMLFence(lang string) bool {
	return lang == "yaml" || lang == "yml"
}

func fenceLanguage(line string) (lang string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, fenceMarker) {
		return "", false
	}
	lang = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, fenceMarker)))
	if i := strings.IndexAny(lang, " \t"); i >= 0 {
		lang = lang[:i]
	}
	return lang, true
}

// styleScenarioMarkup highlights fenced blocks and inline `code` spans.
// Fence marker lines (``` or ```lang) are not shown. ::page:: is handled
// before this runs. Pass width 0 when the story column is unknown.
func styleScenarioMarkup(text string) string {
	return styleScenarioMarkupAtWidth(text, 0)
}

func styleScenarioMarkupAtWidth(text string, width int) string {
	if text == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	var b strings.Builder
	inFence := false
	commandFence := false
	yamlFence := false
	var fence []string
	emit := func(s string) {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(s)
	}
	flushFence := func() {
		body := renderedFence(fence, commandFence, yamlFence, width)
		if body != "" {
			emit(body)
		}
		fence = nil
	}
	for _, line := range lines {
		if lang, ok := fenceLanguage(line); ok {
			if inFence {
				flushFence()
				inFence = false
				commandFence = false
				yamlFence = false
			} else {
				inFence = true
				commandFence = isCommandFence(lang)
				yamlFence = isYAMLFence(lang)
				fence = nil
			}
			continue
		}
		if inFence {
			fence = append(fence, line)
			continue
		}
		if strings.Contains(line, "\x1b") {
			emit(line)
		} else {
			emit(styleInlineCode(line))
		}
	}
	if inFence {
		flushFence()
	}
	return b.String()
}

func renderedFence(fence []string, commandFence, yamlFence bool, width int) string {
	if commandFence {
		var lines []string
		for _, line := range fence {
			if strings.TrimSpace(line) == "" {
				continue
			}
			lines = append(lines, scenarioPromptStyle.Render(scenarioPrompt)+scenarioCodeStyle.Render(line))
		}
		return strings.Join(lines, "\n")
	}
	if !yamlFence {
		if nodes := flowNodes(fence); len(nodes) >= 2 {
			return renderFlowPanel(nodes, width)
		}
	}
	var lines []string
	for _, line := range fence {
		if line == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, scenarioDiagramStyle.Render(scenarioDiagramIndent+line))
	}
	return strings.Join(lines, "\n")
}

func flowNodes(lines []string) []string {
	var nonempty []string
	for _, line := range lines {
		if text := strings.TrimSpace(line); text != "" {
			nonempty = append(nonempty, text)
		}
	}
	if nodes := arrowFlowNodes(nonempty); len(nodes) >= 2 {
		return nodes
	}
	return pipeFlowNodes(nonempty)
}

func arrowFlowNodes(lines []string) []string {
	if len(lines) < 2 || !isFlowNode(lines[0]) {
		return nil
	}
	nodes := []string{lines[0]}
	for _, line := range lines[1:] {
		rest, ok := strings.CutPrefix(line, "->")
		if !ok {
			return nil
		}
		node := strings.TrimSpace(rest)
		if !isFlowNode(node) {
			return nil
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func pipeFlowNodes(lines []string) []string {
	if len(lines) < 3 || !isFlowNode(lines[0]) {
		return nil
	}
	nodes := []string{lines[0]}
	i := 1
	for i < len(lines) {
		if !isFlowConnector(lines[i]) {
			return nil
		}
		for i < len(lines) && isFlowConnector(lines[i]) {
			i++
		}
		if i >= len(lines) || !isFlowNode(lines[i]) {
			return nil
		}
		nodes = append(nodes, lines[i])
		i++
	}
	if len(nodes) < 2 {
		return nil
	}
	return nodes
}

func isFlowConnector(s string) bool {
	switch s {
	case "|", "v", "V", "▼", "↓", "->":
		return true
	default:
		return false
	}
}

func isFlowNode(s string) bool {
	if s == "" || isFlowConnector(s) {
		return false
	}
	if strings.HasPrefix(s, "|") || strings.HasPrefix(s, "->") {
		return false
	}
	return true
}

func renderFlowPanel(nodes []string, paneWidth int) string {
	inner := 1
	for _, node := range nodes {
		if w := lipgloss.Width(node); w > inner {
			inner = w
		}
	}
	cardWidth := inner + 2*flowPadX
	if paneWidth > 0 {
		cardWidth = paneWidth
	}
	inner = cardWidth - 2*flowPadX
	if inner < 1 {
		inner = cardWidth
	}
	var rows []string
	for range flowPadY {
		rows = append(rows, "")
	}
	for i, node := range nodes {
		if i > 0 {
			rows = append(rows, flowConnectorStyle.Width(cardWidth).Align(lipgloss.Center).Render("│"))
			rows = append(rows, flowConnectorStyle.Width(cardWidth).Align(lipgloss.Center).Render("▼"))
		}
		rows = append(rows, flowNodeStyle.Width(cardWidth).Align(lipgloss.Center).Render(fitCells(node, inner)))
	}
	for range flowPadY {
		rows = append(rows, "")
	}
	return strings.Join(rows, "\n")
}

func isFenceMarker(line string) bool {
	_, ok := fenceLanguage(line)
	return ok
}

func styleInlineCode(line string) string {
	var b strings.Builder
	rest := line
	for {
		start := strings.Index(rest, "`")
		if start < 0 {
			b.WriteString(styleProse(rest))
			return b.String()
		}
		closeOffset := strings.Index(rest[start+1:], "`")
		if closeOffset < 0 {
			b.WriteString(styleProse(rest))
			return b.String()
		}
		end := start + 1 + closeOffset
		b.WriteString(styleProse(rest[:start]))
		code := rest[start+1 : end]
		if code == "" {
			b.WriteString("``")
		} else {
			b.WriteString(scenarioInlineStyle.Render(code))
		}
		rest = rest[end+1:]
	}
}

func styleProse(s string) string {
	if s == "" {
		return s
	}
	return scenarioProseStyle.Render(s)
}
