package terminal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// pageBreakLine is an authoring marker. A YAML line that is exactly this
// value (optional surrounding whitespace) starts a new scenario-pane page.
const pageBreakLine = "::page::"

func wrapScenarioLines(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	rendered := lipgloss.NewStyle().Width(width).Render(styleScenarioMarkup(text))
	return strings.Split(rendered, "\n")
}

func splitForcedPages(text string) []string {
	var parts []string
	var b strings.Builder
	flush := func() {
		part := strings.TrimSpace(b.String())
		b.Reset()
		if part != "" {
			parts = append(parts, part)
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == pageBreakLine {
			flush()
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	flush()
	if len(parts) == 0 {
		return []string{""}
	}
	return parts
}

func packPages(parts []string, width, viewport int) [][]string {
	if viewport < 1 {
		viewport = 1
	}
	var pages [][]string
	for _, part := range parts {
		lines := wrapScenarioLines(part, width)
		if len(lines) == 1 && strings.TrimSpace(lines[0]) == "" {
			continue
		}
		for start := 0; start < len(lines); start += viewport {
			end := start + viewport
			if end > len(lines) {
				end = len(lines)
			}
			pages = append(pages, lines[start:end])
		}
	}
	if len(pages) == 0 {
		return [][]string{{""}}
	}
	return pages
}

func paginateScenario(text string, width, height int) [][]string {
	if height < 1 {
		height = 1
	}
	parts := splitForcedPages(text)
	pages := packPages(parts, width, height)
	if len(pages) > 1 && height > 1 {
		pages = packPages(parts, width, height-1)
	}
	return pages
}

func clampStoryPage(page, pages int) int {
	if pages < 1 {
		pages = 1
	}
	if page < 0 {
		return 0
	}
	if page >= pages {
		return pages - 1
	}
	return page
}

func scenarioPageHint(page, pages int) string {
	if pages <= 1 {
		return ""
	}
	return fmt.Sprintf("page %d/%d  Alt+←→", page+1, pages)
}
