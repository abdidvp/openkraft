package gitinfo

import (
	"fmt"

	"github.com/go-git/go-git/v5"
)

// GitInfoAdapter implements domain.GitInfo using go-git.
type GitInfoAdapter struct{}

func New() *GitInfoAdapter {
	return &GitInfoAdapter{}
}

func (g *GitInfoAdapter) IsGitRepo(projectPath string) bool {
	_, err := git.PlainOpen(projectPath)
	return err == nil
}

func (g *GitInfoAdapter) CommitHash(projectPath string) (string, error) {
	repo, err := git.PlainOpen(projectPath)
	if err != nil {
		return "", fmt.Errorf("opening git repo: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("getting HEAD: %w", err)
	}

	return head.Hash().String(), nil
}
