package main

import (
	"os"
	"testing"
)

// TestNewBackupTool tests the NewBackupTool function
func TestNewBackupTool(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "Valid config with log file",
			config: Config{
				OutputFile:        "/tmp/test-backup.log",
				Period:            "1 month",
				Verbose:           false,
				DeleteAfterUpload: false,
			},
			wantErr: false,
		},
		{
			name: "Valid config without log file",
			config: Config{
				OutputFile:        "",
				Period:            "1 month",
				Verbose:           false,
				DeleteAfterUpload: false,
			},
			wantErr: false,
		},
		{
			name: "Valid config with verbose",
			config: Config{
				OutputFile:        "/tmp/test-backup-verbose.log",
				Period:            "1 month",
				Verbose:           true,
				DeleteAfterUpload: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing log files
			if tt.config.OutputFile != "" {
				defer os.Remove(tt.config.OutputFile)
			}

			tool, err := NewBackupTool(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewBackupTool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tool == nil {
				t.Error("NewBackupTool() returned nil tool without error")
			}
			if !tt.wantErr && tool.logger == nil {
				t.Error("NewBackupTool() returned tool with nil logger")
			}
			if !tt.wantErr && tool.cutoffTime.IsZero() {
				t.Error("NewBackupTool() returned tool with zero cutoffTime")
			}
		})
	}
}

// TestAcquireReleaseLock tests lock file operations
func TestAcquireReleaseLock(t *testing.T) {
	lockFile := "/tmp/test-backup.lock"
	defer os.Remove(lockFile)

	bt := &BackupTool{
		config: Config{
			LockFile: lockFile,
		},
	}

	// Test acquiring lock
	err := bt.acquireLock()
	if err != nil {
		t.Fatalf("acquireLock() failed: %v", err)
	}

	// Test that lock file exists
	if _, err := os.Stat(lockFile); os.IsNotExist(err) {
		t.Error("Lock file was not created")
	}

	// Test acquiring lock again (should fail)
	bt2 := &BackupTool{
		config: Config{
			LockFile: lockFile,
		},
	}
	err = bt2.acquireLock()
	if err == nil {
		t.Error("acquireLock() should have failed when lock already exists")
	}

	// Test releasing lock
	bt.releaseLock()

	// Test that lock file is removed
	if _, err := os.Stat(lockFile); !os.IsNotExist(err) {
		t.Error("Lock file was not removed after release")
	}
}

// TestAcquireLockNoLockFile tests acquireLock with empty lock file
func TestAcquireLockNoLockFile(t *testing.T) {
	bt := &BackupTool{
		config: Config{
			LockFile: "",
		},
	}

	// Should not return error when LockFile is empty
	err := bt.acquireLock()
	if err != nil {
		t.Errorf("acquireLock() with empty LockFile returned error: %v", err)
	}

	// releaseLock should also handle empty LockFile gracefully
	bt.releaseLock() // Should not panic
}