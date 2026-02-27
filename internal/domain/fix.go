package domain

type FixPlan struct {
	Applied      []AppliedFix  `json:"applied"`
	Instructions []Instruction `json:"instructions"`
	ScoreBefore  int           `json:"score_before"`
	ScoreAfter   int           `json:"score_after"`
}

type AppliedFix struct {
	Type        string `json:"type"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

type Instruction struct {
	Type        string `json:"type"`
	File        string `json:"file"`
	Line        int    `json:"line,omitempty"`
	Message     string `json:"message"`
	Priority    string `json:"priority"`
	ProjectNorm string `json:"project_norm"`
}

type FixOptions struct {
	DryRun   bool   `json:"dry_run"`
	AutoOnly bool   `json:"auto_only"`
	Category string `json:"category"`
}
