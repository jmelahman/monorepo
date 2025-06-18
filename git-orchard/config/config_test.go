package config

import (
	"testing"
)

// MockConfigReader is a mock implementation for testing
type MockConfigReader struct {
	subtrees      []SubtreeConfig
	orchardConfig OrchardConfig
	err           error
}

func (m *MockConfigReader) ReadSubtreeConfigs() ([]SubtreeConfig, OrchardConfig, error) {
	return m.subtrees, m.orchardConfig, m.err
}

func TestMockConfigReader(t *testing.T) {
	mockSubtrees := []SubtreeConfig{
		{
			Name:       "test-subtree",
			Repository: "https://github.com/example/repo.git",
			Prefix:     "vendor/example",
			Branch:     "main",
		},
	}

	mockConfig := OrchardConfig{Squash: true}

	mock := &MockConfigReader{
		subtrees:      mockSubtrees,
		orchardConfig: mockConfig,
		err:           nil,
	}

	subtrees, config, err := mock.ReadSubtreeConfigs()

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(subtrees) != 1 {
		t.Errorf("Expected 1 subtree, got %d", len(subtrees))
	}

	if subtrees[0].Name != "test-subtree" {
		t.Errorf("Expected subtree name 'test-subtree', got '%s'", subtrees[0].Name)
	}

	if !config.Squash {
		t.Error("Expected squash to be true")
	}
}
