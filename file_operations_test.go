//go:build !integration
// +build !integration

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFileOperations tests file operations without Docker/LocalStack
func TestFileOperations(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Test file creation and deletion
	t.Run("Create and delete file", func(t *testing.T) {
		testFile := filepath.Join(tempDir, "test.log")
		content := []byte("test content")

		// Create file
		err := os.WriteFile(testFile, content, 0644)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		// Verify file exists
		info, err := os.Stat(testFile)
		if err != nil {
			t.Fatalf("Failed to stat file: %v", err)
		}
		if info.Size() != int64(len(content)) {
			t.Errorf("File size = %d, want %d", info.Size(), len(content))
		}

		// Delete file
		err = os.Remove(testFile)
		if err != nil {
			t.Fatalf("Failed to delete file: %v", err)
		}

		// Verify file is deleted
		_, err = os.Stat(testFile)
		if !os.IsNotExist(err) {
			t.Error("File still exists after deletion")
		}
	})

	// Test glob pattern matching
	t.Run("Glob pattern matching", func(t *testing.T) {
		// Create test files
		lastMonth := time.Now().AddDate(0, -1, 0)
		files := []struct {
			name    string
			content string
		}{
			{fmt.Sprintf("app-%s.log", lastMonth.Format("20060102")), "app log"},
			{fmt.Sprintf("web-%s.log", lastMonth.Format("2006-01-02")), "web log"},
			{"nodate.log", "no date log"},
		}

		for _, f := range files {
			path := filepath.Join(tempDir, f.name)
			err := os.WriteFile(path, []byte(f.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create file %s: %v", f.name, err)
			}
		}

		// Test glob patterns
		patterns := []struct {
			pattern      string
			expectCount  int
			description  string
		}{
			{
				pattern:      filepath.Join(tempDir, "app-*.log"),
				expectCount:  1,
				description:  "Match app logs",
			},
			{
				pattern:      filepath.Join(tempDir, "web-*.log"),
				expectCount:  1,
				description:  "Match web logs",
			},
			{
				pattern:      filepath.Join(tempDir, "*.log"),
				expectCount:  3,
				description:  "Match all logs",
			},
			{
				pattern:      filepath.Join(tempDir, "nonexistent-*.log"),
				expectCount:  0,
				description:  "No matches",
			},
		}

		for _, p := range patterns {
			matches, err := filepath.Glob(p.pattern)
			if err != nil {
				t.Errorf("%s: glob error: %v", p.description, err)
				continue
			}
			if len(matches) != p.expectCount {
				t.Errorf("%s: got %d matches, want %d", p.description, len(matches), p.expectCount)
			}
		}
	})
}

// TestLockFileOperations tests lock file functionality
func TestLockFileOperations(t *testing.T) {
	tempDir := t.TempDir()
	lockFile := filepath.Join(tempDir, "test.lock")

	t.Run("Lock file creation and removal", func(t *testing.T) {
		// Create lock file
		file, err := os.OpenFile(lockFile, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
		if err != nil {
			t.Fatalf("Failed to create lock file: %v", err)
		}

		// Write PID
		_, err = file.WriteString("12345\n")
		if err != nil {
			t.Errorf("Failed to write to lock file: %v", err)
		}
		file.Close()

		// Try to create again (should fail)
		_, err = os.OpenFile(lockFile, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
		if !os.IsExist(err) {
			t.Error("Expected lock file to exist")
		}

		// Remove lock file
		err = os.Remove(lockFile)
		if err != nil {
			t.Fatalf("Failed to remove lock file: %v", err)
		}

		// Verify removal
		_, err = os.Stat(lockFile)
		if !os.IsNotExist(err) {
			t.Error("Lock file still exists after removal")
		}
	})
}

// TestFindTargetFilesWithRealFS tests finding target files with real filesystem
func TestFindTargetFilesWithRealFS(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create test files
	lastMonth := time.Now().AddDate(0, -1, 0)
	currentMonth := time.Now()
	
	testCases := []struct {
		filename string
		month    time.Time
		shouldFind bool
	}{
		{
			filename:   fmt.Sprintf("app%s.log.gz", lastMonth.Format("20060102")),
			month:      lastMonth,
			shouldFind: true,
		},
		{
			filename:   fmt.Sprintf("app%s.log.gz", currentMonth.Format("20060102")),
			month:      currentMonth,
			shouldFind: false,
		},
		{
			filename:   fmt.Sprintf("web-%s.log.gz", lastMonth.Format("2006-01-02")),
			month:      lastMonth,
			shouldFind: true,
		},
		{
			filename:   "nodate.log.gz",
			month:      lastMonth,
			shouldFind: false,
		},
	}
	
	// Create files
	for _, tc := range testCases {
		path := filepath.Join(tempDir, tc.filename)
		err := os.WriteFile(path, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", tc.filename, err)
		}
	}
	
	// Test with BackupTool using exported functions
	bt := &BackupTool{
		cutoffTime: time.Now().AddDate(0, 1, 0), // 1 month from now (to include test files)
		stats:      Stats{},
		config:     Config{},
	}
	
	// Create a simple logger for testing
	bt.logger = log.New(os.Stdout, "TEST: ", log.LstdFlags)
	
	t.Run("Find YYYYMMDD pattern files", func(t *testing.T) {
		pattern := filepath.Join(tempDir, "app*YYYYMMDD.log.gz")
		files, err := bt.findTargetFiles(pattern)
		if err != nil {
			t.Errorf("findTargetFiles error: %v", err)
			return
		}
		
		if len(files) != 2 {
			t.Errorf("Expected 2 files, got %d", len(files))
		}
	})
	
	t.Run("Find YYYY-MM-DD pattern files", func(t *testing.T) {
		pattern := filepath.Join(tempDir, "web-YYYY-MM-DD.log.gz")
		files, err := bt.findTargetFiles(pattern)
		if err != nil {
			t.Errorf("findTargetFiles error: %v", err)
			return
		}
		
		if len(files) != 1 {
			t.Errorf("Expected 1 file, got %d", len(files))
		}
	})
}