package scanner

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/openkraft/openkraft/internal/domain"
)

var skipDirs = map[string]bool{
	"vendor":       true,
	"node_modules": true,
	".git":         true,
	"dist":         true,
	"bin":          true,
	"testdata":     true,
}

// FileScanner implements domain.ProjectScanner by walking the filesystem.
type FileScanner struct{}

func New() *FileScanner {
	return &FileScanner{}
}

func (s *FileScanner) Scan(projectPath string, excludePaths ...string) (*domain.ScanResult, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, err
	}

	// Merge extra excludes with built-in skip dirs.
	extraSkip := make(map[string]bool, len(excludePaths))
	for _, p := range excludePaths {
		extraSkip[strings.TrimSuffix(p, "/")] = true
	}

	result := &domain.ScanResult{
		RootPath: absPath,
	}

	err = filepath.WalkDir(absPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if skipDirs[d.Name()] || extraSkip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, _ := filepath.Rel(absPath, path)
		result.AllFiles = append(result.AllFiles, relPath)

		// Detect root-level marker files (only in project root, not subdirs)
		dir := filepath.Dir(relPath)
		isRoot := dir == "."

		switch {
		case d.Name() == "go.mod" && isRoot:
			result.HasGoMod = true
			result.Language = "go"
		case d.Name() == "CLAUDE.md" && isRoot:
			result.HasClaudeMD = true
		case d.Name() == ".cursorrules" && isRoot:
			result.HasCursorRules = true
		case d.Name() == "AGENTS.md" && isRoot:
			result.HasAgentsMD = true
		case d.Name() == ".github" || strings.HasPrefix(relPath, ".github/"):
			result.HasCIConfig = true
		}

		if strings.HasSuffix(d.Name(), ".go") {
			result.GoFiles = append(result.GoFiles, relPath)
			if strings.HasSuffix(d.Name(), "_test.go") {
				result.TestFiles = append(result.TestFiles, relPath)
			}
		}

		if d.Name() == ".openkraft" || strings.HasPrefix(relPath, ".openkraft/") {
			result.HasOpenKraftDir = true
		}

		return nil
	})

	return result, err
}
