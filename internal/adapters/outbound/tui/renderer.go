package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/openkraft/openkraft/internal/domain"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00ff88")).
			Align(lipgloss.Center)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#00ff88")).
			Padding(1, 4).
			Align(lipgloss.Center).
			Width(48)

	gradeColors = map[string]lipgloss.Color{
		"A+": lipgloss.Color("#00ff88"),
		"A":  lipgloss.Color("#00cc66"),
		"B":  lipgloss.Color("#ffcc00"),
		"C":  lipgloss.Color("#ff9900"),
		"D":  lipgloss.Color("#ff4444"),
		"F":  lipgloss.Color("#cc0000"),
	}

	labelStyle = lipgloss.NewStyle().Width(24)
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
)

func RenderScore(score *domain.Score) string {
	var b strings.Builder

	// Header box
	title := headerStyle.Render("OpenKraft AI-Readiness Score")
	scoreText := fmt.Sprintf("%d / 100", score.Overall)
	scoreStyled := lipgloss.NewStyle().
		Bold(true).
		Foreground(gradeColor(score.Grade())).
		Render(scoreText)

	box := boxStyle.Render(title + "\n" + scoreStyled)
	b.WriteString(box)
	b.WriteString("\n\n")

	// Category bars
	for _, cat := range score.Categories {
		b.WriteString(renderCategory(cat))
		b.WriteString("\n")
	}

	// Grade
	grade := score.Grade()
	gradeStyled := lipgloss.NewStyle().
		Bold(true).
		Foreground(gradeColor(grade)).
		Render(grade)
	b.WriteString(fmt.Sprintf("\n  Grade: %s\n", gradeStyled))

	return b.String()
}

func renderCategory(cat domain.CategoryScore) string {
	label := labelStyle.Render(fmt.Sprintf("  %s", cat.Name))
	bar := progressBar(cat.Score, 15)
	weight := dimStyle.Render(fmt.Sprintf("(weight: %d%%)", int(cat.Weight*100)))
	return fmt.Sprintf("%s %s  %d/100  %s", label, bar, cat.Score, weight)
}

func progressBar(score, width int) string {
	filled := score * width / 100
	empty := width - filled
	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

// RenderHistory formats score history for terminal output.
func RenderHistory(entries []domain.ScoreEntry) string {
	if len(entries) == 0 {
		return "  No score history found.\n"
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Score History"))
	b.WriteString("\n\n")

	for i, e := range entries {
		hash := e.CommitHash
		if len(hash) > 7 {
			hash = hash[:7]
		}
		if hash == "" {
			hash = "-------"
		}

		line := fmt.Sprintf("  %s  %s  %d/100  %s", e.Timestamp[:10], hash, e.Overall, e.Grade)

		if i > 0 {
			diff := e.Overall - entries[i-1].Overall
			if diff > 0 {
				line += fmt.Sprintf("  ↑%d", diff)
			} else if diff < 0 {
				line += fmt.Sprintf("  ↓%d", -diff)
			}
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func gradeColor(grade string) lipgloss.Color {
	if c, ok := gradeColors[grade]; ok {
		return c
	}
	return lipgloss.Color("#ffffff")
}
