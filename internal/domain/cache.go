package domain

type ProjectCache struct {
	ProjectPath   string                   `json:"project_path"`
	ConfigHash    string                   `json:"config_hash"`
	GoModHash     string                   `json:"go_mod_hash"`
	ScanResult    *ScanResult              `json:"scan_result"`
	AnalyzedFiles map[string]*AnalyzedFile `json:"analyzed_files"`
	Modules       []DetectedModule         `json:"modules"`
	BaselineScore *Score                   `json:"baseline_score"`
}

func (c *ProjectCache) IsInvalidated(goModHash, configHash string) bool {
	return c.GoModHash != goModHash || c.ConfigHash != configHash
}
