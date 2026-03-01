package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/abdidvp/openkraft/internal/domain"
)

// ── Claude-inspired warm palette ──
var (
	accent    = lipgloss.Color("#D97706") // amber
	fg        = lipgloss.Color("#E8E6E3") // warm light gray
	dim       = lipgloss.Color("#6B7280") // muted gray
	faint     = lipgloss.Color("#3F3F46") // very dim
	success   = lipgloss.Color("#22C55E") // green
	danger    = lipgloss.Color("#EF4444") // red
	warning   = lipgloss.Color("#F59E0B") // amber-yellow
	info      = lipgloss.Color("#8B949E") // soft blue-gray
	skipColor = lipgloss.Color("#4B5563") // dark gray
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accent).
			Align(lipgloss.Center)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(1, 4).
			Align(lipgloss.Center).
			Width(68)

	gradeColors = map[string]lipgloss.Color{
		"A+": success,
		"A":  success,
		"B":  lipgloss.Color("#A3E635"), // lime
		"C":  warning,
		"D":  lipgloss.Color("#FB923C"), // orange
		"F":  danger,
	}

	dimStyle      = lipgloss.NewStyle().Foreground(dim)
	faintStyle    = lipgloss.NewStyle().Foreground(faint)
	passStyle     = lipgloss.NewStyle().Foreground(success)
	failStyle     = lipgloss.NewStyle().Foreground(danger)
	warnStyle     = lipgloss.NewStyle().Foreground(warning)
	skipStyle     = lipgloss.NewStyle().Foreground(skipColor)
	errorTagStyle = lipgloss.NewStyle().Foreground(danger).Bold(true)
	warnTagStyle  = lipgloss.NewStyle().Foreground(warning).Bold(true)
	infoTagStyle  = lipgloss.NewStyle().Foreground(info)
	fileStyle     = lipgloss.NewStyle().Foreground(dim)
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(fg)
	catNameStyle  = lipgloss.NewStyle().Bold(true).Foreground(fg)
	separatorLine = faintStyle.Render(strings.Repeat("─", 64))
)

func RenderScore(score *domain.Score) string {
	var b strings.Builder

	// ── Header ──
	grade := score.Grade()
	title := headerStyle.Render("openkraft")
	subtitle := dimStyle.Render("AI-Readiness Score")
	scoreLine := fmt.Sprintf("%d / 100", score.Overall)
	scoreStyled := lipgloss.NewStyle().
		Bold(true).
		Foreground(gradeColor(grade)).
		Render(scoreLine)
	gradeStyled := lipgloss.NewStyle().
		Bold(true).
		Foreground(gradeColor(grade)).
		Render(grade)

	b.WriteString(boxStyle.Render(title + "\n" + subtitle + "\n\n" + scoreStyled + "  " + gradeStyled))
	b.WriteString("\n\n")

	// ── Categories ──
	for i, cat := range score.Categories {
		renderCategoryFull(&b, cat)
		if i < len(score.Categories)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString("  " + separatorLine)
	b.WriteString("\n\n")

	// ── Issues ──
	issues := collectAndSortIssues(score)
	if len(issues) > 0 {
		errorCount, warnCount, infoCount := countSeverities(issues)
		b.WriteString("  ")
		b.WriteString(titleStyle.Render("Issues"))
		b.WriteString("  ")
		if errorCount > 0 {
			b.WriteString(errorTagStyle.Render(fmt.Sprintf("%d errors", errorCount)))
			b.WriteString("  ")
		}
		if warnCount > 0 {
			b.WriteString(warnTagStyle.Render(fmt.Sprintf("%d warnings", warnCount)))
			b.WriteString("  ")
		}
		if infoCount > 0 {
			b.WriteString(infoTagStyle.Render(fmt.Sprintf("%d info", infoCount)))
		}
		b.WriteString("\n\n")

		for _, issue := range issues {
			renderIssue(&b, issue)
		}
	} else {
		b.WriteString("  " + passStyle.Render("No issues found.") + "\n")
	}

	b.WriteString("\n")
	return b.String()
}

func renderCategoryFull(b *strings.Builder, cat domain.CategoryScore) {
	// Category header
	color := scoreColor(cat.Score)
	scoreText := lipgloss.NewStyle().Bold(true).Foreground(color).Render(fmt.Sprintf("%d", cat.Score))
	bar := coloredBar(cat.Score, 20)
	weight := dimStyle.Render(fmt.Sprintf("%d%%", int(cat.Weight*100)))

	name := catNameStyle.Render(padRight(cat.Name, 20))
	fmt.Fprintf(b, "  %s %s  %s %s\n", name, bar, scoreText, weight)

	// Sub-metrics
	for _, sm := range cat.SubMetrics {
		renderSubMetric(b, sm)
	}
}

func renderSubMetric(b *strings.Builder, sm domain.SubMetric) {
	name := padRight(sm.Name, 34)

	if sm.Skipped {
		fmt.Fprintf(b, "    %s %s %s\n",
			skipStyle.Render("○"),
			skipStyle.Render(name),
			skipStyle.Render("skipped"),
		)
		return
	}

	pct := 0
	if sm.Points > 0 {
		pct = sm.Score * 100 / sm.Points
	}

	var icon string
	switch {
	case pct >= 80:
		icon = passStyle.Render("●")
	case pct >= 40:
		icon = warnStyle.Render("●")
	default:
		icon = failStyle.Render("●")
	}

	score := dimStyle.Render(fmt.Sprintf("%d/%d", sm.Score, sm.Points))

	if sm.Detail != "" {
		fmt.Fprintf(b, "    %s %s %s  %s\n", icon, name, score, faintStyle.Render(sm.Detail))
	} else {
		fmt.Fprintf(b, "    %s %s %s\n", icon, name, score)
	}
}

func renderIssue(b *strings.Builder, issue domain.Issue) {
	tag := severityTag(issue.Severity)
	file := ""
	if issue.File != "" {
		file = shortenPath(issue.File)
	}

	if file != "" {
		fmt.Fprintf(b, "    %s %s\n", tag, fileStyle.Render(file))
		fmt.Fprintf(b, "         %s\n", dimStyle.Render(issue.Message))
	} else {
		fmt.Fprintf(b, "    %s %s\n", tag, dimStyle.Render(issue.Message))
	}
}

func severityTag(severity string) string {
	switch severity {
	case domain.SeverityError:
		return errorTagStyle.Render("error")
	case domain.SeverityWarning:
		return warnTagStyle.Render("warn ")
	default:
		return infoTagStyle.Render("info ")
	}
}

func countSeverities(issues []domain.Issue) (errors, warnings, infos int) {
	for _, i := range issues {
		switch i.Severity {
		case domain.SeverityError:
			errors++
		case domain.SeverityWarning:
			warnings++
		default:
			infos++
		}
	}
	return
}

func collectAndSortIssues(score *domain.Score) []domain.Issue {
	var all []domain.Issue
	for _, cat := range score.Categories {
		all = append(all, cat.Issues...)
	}
	sortBySeverity(all)
	return all
}

func sortBySeverity(issues []domain.Issue) {
	order := map[string]int{
		domain.SeverityError:   0,
		domain.SeverityWarning: 1,
		domain.SeverityInfo:    2,
	}
	for i := 1; i < len(issues); i++ {
		for j := i; j > 0 && order[issues[j].Severity] < order[issues[j-1].Severity]; j-- {
			issues[j], issues[j-1] = issues[j-1], issues[j]
		}
	}
}

func coloredBar(score, width int) string {
	filled := max(0, min(score*width/100, width))
	empty := width - filled

	color := scoreColor(score)
	filledStr := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled))
	emptyStr := lipgloss.NewStyle().Foreground(faint).Render(strings.Repeat("░", empty))
	return filledStr + emptyStr
}

func scoreColor(score int) lipgloss.Color {
	switch {
	case score >= 80:
		return success
	case score >= 60:
		return lipgloss.Color("#A3E635") // lime
	case score >= 40:
		return warning
	default:
		return danger
	}
}

func shortenPath(path string) string {
	if idx := strings.Index(path, "internal/"); idx >= 0 {
		return path[idx:]
	}
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) > 3 {
		return strings.Join(parts[len(parts)-3:], "/")
	}
	return path
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// RenderHistory formats score history for terminal output.
func RenderHistory(entries []domain.ScoreEntry) string {
	if len(entries) == 0 {
		return "  " + dimStyle.Render("No score history found.") + "\n"
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + titleStyle.Render("Score History") + "\n")
	b.WriteString("  " + faintStyle.Render(strings.Repeat("─", 50)) + "\n\n")

	for i, e := range entries {
		hash := e.CommitHash
		if len(hash) > 7 {
			hash = hash[:7]
		}
		if hash == "" {
			hash = "·······"
		}

		scoreStyled := lipgloss.NewStyle().
			Foreground(scoreColor(e.Overall)).
			Render(fmt.Sprintf("%d/100", e.Overall))

		line := fmt.Sprintf("  %s  %s  %s  %s",
			dimStyle.Render(e.Timestamp[:10]),
			faintStyle.Render(hash),
			scoreStyled,
			e.Grade,
		)

		if i > 0 {
			diff := e.Overall - entries[i-1].Overall
			if diff > 0 {
				line += "  " + passStyle.Render(fmt.Sprintf("↑%d", diff))
			} else if diff < 0 {
				line += "  " + failStyle.Render(fmt.Sprintf("↓%d", -diff))
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
	return fg
}

