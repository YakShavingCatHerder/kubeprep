package terminal

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const fenceMarker = "```"

var (
	// Fenced commands: green, with a gutter, so they read as something to type.
	scenarioCodeGutter = lipgloss.NewStyle().Foreground(lipgloss.Color("65"))
	scenarioCodeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	// Inline tokens: the same cyan as section labels, so READY/STATUS stay in the
	// sentence instead of looking like a second command.
	scenarioInlineStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110"))
	// Body copy sits one step down so commands and keywords carry the hierarchy.
	scenarioProseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

// styleScenarioMarkup highlights fenced blocks and inline `code` spans.
// Fence marker lines (``` or ```lang) are not shown. ::page:: is handled
// before this runs.
func styleScenarioMarkup(text string) string {
	if text == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	var b strings.Builder
	inFence := false
	for i, line := range lines {
		if isFenceMarker(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			b.WriteString(scenarioCodeGutter.Render("│ ") + scenarioCodeStyle.Render(line))
		} else if strings.Contains(line, "\x1b") {
			b.WriteString(line)
		} else {
			b.WriteString(styleInlineCode(line))
		}
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func isFenceMarker(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), fenceMarker)
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
