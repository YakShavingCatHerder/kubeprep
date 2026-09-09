package terminal

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	fenceMarker           = "```"
	scenarioPrompt        = "$ "
	scenarioDiagramIndent = "  "
)

var (
	// Fenced commands: a display-only prompt, then green. The $ is chrome,
	// not something to type and not something authors put in the YAML.
	scenarioPromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	scenarioCodeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	// Diagrams and YAML samples: indented reading matter, no prompt.
	scenarioDiagramStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	// Inline tokens: the same cyan as section labels, so READY/STATUS stay in the
	// sentence instead of looking like a second command.
	scenarioInlineStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110"))
	// Body copy sits one step down so commands and keywords carry the hierarchy.
	scenarioProseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

func isCommandFence(lang string) bool {
	switch lang {
	case "kubectl", "sh", "bash", "shell", "zsh", "console":
		return true
	default:
		return false
	}
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
// before this runs.
func styleScenarioMarkup(text string) string {
	if text == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	var b strings.Builder
	inFence := false
	commandFence := false
	for i, line := range lines {
		if lang, ok := fenceLanguage(line); ok {
			if !inFence {
				inFence = true
				commandFence = isCommandFence(lang)
			} else {
				inFence = false
				commandFence = false
			}
			continue
		}
		if inFence {
			if commandFence {
				if strings.TrimSpace(line) != "" {
					b.WriteString(scenarioPromptStyle.Render(scenarioPrompt) + scenarioCodeStyle.Render(line))
				}
			} else if line != "" {
				b.WriteString(scenarioDiagramStyle.Render(scenarioDiagramIndent + line))
			}
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
