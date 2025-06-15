package main

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"
)

// TestStats tests the Stats struct
func TestStats(t *testing.T) {
	stats := Stats{
		TotalFiles: 10,
		Uploaded:   8,
		Deleted:    7,
		Errors:     1,
		Skipped:    2,
	}

	// Test initial values
	if stats.TotalFiles != 10 {
		t.Errorf("Expected TotalFiles to be 10, got %d", stats.TotalFiles)
	}
	if stats.Uploaded != 8 {
		t.Errorf("Expected Uploaded to be 8, got %d", stats.Uploaded)
	}
	if stats.Deleted != 7 {
		t.Errorf("Expected Deleted to be 7, got %d", stats.Deleted)
	}
	if stats.Errors != 1 {
		t.Errorf("Expected Errors to be 1, got %d", stats.Errors)
	}
	if stats.Skipped != 2 {
		t.Errorf("Expected Skipped to be 2, got %d", stats.Skipped)
	}
}

// TestLogSummary tests the logSummary method
func TestLogSummary(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	// Create BackupTool with test data
	bt := &BackupTool{
		logger:     logger,
		cutoffTime: time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC),
		stats: Stats{
			TotalFiles: 5,
			Uploaded:   4,
			Deleted:    4,
			Skipped:    1,
			Errors:     0,
		},
	}

	// Call logSummary
	bt.logSummary("*YYYYMMDD.log.gz")

	// Check output
	output := buf.String()
	expectedLines := []string{
		"=== Backup Summary ===",
		"Glob pattern: *YYYYMMDD.log.gz",
		"Cutoff time: 2024-11-01 00:00:00",
		"Total files: 5",
		"Uploaded: 4",
		"Deleted: 4",
		"Skipped: 1",
		"Errors: 0",
	}

	for _, expected := range expectedLines {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected log output to contain '%s', but it didn't. Output: %s", expected, output)
		}
	}
}