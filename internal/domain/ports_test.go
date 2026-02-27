package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScanResult_AddFile_GoFile(t *testing.T) {
	s := &ScanResult{}
	s.AddFile("foo.go")
	assert.Contains(t, s.GoFiles, "foo.go")
	assert.Contains(t, s.AllFiles, "foo.go")
	assert.NotContains(t, s.TestFiles, "foo.go")
}

func TestScanResult_AddFile_TestFile(t *testing.T) {
	s := &ScanResult{}
	s.AddFile("foo_test.go")
	assert.Contains(t, s.GoFiles, "foo_test.go")
	assert.Contains(t, s.TestFiles, "foo_test.go")
	assert.Contains(t, s.AllFiles, "foo_test.go")
}

func TestScanResult_AddFile_NonGoFile(t *testing.T) {
	s := &ScanResult{}
	s.AddFile("readme.md")
	assert.Contains(t, s.AllFiles, "readme.md")
	assert.Empty(t, s.GoFiles)
	assert.Empty(t, s.TestFiles)
}

func TestScanResult_RemoveFile(t *testing.T) {
	s := &ScanResult{
		GoFiles:   []string{"foo.go", "bar.go"},
		TestFiles: []string{"foo_test.go"},
		AllFiles:  []string{"foo.go", "bar.go", "foo_test.go"},
	}
	s.RemoveFile("foo.go")
	assert.NotContains(t, s.GoFiles, "foo.go")
	assert.NotContains(t, s.AllFiles, "foo.go")
	assert.Contains(t, s.GoFiles, "bar.go")
}

func TestScanResult_RemoveFile_NonExistent(t *testing.T) {
	s := &ScanResult{
		GoFiles:  []string{"foo.go"},
		AllFiles: []string{"foo.go"},
	}
	s.RemoveFile("nonexistent.go") // should not panic
	assert.Contains(t, s.GoFiles, "foo.go")
}

func TestScanResult_AddFile_Duplicate(t *testing.T) {
	s := &ScanResult{}
	s.AddFile("foo.go")
	s.AddFile("foo.go")
	assert.Len(t, s.GoFiles, 1)
	assert.Len(t, s.AllFiles, 1)
}
