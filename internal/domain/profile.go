package domain

// ScoringProfile carries all parameters that scorers need.
// Built from project-type defaults merged with user overrides.
type ScoringProfile struct {
	// Structure
	ExpectedLayers       []string
	ExpectedDirs         []string
	LayerAliases         map[string]string
	ExpectedFileSuffixes []string
	NamingConvention     string // "auto", "bare", "suffixed"

	// Code Health
	MaxFunctionLines  int
	MaxFileLines      int
	MaxNestingDepth   int
	MaxParameters     int
	MaxConditionalOps      int
	MaxCognitiveComplexity int
	MaxDuplicationPercent  int
	MinCloneTokens         int
	ExemptParamPatterns    []string

	// Template function detection: functions whose body is dominated by
	// string literals (e.g., shell completion scripts) receive relaxed
	// size thresholds to avoid false positives.
	StringLiteralThreshold    float64 // ratio above which a function is "template" (default 0.8)
	TemplateFuncSizeMultiplier int    // size limit multiplier for template functions (default 5)

	// CGo/FFI: files with import "C" get a relaxed parameter threshold
	// since wrapper functions must match C API signatures.
	CGoParamThreshold int // max params for CGo wrapper functions (default 12)

	// Context Quality
	ContextFiles []ContextFileSpec

	// Verifiability
	MinTestRatio float64

	// Predictability
	MaxGlobalVarPenalty int
}

// ContextFileSpec describes an AI context file to check during scoring.
type ContextFileSpec struct {
	Name    string `yaml:"name"     json:"name"`
	Points  int    `yaml:"points"   json:"points"`
	MinSize int    `yaml:"min_size" json:"min_size,omitempty"`
}

// DefaultProfile returns the base scoring profile with sensible Go defaults.
func DefaultProfile() ScoringProfile {
	return ScoringProfile{
		ExpectedLayers: []string{"domain", "application", "adapters"},
		ExpectedDirs:   []string{"internal", "cmd"},
		LayerAliases: map[string]string{
			"adapter":        "adapters",
			"infra":          "adapters",
			"infrastructure": "adapters",
			"app":            "application",
			"core":           "application",
		},
		ExpectedFileSuffixes: []string{
			"_model", "_service", "_handler", "_repository",
			"_ports", "_errors", "_routes", "_rule",
		},
		NamingConvention: "auto",
		MaxFunctionLines: 50,
		MaxFileLines:     300,
		MaxNestingDepth:  3,
		MaxParameters:    4,
		MaxConditionalOps:          2,
		MaxCognitiveComplexity:     25,
		MaxDuplicationPercent:      15,
		MinCloneTokens:             75,
		ExemptParamPatterns:        []string{"Reconstruct"},
		StringLiteralThreshold:     0.8,
		TemplateFuncSizeMultiplier: 5,
		CGoParamThreshold:         12,
		ContextFiles: []ContextFileSpec{
			{Name: "CLAUDE.md", Points: 10, MinSize: 500},
			{Name: "AGENTS.md", Points: 8},
			{Name: ".cursorrules", Points: 7, MinSize: 200},
			{Name: ".github/copilot-instructions.md", Points: 5},
		},
		MinTestRatio:        0.5,
		MaxGlobalVarPenalty: 3,
	}
}

// DefaultProfileForType returns a scoring profile tuned for a specific project type.
func DefaultProfileForType(pt ProjectType) ScoringProfile {
	p := DefaultProfile()

	switch pt {
	case ProjectTypeCLI:
		p.ExpectedLayers = []string{"domain", "application"}
		p.ExpectedFileSuffixes = []string{"_model", "_service"}
		p.ContextFiles = []ContextFileSpec{
			{Name: "CLAUDE.md", Points: 15, MinSize: 500},
			{Name: ".cursorrules", Points: 7, MinSize: 200},
			{Name: ".github/copilot-instructions.md", Points: 8},
		}

	case ProjectTypeLibrary:
		p.ExpectedLayers = []string{"domain"}
		p.ExpectedDirs = []string{"pkg"}
		p.ExpectedFileSuffixes = []string{"_model", "_errors"}
		p.MaxFunctionLines = 40
		p.MaxFileLines = 250
		p.MaxParameters = 3
		p.MaxCognitiveComplexity = 20
		p.MinTestRatio = 0.8
		p.ContextFiles = []ContextFileSpec{
			{Name: "CLAUDE.md", Points: 12, MinSize: 500},
			{Name: "AGENTS.md", Points: 8},
			{Name: ".github/copilot-instructions.md", Points: 10},
		}

	case ProjectTypeMicroservice:
		p.ExpectedFileSuffixes = []string{
			"_model", "_service", "_handler", "_repository",
		}
		p.ContextFiles = []ContextFileSpec{
			{Name: "CLAUDE.md", Points: 10, MinSize: 500},
			{Name: "AGENTS.md", Points: 8},
			{Name: ".cursorrules", Points: 7, MinSize: 200},
			{Name: ".github/copilot-instructions.md", Points: 5},
		}

	default: // API or unrecognized — use base defaults
	}

	return p
}
