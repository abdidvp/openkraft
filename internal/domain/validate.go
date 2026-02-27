package domain

type ValidationResult struct {
	Status       string       `json:"status"`
	FilesChecked []string     `json:"files_checked"`
	DriftIssues  []DriftIssue `json:"drift_issues"`
	ScoreImpact  ScoreImpact  `json:"score_impact"`
	Suggestions  []string     `json:"suggestions"`
}

type DriftIssue struct {
	File      string `json:"file"`
	Line      int    `json:"line,omitempty"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	Category  string `json:"category"`
	DriftType string `json:"drift_type"`
}

type ScoreImpact struct {
	Overall    int            `json:"overall"`
	Categories map[string]int `json:"categories"`
}
