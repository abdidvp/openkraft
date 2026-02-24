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

// ModuleDetector detects module boundaries from scan results.
type ModuleDetector interface {
	Detect(scan *ScanResult) ([]DetectedModule, error)
}

// DetectedModule represents a module found in the project.
type DetectedModule struct {
	Name   string   `json:"name"`
	Path   string   `json:"path"`
	Layers []string `json:"layers"`
	Files  []string `json:"files"`
}

// CodeAnalyzer parses source files and extracts structural information.
type CodeAnalyzer interface {
	AnalyzeFile(filePath string) (*AnalyzedFile, error)
}

// AnalyzedFile holds the structural analysis of a single source file.
type AnalyzedFile struct {
	Path       string     `json:"path"`
	Package    string     `json:"package"`
	Structs    []string   `json:"structs,omitempty"`
	Functions  []Function `json:"functions,omitempty"`
	Interfaces []string   `json:"interfaces,omitempty"`
	Imports    []string   `json:"imports,omitempty"`
}

// Function represents a function or method extracted from source.
type Function struct {
	Name     string `json:"name"`
	Receiver string `json:"receiver,omitempty"`
}
