package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/openkraft/openkraft/internal/domain"
)

var (
	sectionHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)
	warningItemStyle   = lipgloss.NewStyle().Foreground(warning)
	hintStyle          = lipgloss.NewStyle().Foreground(dim).Italic(true)
)

// RenderCheckReport renders a CheckReport as a styled TUI string.
func RenderCheckReport(report *domain.CheckReport) string {
	var b strings.Builder

	// Header
	grade := domain.GradeFor(report.Score)
	scoreStyled := lipgloss.NewStyle().
		Bold(true).
		Foreground(gradeColor(grade)).
		Render(fmt.Sprintf("%d/100  %s", report.Score, grade))

	moduleLine := titleStyle.Render(report.Module) + "  " + scoreStyled
	goldenLine := dimStyle.Render(fmt.Sprintf("vs golden: %s", report.GoldenModule))

	b.WriteString(boxStyle.Render(moduleLine + "\n" + goldenLine))
	b.WriteString("\n")

	renderMissingSection(&b, "Missing Files", report.MissingFiles, true)
	renderMissingSection(&b, "Missing Structures", report.MissingStructs, false)
	renderMissingSection(&b, "Missing Methods", report.MissingMethods, false)
	renderMissingSection(&b, "Missing Interfaces", report.MissingInterfaces, false)
	renderMissingSection(&b, "Pattern Violations", report.PatternViolations, false)

	// Footer
	b.WriteString("\n")
	b.WriteString("  " + hintStyle.Render("Use openkraft MCP server with Claude Code to fix automatically."))
	b.WriteString("\n")

	return b.String()
}

func renderMissingSection(b *strings.Builder, title string, items []domain.MissingItem, isFile bool) {
	if len(items) == 0 {
		return
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %s\n",
		sectionHeaderStyle.Render(title),
		dimStyle.Render(fmt.Sprintf("(%d)", len(items))),
	))

	for _, item := range items {
		if isFile {
			line := fmt.Sprintf("    %s %s", failStyle.Render("●"), item.Name)
			if item.Expected != "" {
				line += "  " + faintStyle.Render(item.Expected)
			}
			b.WriteString(line + "\n")
		} else {
			line := fmt.Sprintf("    %s ", warningItemStyle.Render("●"))
			if item.File != "" {
				line += fileStyle.Render(item.File) + "  " + item.Name
			} else {
				line += item.Name
			}
			b.WriteString(line + "\n")
		}
	}
}
