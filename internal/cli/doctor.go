package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/YakShavingCatHerder/kubeprep/internal/cluster"
	"github.com/charmbracelet/lipgloss"
)

var doctorHeaderColors = []string{"110", "75", "183", "73", "216"}

func writeDoctorReport(out io.Writer, results []cluster.CheckResult) bool {
	color := writerIsTerminal(out)
	failed := false
	for i, result := range results {
		if i > 0 {
			fmt.Fprintln(out)
		}
		checking := result.Checking
		if checking == "" {
			checking = "checking " + strings.ToLower(result.Name)
		}
		fmt.Fprintln(out, doctorHeaderStyle(i, color).Render(checking))
		for _, line := range doctorBodyLines(result) {
			fmt.Fprintln(out, doctorDetailStyle(color).Render("  "+line))
		}
		status := result.Status
		if status == "" {
			status = strings.ToLower(result.Name)
		}
		if result.OK {
			fmt.Fprintln(out, doctorPassStyle(color).Render("✓ "+status))
			continue
		}
		failed = true
		fmt.Fprintln(out, doctorFailStyle(color).Render("✗ "+status))
	}
	return failed
}

func doctorBodyLines(result cluster.CheckResult) []string {
	var lines []string
	for _, block := range []string{result.Detail, result.Remediation} {
		if strings.TrimSpace(block) == "" {
			continue
		}
		for _, line := range strings.Split(block, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			lines = append(lines, line)
		}
	}
	return lines
}

func doctorHeaderStyle(index int, color bool) lipgloss.Style {
	style := lipgloss.NewStyle()
	if !color {
		return style
	}
	fg := doctorHeaderColors[index%len(doctorHeaderColors)]
	return style.Foreground(lipgloss.Color(fg))
}

func doctorDetailStyle(color bool) lipgloss.Style {
	style := lipgloss.NewStyle()
	if !color {
		return style
	}
	return style.Foreground(lipgloss.Color("245"))
}

func doctorPassStyle(color bool) lipgloss.Style {
	style := lipgloss.NewStyle()
	if !color {
		return style
	}
	return style.Foreground(lipgloss.Color("114"))
}

func doctorFailStyle(color bool) lipgloss.Style {
	style := lipgloss.NewStyle()
	if !color {
		return style
	}
	return style.Foreground(lipgloss.Color("203"))
}
