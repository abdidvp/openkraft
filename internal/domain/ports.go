package domain

import "strings"

// ProjectScanner scans a project directory and returns file metadata.
type ProjectScanner interface {
	Scan(projectPath string, excludePaths ...string) (*ScanResult, error)
}

// ArchLayout describes the project's architectural layout.
type ArchLayout string

const (
	// LayoutPerFeature is internal/{feature}/{layer}/ (e.g., internal/payments/domain/).
	LayoutPerFeature ArchLayout = "per-feature"
	// LayoutCrossCutting is internal/{layer}/{feature}/ (e.g., internal/domain/scoring/).
	LayoutCrossCutting ArchLayout = "cross-cutting"
)

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
	HasOpenKraftDir        bool   `json:"has_openkraft_dir"`
	HasCIConfig            bool   `json:"has_ci_config"`
	HasCopilotInstructions bool   `json:"has_copilot_instructions"`
	ClaudeMDSize           int    `json:"claude_md_size"`
	ClaudeMDContent        string `json:"-"`
	AgentsMDSize           int    `json:"agents_md_size"`
	CursorRulesSize        int    `json:"cursor_rules_size"`
	ReadmeSize             int        `json:"readme_size"`
	Layout                 ArchLayout `json:"layout"`
}

// AddFile adds a file path to the appropriate file lists.
// Go files (.go) are added to GoFiles; test files (_test.go) also to TestFiles.
// All files are added to AllFiles. Duplicates are avoided.
func (s *ScanResult) AddFile(path string) {
	if containsStr(s.AllFiles, path) {
		return
	}
	s.AllFiles = append(s.AllFiles, path)

	if strings.HasSuffix(path, ".go") {
		if !containsStr(s.GoFiles, path) {
			s.GoFiles = append(s.GoFiles, path)
		}
		if strings.HasSuffix(path, "_test.go") {
			if !containsStr(s.TestFiles, path) {
				s.TestFiles = append(s.TestFiles, path)
			}
		}
	}
}

// RemoveFile removes a file path from all file lists.
func (s *ScanResult) RemoveFile(path string) {
	s.GoFiles = removeStr(s.GoFiles, path)
	s.TestFiles = removeStr(s.TestFiles, path)
	s.AllFiles = removeStr(s.AllFiles, path)
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func removeStr(slice []string, s string) []string {
	result := slice[:0]
	for _, v := range slice {
		if v != s {
			result = append(result, v)
		}
	}
	return result
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
	Path           string       `json:"path"`
	Package        string       `json:"package"`
	Structs        []string     `json:"structs,omitempty"`
	Functions      []Function   `json:"functions,omitempty"`
	Interfaces     []string       `json:"interfaces,omitempty"`
	InterfaceDefs  []InterfaceDef `json:"interface_defs,omitempty"`
	Imports        []string     `json:"imports,omitempty"`
	PackageDoc     bool         `json:"package_doc,omitempty"`
	InitFunctions  int          `json:"init_functions,omitempty"`
	GlobalVars     []string     `json:"global_vars,omitempty"`
	ErrorCalls     []ErrorCall  `json:"error_calls,omitempty"`
	TypeAssertions []TypeAssert `json:"type_assertions,omitempty"`
	TotalLines       int          `json:"total_lines,omitempty"`
	NormalizedTokens []int        `json:"-"`
	IsGenerated      bool         `json:"is_generated,omitempty"`
	HasCGoImport   bool         `json:"has_cgo_import,omitempty"`
}

// Function represents a function or method extracted from source.
type Function struct {
	Name               string   `json:"name"`
	Receiver           string   `json:"receiver,omitempty"`
	Exported           bool     `json:"exported"`
	LineStart          int      `json:"line_start"`
	LineEnd            int      `json:"line_end"`
	Params             []Param  `json:"params,omitempty"`
	Returns            []string `json:"returns,omitempty"`
	MaxNesting         int      `json:"max_nesting"`
	MaxCondOps          int      `json:"max_cond_ops"`
	CognitiveComplexity int      `json:"cognitive_complexity,omitempty"`
	StringLiteralRatio  float64  `json:"string_literal_ratio,omitempty"`
	MaxCaseArms        int      `json:"max_case_arms,omitempty"`
	AvgCaseLines       float64  `json:"avg_case_lines,omitempty"`
}

// Param represents a function parameter.
type Param struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ErrorCall represents an error creation call found in source.
type ErrorCall struct {
	Type       string `json:"type"`       // "fmt.Errorf" or "errors.New"
	HasWrap    bool   `json:"has_wrap"`    // contains %w
	HasContext bool   `json:"has_context"` // has variable interpolation
	Format     string `json:"format"`      // the format string literal
}

// InterfaceDef represents an interface with its method signatures.
type InterfaceDef struct {
	Name    string   `json:"name"`
	Methods []string `json:"methods"` // method names
}

// TypeAssert represents a type assertion found in source.
type TypeAssert struct {
	Safe bool `json:"safe"` // true if comma-ok pattern (v, ok := x.(T))
}

// GitInfo provides git metadata for the current project.
type GitInfo interface {
	CommitHash(projectPath string) (string, error)
	IsGitRepo(projectPath string) bool
}

// ScoreHistory persists and retrieves historical scores.
type ScoreHistory interface {
	Save(projectPath string, entry ScoreEntry) error
	Load(projectPath string) ([]ScoreEntry, error)
}

// ConfigLoader loads project configuration from the project directory.
type ConfigLoader interface {
	Load(projectPath string) (ProjectConfig, error)
}

// CacheStore persists and retrieves project analysis caches.
type CacheStore interface {
	Load(projectPath string) (*ProjectCache, error)
	Save(cache *ProjectCache) error
	Invalidate(projectPath string) error
}

// ScoreEntry represents a single historical score record.
type ScoreEntry struct {
	Timestamp  string `json:"timestamp"`
	CommitHash string `json:"commit_hash,omitempty"`
	Overall    int    `json:"overall"`
	Grade      string `json:"grade"`
}
