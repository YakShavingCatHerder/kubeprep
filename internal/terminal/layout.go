package terminal

const (
	minSideBySideWidth = 100
	defaultWidth       = 80
	defaultHeight      = 24
	headerLines        = 1
	defaultFooterLines = 1
	hintFooterLines    = 3
	stackedScenarioCap = 12
	scenarioInnerPad   = 1
)

// pane is a rectangular region in terminal cells.
type pane struct {
	Width  int
	Height int
}

func (p pane) Inner() pane {
	if p.Width < 4 || p.Height < 3 {
		return p
	}
	return pane{Width: p.Width - 2, Height: p.Height - 2}
}

// Inset shrinks a pane by n cells on every side.
func (p pane) Inset(n int) pane {
	if n < 1 {
		return p
	}
	width := p.Width - 2*n
	height := p.Height - 2*n
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	return pane{Width: width, Height: height}
}

// Content is the writable area inside the border and inner padding.
func (p pane) Content() pane {
	return p.Inner().Inset(scenarioInnerPad)
}

// splitLayout is the permanent scenario | lab-shell geometry.
type splitLayout struct {
	Header     pane
	Scenario   pane
	Shell      pane
	Footer     pane
	SideBySide bool
	Zoomed     bool
}

func computeSplitLayout(width, height int, zoomed bool, footerHeight int) splitLayout {
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}
	if footerHeight < 1 {
		footerHeight = defaultFooterLines
	}

	header := pane{Width: width, Height: headerLines}
	footer := pane{Width: width, Height: footerHeight}
	bodyHeight := height - header.Height - footer.Height
	if bodyHeight < 4 {
		bodyHeight = 4
	}

	layout := splitLayout{
		Header: header,
		Footer: footer,
		Zoomed: zoomed,
	}
	if zoomed {
		layout.Shell = pane{Width: width, Height: bodyHeight}
		return layout
	}
	if width >= minSideBySideWidth {
		scenarioWidth := width * 2 / 5
		if scenarioWidth < 36 {
			scenarioWidth = 36
		}
		if scenarioWidth > width-42 {
			scenarioWidth = width - 42
		}
		if scenarioWidth < 20 {
			scenarioWidth = width / 2
		}
		layout.SideBySide = true
		layout.Scenario = pane{Width: scenarioWidth, Height: bodyHeight}
		layout.Shell = pane{Width: width - scenarioWidth, Height: bodyHeight}
		return layout
	}
	scenarioHeight := bodyHeight / 2
	if scenarioHeight < 6 {
		scenarioHeight = 6
	}
	if scenarioHeight > stackedScenarioCap {
		scenarioHeight = stackedScenarioCap
	}
	if scenarioHeight > bodyHeight-6 {
		scenarioHeight = bodyHeight - 6
	}
	if scenarioHeight < 4 {
		scenarioHeight = bodyHeight / 2
	}
	layout.Scenario = pane{Width: width, Height: scenarioHeight}
	layout.Shell = pane{Width: width, Height: bodyHeight - scenarioHeight}
	return layout
}
