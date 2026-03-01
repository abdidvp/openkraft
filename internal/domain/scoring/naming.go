package scoring

import (
	"math"
	"strings"
	"unicode"

	"github.com/fatih/camelcase"

	"github.com/abdidvp/openkraft/internal/domain"
)

// vagueWords are generic function name words that reduce discoverability.
var vagueWords = map[string]bool{
	"Handle": true, "Process": true, "Data": true, "Run": true,
	"Do": true, "Execute": true, "Manage": true, "Util": true,
	"Helper": true, "Info": true, "Stuff": true, "Thing": true,
	"Item": true, "Object": true, "Temp": true,
}

// WordCountScore returns a score [0,1] based on the number of CamelCase words
// in a function name. 2-4 words is optimal.
func WordCountScore(name string) float64 {
	words := camelcase.Split(name)
	n := len(words)
	switch {
	case n >= 2 && n <= 4:
		return 1.0
	case n == 1:
		return 0.5
	case n == 5:
		return 0.7
	default: // 0 or >5
		return 0.5
	}
}

// VocabularySpecificity returns the ratio of non-vague words in a name.
func VocabularySpecificity(name string) float64 {
	words := camelcase.Split(name)
	if len(words) == 0 {
		return 0
	}
	nonVague := 0
	for _, w := range words {
		// Capitalize first letter manually to avoid deprecated strings.Title.
		low := strings.ToLower(w)
		titled := strings.ToUpper(low[:1]) + low[1:]
		if !vagueWords[titled] {
			nonVague++
		}
	}
	return float64(nonVague) / float64(len(words))
}

// ShannonEntropy computes the Shannon entropy of a set of names, normalised
// to [0,1]. Higher values indicate more diverse (unique) naming.
func ShannonEntropy(names []string) float64 {
	if len(names) <= 1 {
		return 0
	}

	freq := make(map[string]int)
	for _, n := range names {
		freq[n]++
	}

	total := float64(len(names))
	entropy := 0.0
	for _, count := range freq {
		p := float64(count) / total
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}

	// Normalise by max possible entropy.
	maxEntropy := math.Log2(total)
	if maxEntropy == 0 {
		return 0
	}
	normalised := entropy / maxEntropy
	if normalised > 1.0 {
		normalised = 1.0
	}
	return normalised
}

// hasVerbNounPattern checks if an exported function name has at least 2
// CamelCase words (verb + noun). Go naming conventions dictate that exported
// function names naturally use verb+noun structure (CreateUser, ScoreProject,
// AnalyzeFile). Single-word names (String, Error, Len) don't qualify.
func hasVerbNounPattern(name string) bool {
	if len(name) == 0 || !unicode.IsUpper(rune(name[0])) {
		return false
	}
	return len(camelcase.Split(name)) >= 2
}

// HasVerbNounPattern exports hasVerbNounPattern for testing.
var HasVerbNounPattern = hasVerbNounPattern

// WordCount returns the number of CamelCase words in a name.
func WordCount(name string) int {
	return len(camelcase.Split(name))
}

// genericWords score 0.0 — fully generic identifiers.
var genericWords = map[string]bool{
	"Get": true, "Set": true, "Do": true, "Run": true,
	"Handle": true, "Process": true, "Execute": true, "Make": true,
	"Data": true, "Info": true, "Item": true, "Object": true,
	"Thing": true, "Stuff": true, "Temp": true, "Manager": true,
	"Handler": true, "Helper": true, "Util": true,
}

// actionWords score 0.5 — verbs with clear semantics.
var actionWords = map[string]bool{
	"Validate": true, "Parse": true, "Format": true, "Convert": true,
	"Transform": true, "Compute": true, "Calculate": true, "Build": true,
	"Render": true,
}

// IdentifierSpecificity scores a function name based on word specificity.
// Generic words = 0.0, action words = 0.5, domain vocab = 1.0, unknown = 0.75.
func IdentifierSpecificity(name string, domainVocab map[string]bool) float64 {
	words := camelcase.Split(name)
	if len(words) == 0 {
		return 0
	}
	var total float64
	for _, w := range words {
		titled := titleCase(w)
		switch {
		case genericWords[titled]:
			total += 0.0
		case actionWords[titled]:
			total += 0.5
		case domainVocab[titled]:
			total += 1.0
		default:
			total += 0.75
		}
	}
	return total / float64(len(words))
}

// ExtractDomainVocabulary builds a set of words found in struct and interface
// names across the project, split by CamelCase boundaries.
func ExtractDomainVocabulary(analyzed map[string]*domain.AnalyzedFile) map[string]bool {
	vocab := make(map[string]bool)
	for _, af := range analyzed {
		if af.IsGenerated {
			continue
		}
		for _, s := range af.Structs {
			for _, w := range camelcase.Split(s) {
				vocab[titleCase(w)] = true
			}
		}
		for _, iface := range af.Interfaces {
			for _, w := range camelcase.Split(iface) {
				vocab[titleCase(w)] = true
			}
		}
	}
	return vocab
}

// SymbolCollisionRate returns the fraction of exported function names that
// appear in 2+ packages. Generated files are excluded.
func SymbolCollisionRate(analyzed map[string]*domain.AnalyzedFile) float64 {
	// Group exported function names by package.
	type nameInfo struct {
		packages map[string]bool
	}
	names := make(map[string]*nameInfo)
	totalNames := 0

	for _, af := range analyzed {
		if af.IsGenerated || strings.HasSuffix(af.Path, "_test.go") {
			continue
		}
		for _, fn := range af.Functions {
			if !fn.Exported || fn.Receiver != "" {
				continue
			}
			totalNames++
			ni, ok := names[fn.Name]
			if !ok {
				ni = &nameInfo{packages: make(map[string]bool)}
				names[fn.Name] = ni
			}
			ni.packages[af.Package] = true
		}
	}

	if totalNames == 0 {
		return 0
	}

	collisions := 0
	for _, ni := range names {
		if len(ni.packages) >= 2 {
			collisions++
		}
	}

	return float64(collisions) / float64(len(names))
}

// titleCase returns a word with the first letter uppercased.
func titleCase(w string) string {
	if len(w) == 0 {
		return w
	}
	low := strings.ToLower(w)
	return strings.ToUpper(low[:1]) + low[1:]
}
