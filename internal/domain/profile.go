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
	MaxFunctionLines       int
	MaxFileLines           int
	MaxNestingDepth        int
	MaxParameters          int
	MaxConditionalOps      int
	MaxCognitiveComplexity int
	MaxDuplicationPercent  int
	MinCloneTokens         int
	ExemptParamPatterns    []string

	// Template function detection: functions whose body is dominated by
	// string literals (e.g., shell completion scripts) receive relaxed
	// size thresholds to avoid false positives.
	StringLiteralThreshold     float64 // ratio above which a function is "template" (default 0.8)
	TemplateFuncSizeMultiplier int     // size limit multiplier for template functions (default 5)

	// CGo/FFI: files with import "C" get a relaxed parameter threshold
	// since wrapper functions must match C API signatures.
	CGoParamThreshold int // max params for CGo wrapper functions (default 12)

	// Context Quality
	ContextFiles []ContextFileSpec

	// Verifiability
	MinTestRatio float64

	// Discoverability
	MinNamingWordScore         float64    // WCS threshold for "descriptive" (default: 0.7)
	NamingConsistencyThreshold float64    // min dominant % to flag violations (default: 0.60)
	NamingCompositeWeights     [3]float64 // WCS, specificity, entropy weights (default: {0.30, 0.30, 0.25})
	CollisionWeight            float64    // weight for collision rate signal (default: 0.15)
	StructureCompositeWeights  [3]float64 // layers, suffix, filecount weights (default: {0.5, 0.3, 0.2})

	// Import graph
	CyclePenaltyWeight        float64 // weight of cycle penalty within graph score (default: 0.40)
	MaxDistanceFromMain       float64 // distance threshold above which score decays (default: 0.40)
	CouplingOutlierMultiplier float64 // Ce > multiplier * median = outlier (default: 2.0)
	CompositionRoots          []string // module-relative paths exempt from adapter-to-adapter violations

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
		NamingConvention:           "auto",
		MaxFunctionLines:           50,
		MaxFileLines:               300,
		MaxNestingDepth:            3,
		MaxParameters:              4,
		MaxConditionalOps:          2,
		MaxCognitiveComplexity:     25,
		MaxDuplicationPercent:      15,
		MinCloneTokens:             75,
		ExemptParamPatterns:        []string{"Reconstruct"},
		StringLiteralThreshold:     0.8,
		TemplateFuncSizeMultiplier: 5,
		CGoParamThreshold:          12,
		ContextFiles: []ContextFileSpec{
			{Name: "CLAUDE.md", Points: 10, MinSize: 500},
			{Name: "AGENTS.md", Points: 8},
			{Name: ".cursorrules", Points: 7, MinSize: 200},
			{Name: ".github/copilot-instructions.md", Points: 5},
		},
		MinTestRatio:               0.5,
		MinNamingWordScore:         0.7,
		NamingConsistencyThreshold: 0.60,
		NamingCompositeWeights:     [3]float64{0.30, 0.30, 0.25},
		CollisionWeight:            0.15,
		StructureCompositeWeights:  [3]float64{0.5, 0.3, 0.2},
		CyclePenaltyWeight:        0.40,
		MaxDistanceFromMain:       0.40,
		CouplingOutlierMultiplier: 2.0,
		MaxGlobalVarPenalty:       3,
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
