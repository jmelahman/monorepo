package history

import (
	"testing"
)

// MockHistoryReader is a mock implementation for testing
type MockHistoryReader struct {
	subtrees map[string]SubtreeHistoryInfo
	err      error
}

func (m *MockHistoryReader) GetSubtreesFromHistory() (map[string]SubtreeHistoryInfo, error) {
	return m.subtrees, m.err
}

func TestMockHistoryReader(t *testing.T) {
	mockSubtrees := map[string]SubtreeHistoryInfo{
		"vendor/example": {
			Prefix:      "vendor/example",
			LastCommit:  "abc123",
			LastMessage: "Add 'vendor/example/' from commit 'def456'",
		},
	}

	mock := &MockHistoryReader{
		subtrees: mockSubtrees,
		err:      nil,
	}

	subtrees, err := mock.GetSubtreesFromHistory()

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(subtrees) != 1 {
		t.Errorf("Expected 1 subtree, got %d", len(subtrees))
	}

	info, exists := subtrees["vendor/example"]
	if !exists {
		t.Error("Expected subtree 'vendor/example' to exist")
	}

	if info.LastCommit != "abc123" {
		t.Errorf("Expected last commit 'abc123', got '%s'", info.LastCommit)
	}
}
