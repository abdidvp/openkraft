package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/openkraft/openkraft/internal/domain"
)

var (
	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#ffcc00"))

	missingFileStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ff4444"))

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff9900"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true)
)

// RenderCheckReport renders a CheckReport as a styled TUI string.
func RenderCheckReport(report *domain.CheckReport) string {
	var b strings.Builder

	// Header box with module name, score, and golden module
	grade := domain.GradeFor(report.Score)
	scoreColor := gradeColor(grade)

	scoreText := lipgloss.NewStyle().
		Bold(true).
		Foreground(scoreColor).
		Render(fmt.Sprintf("Score: %d/100", report.Score))

	moduleLine := fmt.Sprintf("Module: %s — %s", report.Module, scoreText)
	goldenLine := fmt.Sprintf("Golden module: %s", report.GoldenModule)

	box := boxStyle.Render(moduleLine + "\n" + goldenLine)
	b.WriteString(box)
	b.WriteString("\n")

	// Missing Files
	if len(report.MissingFiles) > 0 {
		b.WriteString("\n")
		header := sectionHeaderStyle.Render(
			fmt.Sprintf("  MISSING FILES (%d):", len(report.MissingFiles)))
		b.WriteString(header)
		b.WriteString("\n")
		for _, item := range report.MissingFiles {
			line := missingFileStyle.Render(
				fmt.Sprintf("  ✗ %-34s", item.Name))
			if item.Expected != "" {
				line += dimStyle.Render(fmt.Sprintf(" (expected: %s)", item.Expected))
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Missing Structures
	if len(report.MissingStructs) > 0 {
		b.WriteString("\n")
		header := sectionHeaderStyle.Render(
			fmt.Sprintf("  MISSING STRUCTURES (%d):", len(report.MissingStructs)))
		b.WriteString(header)
		b.WriteString("\n")
		for _, item := range report.MissingStructs {
			line := warningStyle.Render("  ⚠ ")
			if item.File != "" {
				line += fmt.Sprintf("%s — Missing: %s", item.File, item.Name)
			} else {
				line += item.Name
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Missing Methods
	if len(report.MissingMethods) > 0 {
		b.WriteString("\n")
		header := sectionHeaderStyle.Render(
			fmt.Sprintf("  MISSING METHODS (%d):", len(report.MissingMethods)))
		b.WriteString(header)
		b.WriteString("\n")
		for _, item := range report.MissingMethods {
			line := warningStyle.Render("  ⚠ ")
			if item.File != "" {
				line += fmt.Sprintf("%s — Missing: %s", item.File, item.Name)
			} else {
				line += item.Name
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Missing Interfaces
	if len(report.MissingInterfaces) > 0 {
		b.WriteString("\n")
		header := sectionHeaderStyle.Render(
			fmt.Sprintf("  MISSING INTERFACES (%d):", len(report.MissingInterfaces)))
		b.WriteString(header)
		b.WriteString("\n")
		for _, item := range report.MissingInterfaces {
			line := warningStyle.Render("  ⚠ ")
			if item.File != "" {
				line += fmt.Sprintf("%s — Missing: %s", item.File, item.Name)
			} else {
				line += item.Name
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Pattern Violations
	if len(report.PatternViolations) > 0 {
		b.WriteString("\n")
		header := sectionHeaderStyle.Render(
			fmt.Sprintf("  PATTERN VIOLATIONS (%d):", len(report.PatternViolations)))
		b.WriteString(header)
		b.WriteString("\n")
		for _, item := range report.PatternViolations {
			line := warningStyle.Render("  ⚠ ") + item.Name
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Footer hint
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("  Run with an AI agent (Claude Code + MCP) to fix automatically."))
	b.WriteString("\n")

	return b.String()
}
