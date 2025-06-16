package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	DefaultLockFile     = "/var/run/backup-log-to-s3.lock"
	DefaultStorageClass = "STANDARD_IA"
	
	// ANSI color codes
	ColorRed    = "\033[31m"
	ColorReset  = "\033[0m"
)

// Version is set by ldflags during build
var Version = "dev"

// Config holds the configuration for the backup tool
type Config struct {
	S3Bucket         string
	S3Prefix         string
	AWSRegion        string
	OutputFile       string
	LockFile         string
	StorageClass     string
	Period           string
	DryRun           bool
	Verbose          bool
	DeleteAfterUpload bool
	Help             bool
	Version          bool
	// AWS CLI compatible options
	Profile          string
	EndpointURL      string
	NoVerifySSL      bool
	CABundle         string
	CLIReadTimeout   int
	CLIConnectTimeout int
}

// Stats holds the statistics for the backup operation
type Stats struct {
	TotalFiles int
	Uploaded   int
	Deleted    int
	Errors     int
	Skipped    int
}

// BackupTool represents the main backup tool
type BackupTool struct {
	config      Config
	s3Client    *s3.Client
	logger      *log.Logger
	stats       Stats
	lockFile    *os.File
	cutoffTime  time.Time
}

// NewBackupTool creates a new backup tool instance
func NewBackupTool(config Config) (*BackupTool, error) {
	// Setup logger
	var logWriter io.Writer
	if config.OutputFile != "" {
		logFile, err := os.OpenFile(config.OutputFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		if config.Verbose {
			logWriter = io.MultiWriter(os.Stdout, logFile)
		} else {
			logWriter = logFile
		}
	} else {
		logWriter = os.Stdout
	}

	logger := log.New(logWriter, "", log.LstdFlags)

	// Calculate cutoff time based on period
	cutoffTime, err := calculateCutoffTime(config.Period)
	if err != nil {
		return nil, fmt.Errorf("invalid period '%s': %w", config.Period, err)
	}

	return &BackupTool{
		config:     config,
		logger:     logger,
		cutoffTime: cutoffTime,
	}, nil
}

// parsePeriod parses a period string like "1 day", "7 days", "1 month" etc.
func parsePeriod(period string) (time.Duration, error) {
	parts := strings.Fields(strings.TrimSpace(period))
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid period format. Expected format: '1 day', '7 days', '1 month', etc.\n\nExamples:\n  \"1 day\"     - Files older than 1 day\n  \"7 days\"    - Files older than 7 days\n  \"1 month\"   - Files older than 1 month\n  \"2 months\"  - Files older than 2 months\n  \"1 year\"    - Files older than 1 year")
	}
	
	value := parts[0]
	unit := strings.ToLower(parts[1])
	
	// Parse the numeric value
	var intVal int
	if _, err := fmt.Sscanf(value, "%d", &intVal); err != nil {
		return 0, fmt.Errorf("invalid numeric value: %s\n\nExamples:\n  \"1 day\"     - Files older than 1 day\n  \"7 days\"    - Files older than 7 days\n  \"1 month\"   - Files older than 1 month\n  \"2 months\"  - Files older than 2 months\n  \"1 year\"    - Files older than 1 year", value)
	}
	
	// Convert based on unit
	switch unit {
	case "day", "days":
		return time.Duration(intVal) * 24 * time.Hour, nil
	case "month", "months":
		return time.Duration(intVal) * 24 * 30 * time.Hour, nil // Approximate 30 days
	case "year", "years":
		return time.Duration(intVal) * 24 * 365 * time.Hour, nil // Approximate 365 days
	default:
		return 0, fmt.Errorf("unsupported time unit: %s. Supported units: day/days, month/months, year/years\n\nExamples:\n  \"1 day\"     - Files older than 1 day\n  \"7 days\"    - Files older than 7 days\n  \"1 month\"   - Files older than 1 month\n  \"2 months\"  - Files older than 2 months\n  \"1 year\"    - Files older than 1 year", unit)
	}
}

// calculateCutoffTime calculates the cutoff time based on the period
// Returns the start of the day for the cutoff date for more intuitive comparison
func calculateCutoffTime(period string) (time.Time, error) {
	duration, err := parsePeriod(period)
	if err != nil {
		return time.Time{}, err
	}
	
	now := time.Now()
	cutoff := now.Add(-duration)
	
	// Normalize to start of day (00:00:00) for date-based comparison
	// This makes "1 day" include all files from the previous calendar day
	cutoffDate := time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, cutoff.Location())
	
	// Add one day to make the comparison more intuitive
	// "1 day" should target files older than today (i.e., yesterday and before)
	cutoffDate = cutoffDate.AddDate(0, 0, 1)
	return cutoffDate, nil
}

// initAWS initializes the AWS S3 client
func (bt *BackupTool) initAWS(ctx context.Context) error {
	var loadOptions []func(*config.LoadOptions) error
	
	// Add region if specified
	if bt.config.AWSRegion != "" {
		loadOptions = append(loadOptions, config.WithRegion(bt.config.AWSRegion))
	}
	
	// Add profile if specified
	if bt.config.Profile != "" {
		loadOptions = append(loadOptions, config.WithSharedConfigProfile(bt.config.Profile))
	}
	
	// Check if region will be available
	if bt.config.AWSRegion == "" && os.Getenv("AWS_DEFAULT_REGION") == "" && os.Getenv("AWS_REGION") == "" {
		// Also check ~/.aws/config default region
		homeDir, _ := os.UserHomeDir()
		configFile := filepath.Join(homeDir, ".aws", "config")
		hasConfigRegion := false
		
		if data, err := os.ReadFile(configFile); err == nil {
			// Simple check for region in default profile
			if strings.Contains(string(data), "region") {
				hasConfigRegion = true
			}
		}
		
		if !hasConfigRegion {
			return fmt.Errorf("AWS region is not set. Please specify -region flag or set AWS_DEFAULT_REGION environment variable")
		}
	}
	
	// Load config
	cfg, err := config.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}
	
	// Create custom HTTP client if needed
	if bt.config.NoVerifySSL || bt.config.CABundle != "" || bt.config.CLIReadTimeout > 0 || bt.config.CLIConnectTimeout > 0 {
		httpClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: bt.config.NoVerifySSL,
				},
			},
		}
		
		// Set timeouts if specified
		if bt.config.CLIReadTimeout > 0 {
			httpClient.Timeout = time.Duration(bt.config.CLIReadTimeout) * time.Second
		}
		
		// Configure custom HTTP client
		cfg.HTTPClient = httpClient
	}
	
	// Create S3 client options
	s3Options := []func(*s3.Options){}
	
	// Add endpoint URL if specified
	if bt.config.EndpointURL != "" {
		s3Options = append(s3Options, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(bt.config.EndpointURL)
			// Use path-style addressing for LocalStack compatibility
			o.UsePathStyle = true
		})
	}

	bt.s3Client = s3.NewFromConfig(cfg, s3Options...)

	// Test S3 access
	_, err = bt.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bt.config.S3Bucket),
	})
	if err != nil {
		return fmt.Errorf("cannot access S3 bucket %s: %w", bt.config.S3Bucket, err)
	}

	bt.logger.Printf("AWS S3 client initialized successfully")
	return nil
}

// acquireLock acquires a lock file to prevent concurrent execution
func (bt *BackupTool) acquireLock() error {
	if bt.config.LockFile == "" {
		return nil
	}

	lockFile, err := os.OpenFile(bt.config.LockFile, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("another instance is already running (lock file exists: %s)", bt.config.LockFile)
		}
		return fmt.Errorf("failed to create lock file: %w", err)
	}

	// Write PID to lock file
	_, err = lockFile.WriteString(fmt.Sprintf("%d\n", os.Getpid()))
	if err != nil {
		lockFile.Close()
		os.Remove(bt.config.LockFile)
		return fmt.Errorf("failed to write to lock file: %w", err)
	}

	bt.lockFile = lockFile
	return nil
}

// releaseLock releases the lock file
func (bt *BackupTool) releaseLock() {
	if bt.lockFile != nil {
		bt.lockFile.Close()
		os.Remove(bt.config.LockFile)
	}
}

// extractDateFromFilename extracts date from filename and returns time.Time
func extractDateFromFilename(filename string) (time.Time, error) {
	// Pattern 1: YYYYMMDD format (e.g., app20241215.log.gz)
	re1 := regexp.MustCompile(`(\d{8})`)
	if matches := re1.FindStringSubmatch(filename); len(matches) > 1 {
		dateStr := matches[1]
		if len(dateStr) == 8 {
			t, err := time.Parse("20060102", dateStr)
			if err != nil {
				return time.Time{}, fmt.Errorf("failed to parse date %s: %w", dateStr, err)
			}
			return t, nil
		}
	}

	// Pattern 2: YYYY-MM-DD format (e.g., 2024-12-15.gz)
	re2 := regexp.MustCompile(`(\d{4}-\d{2}-\d{2})`)
	if matches := re2.FindStringSubmatch(filename); len(matches) > 1 {
		dateStr := matches[1]
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse date %s: %w", dateStr, err)
		}
		return t, nil
	}

	// Pattern 3: YYYY/MM/DD format (e.g., 2024/12/15.gz)
	re3 := regexp.MustCompile(`(\d{4}/\d{2}/\d{2})`)
	if matches := re3.FindStringSubmatch(filename); len(matches) > 1 {
		dateStr := matches[1]
		t, err := time.Parse("2006/01/02", dateStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse date %s: %w", dateStr, err)
		}
		return t, nil
	}

	// Pattern 4: YYYY_MM_DD format (e.g., log_2024_12_15.gz)
	re4 := regexp.MustCompile(`(\d{4}_\d{2}_\d{2})`)
	if matches := re4.FindStringSubmatch(filename); len(matches) > 1 {
		dateStr := matches[1]
		// Replace underscores with hyphens for parsing
		normalizedDate := strings.ReplaceAll(dateStr, "_", "-")
		t, err := time.Parse("2006-01-02", normalizedDate)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse date %s: %w", dateStr, err)
		}
		return t, nil
	}

	return time.Time{}, fmt.Errorf("no date pattern found in filename: %s", filename)
}

// processPrefixWithDate processes a prefix that may contain date format tokens
// Returns the processed prefix with date values substituted
func processPrefixWithDate(prefix string, date time.Time) string {
	// Replace date format tokens with actual date values
	result := prefix
	result = strings.ReplaceAll(result, "YYYY", date.Format("2006"))
	result = strings.ReplaceAll(result, "MM", date.Format("01"))
	result = strings.ReplaceAll(result, "DD", date.Format("02"))
	
	return result
}

// convertGlobPattern converts user glob pattern to actual pattern
func convertGlobPattern(pattern string) string {
	// Replace YYYY/MM/DD with wildcard first (longest pattern first)
	pattern = strings.ReplaceAll(pattern, "YYYY/MM/DD", "*")
	// Replace YYYY-MM-DD with wildcard
	pattern = strings.ReplaceAll(pattern, "YYYY-MM-DD", "*")
	// Replace YYYY_MM_DD with wildcard
	pattern = strings.ReplaceAll(pattern, "YYYY_MM_DD", "*")
	// Replace YYYYMMDD with wildcard
	pattern = strings.ReplaceAll(pattern, "YYYYMMDD", "*")
	return pattern
}

// findTargetFiles finds files matching the glob pattern and cutoff time
func (bt *BackupTool) findTargetFiles(globPattern string) ([]string, error) {
	bt.logger.Printf("Searching for files matching pattern: %s", globPattern)
	bt.logger.Printf("Cutoff time: %s", bt.cutoffTime.Format("2006-01-02 15:04:05"))

	// Convert glob pattern
	searchPattern := convertGlobPattern(globPattern)
	bt.logger.Printf("Converted search pattern: %s", searchPattern)

	// Find files using glob
	matches, err := filepath.Glob(searchPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob pattern %s: %w", searchPattern, err)
	}

	var targetFiles []string
	for _, file := range matches {
		// Check if file exists and is regular file
		if info, err := os.Stat(file); err != nil || info.IsDir() {
			continue
		}

		filename := filepath.Base(file)
		fileDate, err := extractDateFromFilename(filename)
		if err != nil {
			bt.logger.Printf("Could not extract date from filename: %s (%v)", file, err)
			bt.stats.Skipped++
			continue
		}

		// Normalize file date to start of day for fair comparison
		fileDateNormalized := time.Date(fileDate.Year(), fileDate.Month(), fileDate.Day(), 0, 0, 0, 0, fileDate.Location())
		
		// Files older than cutoff date should be included
		if fileDateNormalized.Before(bt.cutoffTime) {
			targetFiles = append(targetFiles, file)
			bt.logger.Printf("Target file found: %s (date: %s)", file, fileDate.Format("2006-01-02"))
		} else {
			bt.logger.Printf("File skipped (too recent): %s (date: %s)", file, fileDate.Format("2006-01-02"))
			bt.stats.Skipped++
		}
	}

	bt.stats.TotalFiles = len(targetFiles)
	return targetFiles, nil
}

// printError prints an error message in red color
func printError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, ColorRed+"Error: "+format+ColorReset+"\n", args...)
}

// uploadToS3 uploads a file to S3
func (bt *BackupTool) uploadToS3(ctx context.Context, filePath string) error {
	filename := filepath.Base(filePath)
	
	// Generate S3 key with optional date-based directory structure in prefix
	var s3Key string
	if strings.Contains(bt.config.S3Prefix, "YYYY") || strings.Contains(bt.config.S3Prefix, "MM") || strings.Contains(bt.config.S3Prefix, "DD") {
		// Extract date from filename
		fileDate, err := extractDateFromFilename(filename)
		if err != nil {
			return fmt.Errorf("failed to extract date from filename %s for date-based prefix: %w", filename, err)
		}
		
		// Process prefix with date substitution
		processedPrefix := processPrefixWithDate(bt.config.S3Prefix, fileDate)
		s3Key = fmt.Sprintf("%s/%s", processedPrefix, filename)
	} else {
		s3Key = fmt.Sprintf("%s/%s", bt.config.S3Prefix, filename)
	}

	bt.logger.Printf("Uploading: %s -> s3://%s/%s", filePath, bt.config.S3Bucket, s3Key)

	if bt.config.DryRun {
		bt.logger.Printf("DRY RUN: Would upload %s to s3://%s/%s", filePath, bt.config.S3Bucket, s3Key)
		return nil
	}

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	// Get hostname
	hostname, _ := os.Hostname()

	// Upload to S3
	_, err = bt.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(bt.config.S3Bucket),
		Key:          aws.String(s3Key),
		Body:         file,
		StorageClass: types.StorageClass(bt.config.StorageClass),
		Metadata: map[string]string{
			"source-host":   hostname,
			"backup-date":   time.Now().UTC().Format(time.RFC3339),
			"original-path": filePath,
		},
	})

	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	bt.logger.Printf("Upload successful: s3://%s/%s", bt.config.S3Bucket, s3Key)
	return nil
}

// deleteLocalFile deletes the local file
func (bt *BackupTool) deleteLocalFile(filePath string) error {
	if bt.config.DryRun {
		bt.logger.Printf("DRY RUN: Would delete %s", filePath)
		return nil
	}

	err := os.Remove(filePath)
	if err != nil {
		return fmt.Errorf("failed to delete local file %s: %w", filePath, err)
	}

	bt.logger.Printf("Local file deleted: %s", filePath)
	return nil
}

// processFiles processes the found files
func (bt *BackupTool) processFiles(ctx context.Context, files []string) error {
	for _, file := range files {
		// Double-check file still exists
		if _, err := os.Stat(file); err != nil {
			bt.logger.Printf("File not found (may have been processed): %s", file)
			bt.stats.Skipped++
			continue
		}

		// Upload to S3
		if err := bt.uploadToS3(ctx, file); err != nil {
			bt.logger.Printf("Upload failed: %s (%v)", file, err)
			bt.stats.Errors++
			continue
		}
		bt.stats.Uploaded++

		// Delete local file only if delete option is enabled
		if bt.config.DeleteAfterUpload {
			if err := bt.deleteLocalFile(file); err != nil {
				bt.logger.Printf("Delete failed: %s (%v)", file, err)
				bt.stats.Errors++
				continue
			}
			bt.stats.Deleted++
		}
	}

	return nil
}

// logSummary logs the summary statistics
func (bt *BackupTool) logSummary(globPattern string) {
	bt.logger.Printf("=== Backup Summary ===")
	bt.logger.Printf("Glob pattern: %s", globPattern)
	bt.logger.Printf("Cutoff time: %s", bt.cutoffTime.Format("2006-01-02 15:04:05"))
	bt.logger.Printf("Total files: %d", bt.stats.TotalFiles)
	bt.logger.Printf("Uploaded: %d", bt.stats.Uploaded)
	bt.logger.Printf("Deleted: %d", bt.stats.Deleted)
	bt.logger.Printf("Skipped: %d", bt.stats.Skipped)
	bt.logger.Printf("Errors: %d", bt.stats.Errors)
}

// Run executes the backup process
func (bt *BackupTool) Run(ctx context.Context, globPattern string) error {
	bt.logger.Printf("=== Log backup process started ===")
	bt.logger.Printf("Glob pattern: %s", globPattern)

	// Validate glob pattern
	if !strings.Contains(globPattern, "YYYYMMDD") &&
		!strings.Contains(globPattern, "YYYY-MM-DD") &&
		!strings.Contains(globPattern, "YYYY/MM/DD") &&
		!strings.Contains(globPattern, "YYYY_MM_DD") {
		return fmt.Errorf("invalid glob pattern. Must contain 'YYYYMMDD', 'YYYY-MM-DD', 'YYYY/MM/DD', or 'YYYY_MM_DD'\n\nExamples:\n  *YYYYMMDD.log.gz           - Matches app20241215.log.gz\n  YYYY-MM-DD.gz              - Matches 2024-12-15.gz\n  YYYY/MM/DD.gz              - Matches 2024/12/15.gz\n  YYYY_MM_DD.gz              - Matches 2024_12_15.gz\n  /var/log/app*YYYYMMDD.gz   - Matches /var/log/app20241215.gz\n  nginx-YYYY-MM-DD.log.gz    - Matches nginx-2024-12-15.log.gz\n  access_YYYY/MM/DD.log.gz   - Matches access_2024/12/15.log.gz")
	}

	// Acquire lock
	if err := bt.acquireLock(); err != nil {
		return err
	}
	defer bt.releaseLock()

	// Initialize AWS
	if err := bt.initAWS(ctx); err != nil {
		return err
	}

	// Find target files
	files, err := bt.findTargetFiles(globPattern)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		bt.logger.Printf("No files found for pattern '%s' before %s", globPattern, bt.cutoffTime.Format("2006-01-02"))
		bt.logger.Printf("=== Log backup process completed ===")
		return nil
	}

	bt.logger.Printf("Found %d files to backup", len(files))

	// Process files
	if err := bt.processFiles(ctx, files); err != nil {
		return err
	}

	// Log summary
	bt.logSummary(globPattern)

	if bt.stats.Errors > 0 {
		bt.logger.Printf("Backup completed with %d errors", bt.stats.Errors)
		return fmt.Errorf("backup completed with %d errors", bt.stats.Errors)
	}

	bt.logger.Printf("=== Log backup process completed successfully ===")
	return nil
}

func parseFlags() (Config, string, string, error) {
	var config Config
	var period string
	var globPattern string

	flag.StringVar(&config.S3Bucket, "bucket", "", "S3 bucket name (required)")
	flag.StringVar(&config.S3Prefix, "prefix", "", "S3 prefix (supports date format like logs/YYYY/MM/DD) (required)")
	flag.StringVar(&config.AWSRegion, "region", "", "AWS region (uses AWS_DEFAULT_REGION if not specified)")
	flag.StringVar(&config.OutputFile, "output", "", "Output log file path (outputs to stdout if not specified)")
	flag.StringVar(&config.LockFile, "lock", DefaultLockFile, "Lock file path")
	flag.StringVar(&config.StorageClass, "storage-class", DefaultStorageClass, "S3 storage class")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Dry run mode")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose logging")
	flag.BoolVar(&config.DeleteAfterUpload, "delete", false, "Delete local files after successful upload")
	flag.BoolVar(&config.Help, "help", false, "Show help")
	flag.BoolVar(&config.Version, "version", false, "Show version")
	
	// AWS CLI compatible options
	flag.StringVar(&config.Profile, "profile", "", "Use a specific profile from your credential file")
	flag.StringVar(&config.EndpointURL, "endpoint-url", "", "Override command's default URL with the given URL")
	flag.BoolVar(&config.NoVerifySSL, "no-verify-ssl", false, "By default, the AWS CLI uses SSL when communicating with AWS services")
	flag.StringVar(&config.CABundle, "ca-bundle", "", "The CA certificate bundle to use when verifying SSL certificates")
	flag.IntVar(&config.CLIReadTimeout, "cli-read-timeout", 0, "The maximum socket read time in seconds (0 means no timeout)")
	flag.IntVar(&config.CLIConnectTimeout, "cli-connect-timeout", 0, "The maximum socket connect time in seconds (0 means no timeout)")

	flag.Parse()

	if config.Help {
		return config, "", "", nil
	}

	if config.Version {
		fmt.Printf("backup-log-to-s3 version %s\n", Version)
		return config, "", "", nil
	}

	// Collect all validation errors
	var errors []string
	
	// Get period and glob pattern from command line args
	args := flag.Args()
	if len(args) != 2 {
		errors = append(errors, "Both period and glob pattern are required")
	} else {
		period = args[0]
		globPattern = args[1]
		// Set period in config
		config.Period = period
	}

	// Validate required fields
	if config.S3Bucket == "" {
		errors = append(errors, "S3 bucket name is required (use -bucket flag)")
	}
	if config.S3Prefix == "" {
		errors = append(errors, "S3 prefix is required (use -prefix flag)")
	}
	
	// If there are validation errors, display them all
	if len(errors) > 0 {
		for _, err := range errors {
			printError(err)
		}
		fmt.Fprintf(os.Stderr, "\nUsage: %s [OPTIONS] <period> <glob_pattern>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Use -help for more information.\n")
		return config, "", "", fmt.Errorf("missing required arguments")
	}
	// OutputFile is now optional - will use stdout if not specified
	
	// Check region - if not specified, AWS SDK will use AWS_DEFAULT_REGION
	// We'll validate this later in initAWS

	return config, period, globPattern, nil
}

func showUsage() {
	fmt.Printf(`Usage: %s [OPTIONS] <period> <glob_pattern>

Log backup tool that uploads files matching the glob pattern to S3.
Only files older than the specified period are processed.
By default, local files are kept after upload. Use -delete to remove them.

ARGUMENTS:
  period          Time period (e.g., "1 day", "7 days", "1 month", "1 year")
  glob_pattern    File pattern with YYYYMMDD, YYYY-MM-DD, YYYY/MM/DD, or YYYY_MM_DD date format

PERIOD EXAMPLES:
  "1 day"         - Files older than 1 day
  "7 days"        - Files older than 7 days  
  "1 month"       - Files older than 1 month
  "2 months"      - Files older than 2 months
  "1 year"        - Files older than 1 year

PATTERN EXAMPLES:
  "*YYYYMMDD.log.gz"           - Matches app20241215.log.gz
  "YYYY-MM-DD.gz"              - Matches 2024-12-15.gz
  "YYYY/MM/DD.gz"              - Matches 2024/12/15.gz
  "YYYY_MM_DD.gz"              - Matches 2024_12_15.gz
  "/var/log/app*YYYYMMDD.gz"   - Matches /var/log/app20241215.gz
  "nginx-YYYY-MM-DD.log.gz"    - Matches nginx-2024-12-15.log.gz
  "access_YYYY/MM/DD.log.gz"   - Matches access_2024/12/15.log.gz
  "system_YYYY_MM_DD.log.gz"   - Matches system_2024_12_15.log.gz

OPTIONS:
  -bucket string
        S3 bucket name (required)
  -prefix string
        S3 prefix (supports date format: YYYY, MM, DD tokens) (required)
        Examples: "logs", "logs/YYYY", "logs/YYYY/MM", "logs/YYYY/MM/DD"
  -region string
        AWS region (uses AWS_DEFAULT_REGION if not specified)
  -output string
        Output log file path (outputs to stdout if not specified)
  -lock string
        Lock file path (default "%s")
  -storage-class string
        S3 storage class (default "%s")
  -dry-run
        Dry run mode (default false)
  -verbose
        Verbose logging (default false)
  -delete
        Delete local files after successful upload (default false)
  -help
        Show this help
  -version
        Show version

AWS CLI COMPATIBLE OPTIONS:
  -profile string
        Use a specific profile from your credential file
  -endpoint-url string
        Override command's default URL with the given URL
  -no-verify-ssl
        By default, the AWS CLI uses SSL when communicating with AWS services
  -ca-bundle string
        The CA certificate bundle to use when verifying SSL certificates
  -cli-read-timeout int
        The maximum socket read time in seconds (0 means no timeout)
  -cli-connect-timeout int
        The maximum socket connect time in seconds (0 means no timeout)

EXAMPLES:
  %s -bucket my-logs -prefix logs "1 day" "*YYYYMMDD.log.gz"
  %s -bucket my-logs -prefix logs "7 days" "YYYY-MM-DD.gz"
  %s -bucket my-logs -prefix logs "1 month" "YYYY/MM/DD.gz"
  %s -bucket my-logs -prefix logs -dry-run "7 days" "/var/log/app*YYYYMMDD.gz"
  
PREFIX WITH DATE FORMAT EXAMPLES:
  %s -bucket my-logs -prefix "logs" "1 month" "*YYYYMMDD.log.gz"
    # Saves to: my-logs/logs/filename.log.gz (no date substitution)
  %s -bucket my-logs -prefix "logs/YYYY" "1 month" "*YYYYMMDD.log.gz"
    # Saves to: my-logs/logs/2024/filename.log.gz (year from filename date)
  %s -bucket my-logs -prefix "logs/YYYY/MM" "1 month" "*YYYYMMDD.log.gz"
    # Saves to: my-logs/logs/2024/12/filename.log.gz (year/month from filename date)
  %s -bucket my-logs -prefix "logs/YYYY/MM/DD" "1 month" "*YYYYMMDD.log.gz"
    # Saves to: my-logs/logs/2024/12/15/filename.log.gz (full date from filename)

DATE TOKEN BEHAVIOR:
  - YYYY: Replaced with 4-digit year from filename date (e.g., 2024)
  - MM:   Replaced with 2-digit month from filename date (e.g., 12)
  - DD:   Replaced with 2-digit day from filename date (e.g., 15)
  - Date is extracted from filename patterns: YYYYMMDD, YYYY-MM-DD, YYYY/MM/DD, YYYY_MM_DD
  - If filename contains no date pattern, upload will fail with error

ENVIRONMENT VARIABLES:
  AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_DEFAULT_REGION
  See AWS documentation for authentication options.

`, os.Args[0], DefaultLockFile, DefaultStorageClass, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

func main() {
	config, _, globPattern, err := parseFlags()
	if err != nil {
		// Error message is already printed by parseFlags() in red
		// Only show usage for missing arguments, not for other errors
		if strings.Contains(err.Error(), "missing required") {
			// Brief usage already shown in parseFlags
		} else {
			printError("%v", err)
			fmt.Fprintf(os.Stderr, "\n")
			showUsage()
		}
		os.Exit(1)
	}

	if config.Help {
		showUsage()
		os.Exit(0)
	}

	if config.Version {
		os.Exit(0)
	}

	backupTool, err := NewBackupTool(config)
	if err != nil {
		printError("%v", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := backupTool.Run(ctx, globPattern); err != nil {
		printError("%v", err)
		os.Exit(1)
	}
}
