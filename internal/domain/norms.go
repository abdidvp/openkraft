package domain

import "sort"

type ProjectNorms struct {
	FunctionLines int     `json:"function_lines"`
	FileLines     int     `json:"file_lines"`
	Parameters    int     `json:"parameters"`
	NamingStyle   string  `json:"naming_style"`
	NamingPct     float64 `json:"naming_pct"`
}

func ComputeNorms(analyzed map[string]*AnalyzedFile) ProjectNorms {
	var funcLines, fileLines, params []int

	for _, af := range analyzed {
		if af.TotalLines > 0 {
			fileLines = append(fileLines, af.TotalLines)
		}
		for _, fn := range af.Functions {
			lines := fn.LineEnd - fn.LineStart + 1
			if lines > 0 {
				funcLines = append(funcLines, lines)
			}
			params = append(params, len(fn.Params))
		}
	}

	return ProjectNorms{
		FunctionLines: percentile90(funcLines),
		FileLines:     percentile90(fileLines),
		Parameters:    percentile90(params),
	}
}

func percentile90(values []int) int {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]int, len(values))
	copy(sorted, values)
	sort.Ints(sorted)
	idx := int(float64(len(sorted)-1) * 0.9)
	return sorted[idx]
}
