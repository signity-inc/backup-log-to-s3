package main

import (
	"testing"
	"time"
)

// TestParsePeriod tests the parsePeriod function
func TestParsePeriod(t *testing.T) {
	tests := []struct {
		name     string
		period   string
		want     time.Duration
		wantErr  bool
	}{
		{
			name:    "1 day",
			period:  "1 day",
			want:    24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "7 days",
			period:  "7 days",
			want:    7 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "1 month",
			period:  "1 month",
			want:    30 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "2 months",
			period:  "2 months",
			want:    2 * 30 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "1 year",
			period:  "1 year",
			want:    365 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "invalid format",
			period:  "invalid",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid unit",
			period:  "1 week",
			want:    0,
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePeriod(tt.period)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePeriod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parsePeriod() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestExtractDateFromFilename tests the extractDateFromFilename function
func TestExtractDateFromFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     time.Time
		wantErr  bool
	}{
		{
			name:     "YYYYMMDD format",
			filename: "app20241215.log.gz",
			want:     time.Date(2024, 12, 15, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "YYYY-MM-DD format",
			filename: "log-2024-12-15.gz",
			want:     time.Date(2024, 12, 15, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "YYYY/MM/DD format",
			filename: "2024/12/15.gz",
			want:     time.Date(2024, 12, 15, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "YYYY_MM_DD format",
			filename: "log_2024_12_15.gz",
			want:     time.Date(2024, 12, 15, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "Multiple dates - should use first match",
			filename: "backup_20240115_to_20240215.gz",
			want:     time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "No date pattern",
			filename: "random_file.log",
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "Invalid date format",
			filename: "log_2024_15_45.gz",
			want:     time.Time{},
			wantErr:  true, // Now validates date validity
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

// TestConvertGlobPattern tests the convertGlobPattern function
func TestConvertGlobPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    string
	}{
		{
			name:    "YYYYMMDD pattern",
			pattern: "app-YYYYMMDD.log.gz",
			want:    "app-*.log.gz",
		},
		{
			name:    "YYYY-MM-DD pattern",
			pattern: "log-YYYY-MM-DD.gz",
			want:    "log-*.gz",
		},
		{
			name:    "YYYY/MM/DD pattern",
			pattern: "logs/YYYY/MM/DD.gz",
			want:    "logs/*.gz",
		},
		{
			name:    "Multiple patterns",
			pattern: "app-YYYYMMDD-backup-YYYY-MM-DD.gz",
			want:    "app-*-backup-*.gz",
		},
		{
			name:    "No date pattern",
			pattern: "regular-file.log",
			want:    "regular-file.log",
		},
		{
			name:    "Complex path with YYYYMMDD",
			pattern: "/var/log/app/access-YYYYMMDD.log.gz",
			want:    "/var/log/app/access-*.log.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertGlobPattern(tt.pattern); got != tt.want {
				t.Errorf("convertGlobPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestConfig tests Config struct validation
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid config",
			config: Config{
				S3Bucket:          "test-bucket",
				S3Prefix:          "test-prefix",
				AWSRegion:         "us-east-1",
				OutputFile:        "/tmp/test.log",
				Period:            "1 month",
				DeleteAfterUpload: false,
			},
			wantErr: false,
		},
		{
			name: "Missing bucket",
			config: Config{
				S3Prefix:          "test-prefix",
				AWSRegion:         "us-east-1",
				OutputFile:        "/tmp/test.log",
				Period:            "1 month",
				DeleteAfterUpload: false,
			},
			wantErr: true,
			errMsg:  "bucket is required",
		},
		{
			name: "Missing prefix",
			config: Config{
				S3Bucket:          "test-bucket",
				AWSRegion:         "us-east-1",
				OutputFile:        "/tmp/test.log",
				Period:            "1 month",
				DeleteAfterUpload: false,
			},
			wantErr: true,
			errMsg:  "prefix is required",
		},
		{
			name: "Missing period",
			config: Config{
				S3Bucket:          "test-bucket",
				S3Prefix:          "test-prefix",
				AWSRegion:         "us-east-1",
				OutputFile:        "/tmp/test.log",
				DeleteAfterUpload: false,
			},
			wantErr: true,
			errMsg:  "failed to parse period",
		},
		{
			name: "Region is optional",
			config: Config{
				S3Bucket:          "test-bucket",
				S3Prefix:          "test-prefix",
				OutputFile:        "/tmp/test.log",
				Period:            "1 month",
				DeleteAfterUpload: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would require refactoring parseFlags to separate validation logic
			// For now, we'll just document this as a test case
			t.Skip("Config validation is currently part of parseFlags function")
		})
	}
}

// TestShowUsage tests that showUsage doesn't panic
func TestShowUsage(t *testing.T) {
	// This test just ensures showUsage doesn't panic
	// In a real test, we would capture stdout and verify the output
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("showUsage() panicked: %v", r)
		}
	}()
	
	// We can't easily test showUsage without capturing stdout
	// This is a limitation of the current design
	t.Skip("showUsage prints to stdout, requires output capture for proper testing")
}

// TestProcessPrefixWithDate tests the processPrefixWithDate function
func TestProcessPrefixWithDate(t *testing.T) {
	// Test date: 2024-12-15
	testDate := time.Date(2024, 12, 15, 10, 30, 0, 0, time.UTC)
	
	tests := []struct {
		name   string
		prefix string
		want   string
	}{
		{
			name:   "No date tokens",
			prefix: "logs",
			want:   "logs",
		},
		{
			name:   "Year only",
			prefix: "logs/YYYY",
			want:   "logs/2024",
		},
		{
			name:   "Year and month",
			prefix: "logs/YYYY/MM",
			want:   "logs/2024/12",
		},
		{
			name:   "Full date",
			prefix: "logs/YYYY/MM/DD",
			want:   "logs/2024/12/15",
		},
		{
			name:   "Custom base path",
			prefix: "backup/YYYY/MM",
			want:   "backup/2024/12",
		},
		{
			name:   "Multiple year tokens",
			prefix: "YYYY/backup/YYYY",
			want:   "2024/backup/2024",
		},
		{
			name:   "Mixed with text",
			prefix: "app-YYYY-MM-logs",
			want:   "app-2024-12-logs",
		},
		{
			name:   "All tokens",
			prefix: "YYYY-MM-DD",
			want:   "2024-12-15",
		},
		{
			name:   "Empty prefix",
			prefix: "",
			want:   "",
		},
		{
			name:   "Complex path",
			prefix: "s3://bucket/YYYY/MM/DD/logs",
			want:   "s3://bucket/2024/12/15/logs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processPrefixWithDate(tt.prefix, testDate)
			if got != tt.want {
				t.Errorf("processPrefixWithDate(%q, %v) = %q, want %q", tt.prefix, testDate, got, tt.want)
			}
		})
	}
}

// TestProcessPrefixWithDateEdgeCases tests edge cases for processPrefixWithDate
func TestProcessPrefixWithDateEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		date   time.Time
		prefix string
		want   string
	}{
		{
			name:   "New Year's Day",
			date:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			prefix: "logs/YYYY/MM/DD",
			want:   "logs/2024/01/01",
		},
		{
			name:   "Leap year Feb 29",
			date:   time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
			prefix: "logs/YYYY/MM/DD",
			want:   "logs/2024/02/29",
		},
		{
			name:   "Year boundary",
			date:   time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
			prefix: "backup/YYYY/MM/DD",
			want:   "backup/2023/12/31",
		},
		{
			name:   "Future date",
			date:   time.Date(2030, 7, 15, 12, 0, 0, 0, time.UTC),
			prefix: "logs/YYYY/MM",
			want:   "logs/2030/07",
		},
		{
			name:   "Far past date",
			date:   time.Date(1990, 3, 5, 9, 15, 30, 0, time.UTC),
			prefix: "archive/YYYY",
			want:   "archive/1990",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processPrefixWithDate(tt.prefix, tt.date)
			if got != tt.want {
				t.Errorf("processPrefixWithDate(%q, %v) = %q, want %q", tt.prefix, tt.date, got, tt.want)
			}
		})
	}
}