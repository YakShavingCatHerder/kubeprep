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
	rendered := lipgloss.NewStyle().Width(width).Render(styleScenarioMarkupAtWidth(text, width))
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
	return packPages(splitForcedPages(text), width, height)
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

func scenarioPageCue(page, pages int) string {
	if pages <= 1 {
		return ""
	}
	return fmt.Sprintf("%d/%d", page+1, pages)
}

func splitScenarioCaption(box pane) (captionHeight int, story pane) {
	story = box
	if box.Height >= 2 {
		return 1, pane{Width: box.Width, Height: box.Height - 1}
	}
	return 0, story
}

// scenarioPaneRegions is the bordered interior, a sticky caption row, and the
// padded story box. The caption shares the first inner row with LAB SHELL.
func scenarioPaneRegions(size pane) (inner pane, captionHeight int, story pane) {
	inner = size.Inner()
	if inner.Width < 1 {
		inner.Width = 1
	}
	if inner.Height < 1 {
		inner.Height = 1
	}
	if inner.Height >= 2 {
		captionHeight = 1
	}
	story = pane{
		Width:  inner.Width - 2*paneInnerPad,
		Height: inner.Height - captionHeight - paneInnerPad,
	}
	if story.Width < 1 {
		story.Width = 1
	}
	if story.Height < 1 {
		story.Height = 1
	}
	return inner, captionHeight, story
}

func scenarioPaneCaption(title string, width int) string {
	label := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110"))
	caption := "SCENARIO"
	if name := strings.TrimSpace(title); name != "" {
		caption += " · " + name
	}
	return padPaneCaption(label.Render(fitCells(caption, captionInnerWidth(width))), width)
}

func captionInnerWidth(width int) int {
	inner := width - 2*paneInnerPad
	if inner < 1 {
		return 1
	}
	return inner
}

func padPaneCaption(text string, width int) string {
	if width < 1 {
		width = 1
	}
	return lipgloss.NewStyle().
		Padding(0, paneInnerPad).
		Width(width).
		MaxWidth(width).
		Render(text)
}

func padPaneBody(body string, inner pane, captionHeight int) string {
	bodyHeight := inner.Height - captionHeight
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	return lipgloss.NewStyle().
		Padding(0, paneInnerPad, paneInnerPad, paneInnerPad).
		Width(inner.Width).
		Height(bodyHeight).
		MaxWidth(inner.Width).
		MaxHeight(bodyHeight).
		Render(body)
}

func fitCells(s string, width int) string {
	if width < 1 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	const ellipsis = "…"
	if width <= lipgloss.Width(ellipsis) {
		return ellipsis
	}
	budget := width - lipgloss.Width(ellipsis)
	var b strings.Builder
	for _, r := range s {
		next := b.String() + string(r)
		if lipgloss.Width(next) > budget {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + ellipsis
}
