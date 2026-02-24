package domain

// ProjectScanner scans a project directory and returns file metadata.
type ProjectScanner interface {
	Scan(projectPath string) (*ScanResult, error)
}

// ScanResult holds the result of scanning a project directory.
type ScanResult struct {
	RootPath        string   `json:"root_path"`
	Language        string   `json:"language"`
	GoFiles         []string `json:"go_files"`
	TestFiles       []string `json:"test_files"`
	AllFiles        []string `json:"all_files"`
	HasGoMod        bool     `json:"has_go_mod"`
	HasClaudeMD     bool     `json:"has_claude_md"`
	HasCursorRules  bool     `json:"has_cursor_rules"`
	HasAgentsMD     bool     `json:"has_agents_md"`
	HasOpenKraftDir bool     `json:"has_openkraft_dir"`
	HasCIConfig     bool     `json:"has_ci_config"`
}
