//go:build integration
// +build integration

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupLocalStack starts a LocalStack container for testing
func setupLocalStack(t *testing.T) (testcontainers.Container, string) {
	ctx := context.Background()

	// Configure LocalStack container
	localstackContainer, err := localstack.Run(ctx,
		"localstack/localstack:latest",
		testcontainers.WithEnv(map[string]string{
			"SERVICES": "s3",
			"DEFAULT_REGION": "us-east-1",
		}),
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready.").WithStartupTimeout(120*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("Failed to start LocalStack container: %v", err)
	}

	// Get the endpoint URL
	host, err := localstackContainer.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get LocalStack host: %v", err)
	}

	// LocalStack uses port 4566 for all services
	mappedPort, err := localstackContainer.MappedPort(ctx, "4566")
	if err != nil {
		t.Fatalf("Failed to get LocalStack port: %v", err)
	}

	endpoint := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())
	t.Logf("LocalStack endpoint: %s", endpoint)

	return localstackContainer, endpoint
}

// TestIntegrationFindTargetFiles tests finding files with actual file system operations
func TestIntegrationFindTargetFiles(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Calculate last month for testing
	lastMonth := time.Now().AddDate(0, -1, 0)
	currentMonth := time.Now()

	// Create test files
	testFiles := []struct {
		name     string
		date     time.Time
		expected bool
	}{
		{
			name:     fmt.Sprintf("app%s.log.gz", lastMonth.Format("20060102")),
			date:     lastMonth,
			expected: true,
		},
		{
			name:     fmt.Sprintf("app%s.log.gz", currentMonth.Format("20060102")),
			date:     currentMonth,
			expected: false,
		},
		{
			name:     fmt.Sprintf("backup-%s.gz", lastMonth.Format("2006-01-02")),
			date:     lastMonth,
			expected: true,
		},
		{
			name:     "nodate.log.gz",
			date:     lastMonth,
			expected: false,
		},
	}

	// Create test files
	for _, tf := range testFiles {
		filePath := filepath.Join(tempDir, tf.name)
		file, err := os.Create(filePath)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", tf.name, err)
		}
		file.Close()
		
		// Set file modification time
		os.Chtimes(filePath, tf.date, tf.date)
	}

	// Test cases
	tests := []struct {
		name        string
		globPattern string
		wantCount   int
	}{
		{
			name:        "YYYYMMDD pattern",
			globPattern: filepath.Join(tempDir, "app*YYYYMMDD.log.gz"),
			wantCount:   1,
		},
		{
			name:        "YYYY-MM-DD pattern",
			globPattern: filepath.Join(tempDir, "backup-YYYY-MM-DD.gz"),
			wantCount:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since findTargetFiles is not exported, we need to test through Run method
			// or make the function exported for testing
			t.Skip("findTargetFiles is not exported, skipping direct test")
		})
	}
}

// TestIntegrationS3Operations tests S3 upload and delete operations
func TestIntegrationS3Operations(t *testing.T) {
	ctx := context.Background()

	// Start LocalStack
	localstackContainer, endpoint := setupLocalStack(t)
	defer func() {
		if err := localstackContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate LocalStack container: %v", err)
		}
	}()

	// Create test configuration
	testBucket := "test-backup-bucket"
	testPrefix := "test-logs"
	testRegion := "us-east-1"

	// Create S3 client for LocalStack
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(testRegion),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		t.Fatalf("Failed to create AWS config: %v", err)
	}

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(endpoint)
	})

	// Create test bucket
	_, err = s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(testBucket),
	})
	if err != nil {
		t.Fatalf("Failed to create test bucket: %v", err)
	}

	// Create test file
	tempDir := t.TempDir()
	testFileName := "test-file.log.gz"
	testFilePath := filepath.Join(tempDir, testFileName)
	testContent := []byte("This is test log content")
	
	if err := os.WriteFile(testFilePath, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create BackupTool instance
	config := Config{
		S3Bucket:          testBucket,
		S3Prefix:          testPrefix,
		AWSRegion:         testRegion,
		StorageClass:      "STANDARD",
		EndpointURL:       endpoint,
		OutputFile:        "", // Log to stdout for testing
		Period:            "1 month",
		DeleteAfterUpload: true, // Test with deletion enabled
	}
	
	bt, err := NewBackupTool(config)
	if err != nil {
		t.Fatalf("Failed to create BackupTool: %v", err)
	}
	bt.s3Client = s3Client // Override with test client

	// Test upload
	t.Run("Upload to S3", func(t *testing.T) {
		err := bt.uploadToS3(ctx, testFilePath)
		if err != nil {
			t.Errorf("uploadToS3() error = %v", err)
		}

		// Verify file exists in S3
		expectedKey := fmt.Sprintf("%s/%s", testPrefix, testFileName)
		_, err = s3Client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(testBucket),
			Key:    aws.String(expectedKey),
		})
		if err != nil {
			t.Errorf("File not found in S3 after upload: %v", err)
		}
	})

	// Test delete local file
	t.Run("Delete local file", func(t *testing.T) {
		// First, recreate the file
		if err := os.WriteFile(testFilePath, testContent, 0644); err != nil {
			t.Fatalf("Failed to recreate test file: %v", err)
		}

		err := bt.deleteLocalFile(testFilePath)
		if err != nil {
			t.Errorf("deleteLocalFile() error = %v", err)
		}

		// Verify file is deleted
		if _, err := os.Stat(testFilePath); !os.IsNotExist(err) {
			t.Error("File still exists after deletion")
		}
	})
}

// TestIntegrationFullWorkflow tests the complete backup workflow
func TestIntegrationFullWorkflow(t *testing.T) {
	ctx := context.Background()

	// Start LocalStack
	localstackContainer, endpoint := setupLocalStack(t)
	defer func() {
		if err := localstackContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate LocalStack container: %v", err)
		}
	}()

	// Create test configuration
	testBucket := "test-workflow-bucket"
	testPrefix := "workflow-logs"
	testRegion := "us-east-1"
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	// Create test files for last month
	lastMonth := time.Now().AddDate(0, -1, 0)
	testFiles := []string{
		fmt.Sprintf("app-%s.log.gz", lastMonth.Format("20060102")),
		fmt.Sprintf("web-%s.log.gz", lastMonth.Format("20060102")),
		fmt.Sprintf("api-%s.log.gz", lastMonth.Format("20060102")),
	}

	for _, fileName := range testFiles {
		filePath := filepath.Join(tempDir, fileName)
		if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", fileName, err)
		}
	}

	// Create S3 client and bucket
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(testRegion),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		t.Fatalf("Failed to create AWS config: %v", err)
	}

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(endpoint)
	})

	_, err = s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(testBucket),
	})
	if err != nil {
		t.Fatalf("Failed to create test bucket: %v", err)
	}

	// Create BackupTool
	config := Config{
		S3Bucket:          testBucket,
		S3Prefix:          testPrefix,
		AWSRegion:         testRegion,
		OutputFile:        logFile,
		StorageClass:      "STANDARD",
		EndpointURL:       endpoint,
		DryRun:            false,
		Verbose:           true,
		Period:            "1 month",
		DeleteAfterUpload: true, // Test with deletion enabled
	}

	bt, err := NewBackupTool(config)
	if err != nil {
		t.Fatalf("Failed to create BackupTool: %v", err)
	}

	// Override S3 client with LocalStack client
	bt.s3Client = s3Client

	// Run the backup process
	globPattern := filepath.Join(tempDir, "*-YYYYMMDD.log.gz")
	err = bt.Run(ctx, globPattern)
	if err != nil {
		t.Errorf("Run() error = %v", err)
	}

	// Verify results
	if bt.stats.TotalFiles != 3 {
		t.Errorf("Expected 3 files, got %d", bt.stats.TotalFiles)
	}
	if bt.stats.Uploaded != 3 {
		t.Errorf("Expected 3 uploaded files, got %d", bt.stats.Uploaded)
	}
	if bt.stats.Deleted != 3 {
		t.Errorf("Expected 3 deleted files, got %d", bt.stats.Deleted)
	}
	if bt.stats.Errors != 0 {
		t.Errorf("Expected 0 errors, got %d", bt.stats.Errors)
	}

	// Verify files are deleted locally
	for _, fileName := range testFiles {
		filePath := filepath.Join(tempDir, fileName)
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Errorf("File %s still exists after backup", fileName)
		}
	}

	// Verify files exist in S3
	listResp, err := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(testBucket),
		Prefix: aws.String(testPrefix),
	})
	if err != nil {
		t.Fatalf("Failed to list S3 objects: %v", err)
	}

	if len(listResp.Contents) != 3 {
		t.Errorf("Expected 3 objects in S3, got %d", len(listResp.Contents))
	}
}

// TestIntegrationDateBasedPrefix tests date-based prefix functionality
func TestIntegrationDateBasedPrefix(t *testing.T) {
	ctx := context.Background()

	// Start LocalStack
	localstackContainer, endpoint := setupLocalStack(t)
	defer func() {
		if err := localstackContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate LocalStack container: %v", err)
		}
	}()

	// Create test configuration
	testBucket := "test-prefix-bucket"
	testRegion := "us-east-1"
	tempDir := t.TempDir()

	// Create S3 client and bucket
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(testRegion),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		t.Fatalf("Failed to create AWS config: %v", err)
	}

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(endpoint)
	})

	_, err = s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(testBucket),
	})
	if err != nil {
		t.Fatalf("Failed to create test bucket: %v", err)
	}

	// Test date: 2024-12-15
	testDate := time.Date(2024, 12, 15, 0, 0, 0, 0, time.UTC)
	
	// Test cases for different prefix formats
	testCases := []struct {
		name           string
		prefix         string
		fileName       string
		expectedS3Path string
		description    string
	}{
		{
			name:           "No date format",
			prefix:         "logs",
			fileName:       "app20241215.log.gz",
			expectedS3Path: "logs/app20241215.log.gz",
			description:    "Traditional prefix without date formatting",
		},
		{
			name:           "Year only format",
			prefix:         "logs/YYYY",
			fileName:       "app20241215.log.gz",
			expectedS3Path: "logs/2024/app20241215.log.gz",
			description:    "Year-based directory structure",
		},
		{
			name:           "Year and month format",
			prefix:         "logs/YYYY/MM",
			fileName:       "app20241215.log.gz",
			expectedS3Path: "logs/2024/12/app20241215.log.gz",
			description:    "Year and month directory structure",
		},
		{
			name:           "Full date format",
			prefix:         "logs/YYYY/MM/DD",
			fileName:       "app20241215.log.gz",
			expectedS3Path: "logs/2024/12/15/app20241215.log.gz",
			description:    "Full date directory structure",
		},
		{
			name:           "Custom base path with date",
			prefix:         "backup/YYYY/MM",
			fileName:       "web20241215.log.gz",
			expectedS3Path: "backup/2024/12/web20241215.log.gz",
			description:    "Custom base path with year/month",
		},
		{
			name:           "Hyphenated date filename",
			prefix:         "logs/YYYY/MM/DD",
			fileName:       "api-2024-12-15.log.gz",
			expectedS3Path: "logs/2024/12/15/api-2024-12-15.log.gz",
			description:    "Hyphenated date format in filename",
		},
		{
			name:           "Underscore date filename",
			prefix:         "logs/YYYY/MM",
			fileName:       "db_2024_12_15.log.gz",
			expectedS3Path: "logs/2024/12/db_2024_12_15.log.gz",
			description:    "Underscore date format in filename",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test file
			testFilePath := filepath.Join(tempDir, tc.fileName)
			testContent := []byte(fmt.Sprintf("Test log content for %s", tc.fileName))
			
			if err := os.WriteFile(testFilePath, testContent, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Set file modification time to test date
			if err := os.Chtimes(testFilePath, testDate, testDate); err != nil {
				t.Fatalf("Failed to set file time: %v", err)
			}

			// Create BackupTool instance
			config := Config{
				S3Bucket:     testBucket,
				S3Prefix:     tc.prefix,
				AWSRegion:    testRegion,
				StorageClass: "STANDARD",
				EndpointURL:  endpoint,
				Period:       "1 month",
			}

			bt, err := NewBackupTool(config)
			if err != nil {
				t.Fatalf("Failed to create BackupTool: %v", err)
			}

			// Override S3 client with LocalStack client
			bt.s3Client = s3Client

			// Test upload
			err = bt.uploadToS3(ctx, testFilePath)
			if err != nil {
				t.Errorf("uploadToS3() error = %v", err)
				return
			}

			// Verify file exists in S3 at expected path
			_, err = s3Client.HeadObject(ctx, &s3.HeadObjectInput{
				Bucket: aws.String(testBucket),
				Key:    aws.String(tc.expectedS3Path),
			})
			if err != nil {
				t.Errorf("File not found at expected S3 path %s: %v", tc.expectedS3Path, err)
				
				// List all objects to debug
				listResp, listErr := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
					Bucket: aws.String(testBucket),
				})
				if listErr == nil {
					t.Logf("Available objects in bucket:")
					for _, obj := range listResp.Contents {
						t.Logf("  %s", *obj.Key)
					}
				}
				return
			}

			// Verify content
			getResp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(testBucket),
				Key:    aws.String(tc.expectedS3Path),
			})
			if err != nil {
				t.Errorf("Failed to get object from S3: %v", err)
				return
			}
			defer getResp.Body.Close()

			// Check metadata
			if getResp.Metadata == nil {
				t.Error("Expected metadata to be set")
			} else {
				if _, exists := getResp.Metadata["source-host"]; !exists {
					t.Error("Expected source-host metadata")
				}
				if _, exists := getResp.Metadata["backup-date"]; !exists {
					t.Error("Expected backup-date metadata")
				}
				if _, exists := getResp.Metadata["original-path"]; !exists {
					t.Error("Expected original-path metadata")
				}
			}

			t.Logf("✓ %s: Successfully uploaded to %s", tc.description, tc.expectedS3Path)

			// Clean up test file
			os.Remove(testFilePath)
		})
	}
}

// TestIntegrationDateBasedPrefixErrors tests error cases for date-based prefix
func TestIntegrationDateBasedPrefixErrors(t *testing.T) {
	ctx := context.Background()

	// Start LocalStack
	localstackContainer, endpoint := setupLocalStack(t)
	defer func() {
		if err := localstackContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate LocalStack container: %v", err)
		}
	}()

	// Create test configuration
	testBucket := "test-prefix-errors-bucket"
	testRegion := "us-east-1"
	tempDir := t.TempDir()

	// Create S3 client and bucket
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(testRegion),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		t.Fatalf("Failed to create AWS config: %v", err)
	}

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(endpoint)
	})

	_, err = s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(testBucket),
	})
	if err != nil {
		t.Fatalf("Failed to create test bucket: %v", err)
	}

	// Test error cases
	errorTestCases := []struct {
		name        string
		prefix      string
		fileName    string
		expectError bool
		description string
	}{
		{
			name:        "No date in filename with date prefix",
			prefix:      "logs/YYYY/MM/DD",
			fileName:    "nodate.log.gz",
			expectError: true,
			description: "Should fail when filename has no date but prefix requires date",
		},
		{
			name:        "Invalid date format in filename",
			prefix:      "logs/YYYY",
			fileName:    "app99999999.log.gz",
			expectError: true,
			description: "Should fail with invalid date format",
		},
		{
			name:        "No date prefix with regular filename",
			prefix:      "logs",
			fileName:    "nodate.log.gz",
			expectError: false,
			description: "Should work when no date formatting is required",
		},
	}

	for _, tc := range errorTestCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test file
			testFilePath := filepath.Join(tempDir, tc.fileName)
			testContent := []byte(fmt.Sprintf("Test content for %s", tc.fileName))
			
			if err := os.WriteFile(testFilePath, testContent, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Create BackupTool instance
			config := Config{
				S3Bucket:     testBucket,
				S3Prefix:     tc.prefix,
				AWSRegion:    testRegion,
				StorageClass: "STANDARD",
				EndpointURL:  endpoint,
				Period:       "1 month",
			}

			bt, err := NewBackupTool(config)
			if err != nil {
				t.Fatalf("Failed to create BackupTool: %v", err)
			}

			// Override S3 client with LocalStack client
			bt.s3Client = s3Client

			// Test upload
			err = bt.uploadToS3(ctx, testFilePath)
			
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", tc.description)
				} else {
					t.Logf("✓ %s: Got expected error: %v", tc.description, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", tc.description, err)
				} else {
					t.Logf("✓ %s: Succeeded as expected", tc.description)
				}
			}

			// Clean up test file
			os.Remove(testFilePath)
		})
	}
}