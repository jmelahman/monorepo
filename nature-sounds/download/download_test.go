package download

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFileWithProgress(t *testing.T) {
	// Create test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("test data")); err != nil {
			t.Errorf("Error writing test data: %v", err)
		}
	}))
	defer ts.Close()

	// Create temp directory
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "testfile")

	// Test download
	err := FileWithProgress(ts.URL, testFile)
	if err != nil {
		t.Fatalf("FileWithProgress failed: %v", err)
	}

	// Verify file exists and has correct content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	if string(content) != "test data" {
		t.Errorf("Expected 'test data', got '%s'", content)
	}
}

func TestFileWithProgress_BadURL(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "testfile")

	err := FileWithProgress("http://invalid.url", testFile)
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
}
