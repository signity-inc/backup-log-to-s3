package main

// Export private functions for testing
var (
	FindTargetFiles = (*BackupTool).findTargetFiles
	UploadToS3      = (*BackupTool).uploadToS3
	DeleteLocalFile = (*BackupTool).deleteLocalFile
	ProcessFiles    = (*BackupTool).processFiles
)