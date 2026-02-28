package scoring

import (
	"math"
	"strings"
	"unicode"

	"github.com/fatih/camelcase"
)

// vagueWords are generic function name words that reduce discoverability.
var vagueWords = map[string]bool{
	"Handle":  true, "Process": true, "Data": true, "Run": true,
	"Do":      true, "Execute": true, "Manage": true, "Util": true,
	"Helper":  true, "Info": true, "Stuff": true, "Thing": true,
	"Item":    true, "Object": true, "Temp": true,
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
		return 0.3
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

