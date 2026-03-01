package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/openkraft/openkraft/internal/domain"
	"github.com/openkraft/openkraft/internal/domain/scoring"
)

const graphMaxRows = 15

// RenderGraph produces a terminal-formatted visualization of the import graph
// including a summary header, metrics table, cycles, and coupling outliers.
func RenderGraph(graph *scoring.ImportGraph, modulePath string, profile *domain.ScoringProfile) string {
	if graph == nil || len(graph.Packages) == 0 {
		return "\n  " + dimStyle.Render("No import graph available (no go.mod found).") + "\n\n"
	}

	annotated := graph.ClassifyPackages(modulePath, profile)

	var b strings.Builder

	// ── Header box ──
	renderGraphHeader(&b, graph, modulePath, scoring.TotalViolations(annotated))

	// ── Metrics table ──
	renderMetricsTable(&b, annotated, modulePath)

	// ── Cycles ──
	renderCyclesSection(&b, graph)

	// ── Coupling outliers ──
	multiplier := 2.0
	if profile != nil && profile.CouplingOutlierMultiplier > 0 {
		multiplier = profile.CouplingOutlierMultiplier
	}
	renderOutliersSection(&b, graph, modulePath, multiplier)

	b.WriteString("\n")
	return b.String()
}

func renderGraphHeader(b *strings.Builder, graph *scoring.ImportGraph, modulePath string, violations int) {
	pkgCount := len(graph.Packages)
	edgeCount := graph.EdgeCount()
	cycleCount := len(graph.DetectCycles())

	title := headerStyle.Render("Import Graph")
	modLine := lipgloss.NewStyle().Bold(true).Foreground(fg).Render(modulePath)

	violationLabel := passStyle.Render(fmt.Sprintf("%d violations", violations))
	if violations > 0 {
		violationLabel = failStyle.Render(fmt.Sprintf("%d violations", violations))
	}

	stats := dimStyle.Render(fmt.Sprintf(
		"%d packages  ·  %d edges  ·  %d cycles  ·  ", pkgCount, edgeCount, cycleCount)) + violationLabel

	b.WriteString(boxStyle.Render(title + "\n\n" + modLine + "\n" + stats))
	b.WriteString("\n\n")
}

type annotatedRow struct {
	shortName  string
	ca, ce     int
	role       scoring.ArchRole
	violations []scoring.PackageViolation
}

func renderMetricsTable(b *strings.Builder, annotated map[string]*scoring.AnnotatedPackage, modulePath string) {
	var rows []annotatedRow
	for pkg, ap := range annotated {
		rows = append(rows, annotatedRow{
			shortName:  stripModulePrefix(pkg, modulePath),
			ca:         len(ap.Node.ImportedBy),
			ce:         len(ap.Node.ImportsInternal),
			role:       ap.Role,
			violations: ap.Violations,
		})
	}

	// Sort: packages with violations first, then by Ca desc, then alphabetical.
	sort.Slice(rows, func(i, j int) bool {
		vi := len(rows[i].violations) > 0
		vj := len(rows[j].violations) > 0
		if vi != vj {
			return vi
		}
		if rows[i].ca != rows[j].ca {
			return rows[i].ca > rows[j].ca
		}
		return rows[i].shortName < rows[j].shortName
	})

	// Header.
	hdrLine := fmt.Sprintf("  %-32s %3s %3s  %-14s  %s",
		"Package", "Ca", "Ce", "Role", "Violations")
	b.WriteString(titleStyle.Render(hdrLine) + "\n")
	b.WriteString("  " + faintStyle.Render(strings.Repeat("─", 68)) + "\n")

	shown := graphMaxRows
	if len(rows) < shown {
		shown = len(rows)
	}

	for _, r := range rows[:shown] {
		name := truncateOrPad(r.shortName, 32)
		role := roleLabel(r.role)
		viol := renderViolations(r.violations)

		line := fmt.Sprintf("  %s %3d %3d  %s  %s",
			dimStyle.Render(name), r.ca, r.ce, role, viol)
		b.WriteString(line + "\n")
	}

	remaining := len(rows) - shown
	if remaining > 0 {
		b.WriteString(faintStyle.Render(fmt.Sprintf("  (%d more packages)\n", remaining)))
	}

	b.WriteString("\n")
}

func roleLabel(role scoring.ArchRole) string {
	s := string(role)
	styled := dimStyle.Render(padRight(s, 14))
	switch role {
	case scoring.RoleCore:
		styled = passStyle.Render(padRight(s, 14))
	case scoring.RolePorts:
		styled = passStyle.Render(padRight(s, 14))
	case scoring.RoleAdapter:
		styled = dimStyle.Render(padRight(s, 14))
	case scoring.RoleOrchestrator:
		styled = dimStyle.Render(padRight(s, 14))
	case scoring.RoleEntryPoint:
		styled = dimStyle.Render(padRight(s, 14))
	}
	return styled
}

func renderViolations(violations []scoring.PackageViolation) string {
	if len(violations) == 0 {
		return dimStyle.Render("—")
	}
	msg := failStyle.Render("✘ " + violations[0].Message)
	if len(violations) > 1 {
		msg += failStyle.Render(fmt.Sprintf(" (+%d)", len(violations)-1))
	}
	return msg
}

func renderCyclesSection(b *strings.Builder, graph *scoring.ImportGraph) {
	b.WriteString("  " + titleStyle.Render("Cycles") + "\n")
	cycles := graph.DetectCycles()
	if len(cycles) == 0 {
		b.WriteString("    " + passStyle.Render("(none)") + "\n")
	} else {
		for _, cycle := range cycles {
			// Show as a → b → c → a
			parts := make([]string, len(cycle))
			copy(parts, cycle)
			parts = append(parts, cycle[0])
			b.WriteString("    " + failStyle.Render(strings.Join(parts, " → ")) + "\n")
		}
	}
	b.WriteString("\n")
}

func renderOutliersSection(b *strings.Builder, graph *scoring.ImportGraph, modulePath string, multiplier float64) {
	b.WriteString("  " + titleStyle.Render("Coupling Outliers") + "\n")
	outliers := graph.CouplingOutliers(multiplier)
	if len(outliers) == 0 {
		b.WriteString("    " + passStyle.Render("(none)") + "\n")
	} else {
		for _, o := range outliers {
			short := stripModulePrefix(o.Package, modulePath)
			b.WriteString("    " + warnStyle.Render(fmt.Sprintf(
				"%s imports %d packages (median: %.0f)", short, o.Ce, o.MedianCe)) + "\n")
		}
	}
}

func truncateOrPad(s string, width int) string {
	if len(s) > width {
		return s[:width-1] + "…"
	}
	return padRight(s, width)
}

func stripModulePrefix(pkg, modulePath string) string {
	if modulePath == "" {
		return pkg
	}
	trimmed := strings.TrimPrefix(pkg, modulePath+"/")
	if trimmed == pkg {
		// Might be the root module itself.
		if pkg == modulePath {
			return "./"
		}
		return pkg
	}
	return trimmed
}
