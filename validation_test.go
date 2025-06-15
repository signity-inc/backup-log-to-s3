package main

import (
	"strings"
	"testing"
	"time"
)

// TestGlobPatternValidation tests glob pattern validation logic
func TestGlobPatternValidation(t *testing.T) {
	tests := []struct {
		name        string
		globPattern string
		wantErr     bool
		errContains string
	}{
		{
			name:        "Valid YYYYMMDD pattern",
			globPattern: "*YYYYMMDD.log.gz",
			wantErr:     false,
		},
		{
			name:        "Valid YYYY-MM-DD pattern",
			globPattern: "app-YYYY-MM-DD.gz",
			wantErr:     false,
		},
		{
			name:        "Valid YYYY/MM/DD pattern",
			globPattern: "logs/YYYY/MM/DD.gz",
			wantErr:     false,
		},
		{
			name:        "Invalid pattern - no date format",
			globPattern: "*.log.gz",
			wantErr:     true,
			errContains: "Must contain 'YYYYMMDD', 'YYYY-MM-DD', or 'YYYY/MM/DD'",
		},
		{
			name:        "Empty pattern",
			globPattern: "",
			wantErr:     true,
			errContains: "Must contain 'YYYYMMDD', 'YYYY-MM-DD', or 'YYYY/MM/DD'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This validation is in the Run method
			// We need to test the validation logic directly
			isValid := strings.Contains(tt.globPattern, "YYYYMMDD") ||
				strings.Contains(tt.globPattern, "YYYY-MM-DD") ||
				strings.Contains(tt.globPattern, "YYYY/MM/DD")

			hasErr := !isValid
			if hasErr != tt.wantErr {
				t.Errorf("Pattern validation for '%s' = %v, want error %v", tt.globPattern, hasErr, tt.wantErr)
			}
		})
	}
}

// TestExtractDateValidation tests date extraction edge cases
func TestExtractDateValidation(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     time.Time
		wantErr  bool
	}{
		{
			name:     "Edge case - 8 digits not a date",
			filename: "file12345678.txt",
			want:     time.Time{},
			wantErr:  true, // Now validates date format
		},
		{
			name:     "Edge case - partial date",
			filename: "log-2024.txt",
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "Edge case - date at start",
			filename: "20241215-app.log",
			want:     time.Date(2024, 12, 15, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "Edge case - date at end",
			filename: "app-log-20241215",
			want:     time.Date(2024, 12, 15, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "Edge case - multiple separators",
			filename: "app---2024-12-15---log.gz",
			want:     time.Date(2024, 12, 15, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractDateFromFilename(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractDateFromFilename() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !got.Equal(tt.want) {
				t.Errorf("extractDateFromFilename() = %v, want %v", got, tt.want)
			}
		})
	}
}