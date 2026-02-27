package domain

type OnboardReport struct {
	ProjectName       string             `json:"project_name"`
	ProjectType       string             `json:"project_type"`
	ArchitectureStyle string             `json:"architecture_style"`
	LayoutStyle       ArchLayout         `json:"layout_style"`
	Modules           []DetectedModule   `json:"modules"`
	NamingConvention  string             `json:"naming_convention"`
	NamingPercentage  float64            `json:"naming_percentage"`
	GoldenModule      string             `json:"golden_module"`
	ModuleBlueprint   []string           `json:"module_blueprint"`
	BuildCommands     []string           `json:"build_commands"`
	TestCommands      []string           `json:"test_commands"`
	DependencyRules   []DependencyRule   `json:"dependency_rules"`
	Interfaces        []InterfaceMapping `json:"interfaces"`
	Norms             ProjectNorms       `json:"norms"`
}

type DependencyRule struct {
	Source  string `json:"source"`
	Forbids string `json:"forbids"`
	Reason  string `json:"reason"`
}

type InterfaceMapping struct {
	Interface      string `json:"interface"`
	Implementation string `json:"implementation"`
	Package        string `json:"package"`
}
