package storage_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pingcap/metering_sdk/common"
	"github.com/pingcap/metering_sdk/config"
	metareader "github.com/pingcap/metering_sdk/reader/meta"
	meteringreader "github.com/pingcap/metering_sdk/reader/metering"
	"github.com/pingcap/metering_sdk/storage"
	metawriter "github.com/pingcap/metering_sdk/writer/meta"
	meteringwriter "github.com/pingcap/metering_sdk/writer/metering"
	"github.com/stretchr/testify/assert"
)

func TestLocalFSProvider(t *testing.T) {
	// Create temporary directory
	tempDir := filepath.Join(os.TempDir(), "tidb-metering-test", "localfs")
	defer os.RemoveAll(tempDir)

	// Create configuration
	providerConfig := &storage.ProviderConfig{
		Type:   storage.ProviderTypeLocalFS,
		Prefix: "test-prefix",
		LocalFS: &storage.LocalFSConfig{
			BasePath:   tempDir,
			CreateDirs: true,
		},
	}

	// Create storage provider
	provider, err := storage.NewObjectStorageProvider(providerConfig)
	assert.NoError(t, err, "Failed to create LocalFS provider")

	// Test file upload
	testContent := []byte("test content for local filesystem")
	err = provider.Upload(context.Background(), "test/file.txt", bytes.NewReader(testContent))
	assert.NoError(t, err, "Failed to upload file")

	// Verify file exists (considering prefix)
	expectedPath := filepath.Join(tempDir, "test-prefix", "test", "file.txt")
	_, err = os.Stat(expectedPath)
	assert.False(t, os.IsNotExist(err), "File was not created at expected path: %s", expectedPath)

	// Verify file content
	content, err := os.ReadFile(expectedPath)
	assert.NoError(t, err, "Failed to read file")
	assert.Equal(t, testContent, content, "File content mismatch")

	t.Logf("LocalFS provider test passed. File created at: %s", expectedPath)
}

func TestS3ProviderConfiguration(t *testing.T) {
	// Test S3 configuration creation (without actually connecting to S3)
	providerConfig := &storage.ProviderConfig{
		Type:   storage.ProviderTypeS3,
		Bucket: "test-bucket",
		Region: "us-west-2",
	}

	// Not calling NewObjectStorageProvider here because we don't have valid AWS credentials
	// Just verify configuration structure is correct
	assert.Equal(t, storage.ProviderTypeS3, providerConfig.Type, "S3 provider type mismatch")
	assert.Equal(t, "test-bucket", providerConfig.Bucket, "S3 bucket mismatch")
	assert.Equal(t, "us-west-2", providerConfig.Region, "S3 region mismatch")

	t.Logf("S3 provider configuration test passed")
}

func TestCOSProviderFactory(t *testing.T) {
	providerConfig := &storage.ProviderConfig{
		Type:   storage.ProviderTypeCOS,
		Bucket: "metering-123456",
		Region: "ap-beijing",
		COS: &storage.COSConfig{
			AccessKey:       "sid",
			SecretAccessKey: "skey",
		},
	}

	provider, err := storage.NewObjectStorageProvider(providerConfig)
	assert.NoError(t, err, "Failed to create COS provider")
	assert.NotNil(t, provider, "COS provider should not be nil")
}

func TestMeteringWriterWithLocalFS(t *testing.T) {
	// Create temporary directory
	tempDir := filepath.Join(os.TempDir(), "tidb-metering-test", "writer")
	defer os.RemoveAll(tempDir)

	// Create configuration
	cfg := config.DefaultConfig()
	providerConfig := &storage.ProviderConfig{
		Type: storage.ProviderTypeLocalFS,
		LocalFS: &storage.LocalFSConfig{
			BasePath:   tempDir,
			CreateDirs: true,
		},
	}

	// Create storage provider
	provider, err := storage.NewObjectStorageProvider(providerConfig)
	assert.NoError(t, err, "Failed to create LocalFS provider")

	// Create writer
	meteringWriter := meteringwriter.NewMeteringWriterWithSharedPool(provider, cfg, "test-shared-pool-1")
	defer meteringWriter.Close()

	// Create test data
	now := time.Now()
	testData := &common.MeteringData{
		Timestamp: now.Unix() / 60 * 60, // Ensure minute-level timestamp
		Category:  "test",
		SelfID:    "testcomponent",
		Data: []map[string]interface{}{
			{
				"logical_cluster": "lc-test",
				"test_metric":     &common.MeteringValue{Value: 123, Unit: "count"},
			},
		},
	}

	// Write data
	err = meteringWriter.Write(context.Background(), testData)
	assert.NoError(t, err, "Failed to write metering data")

	// Verify file is created
	expectedPath := filepath.Join(tempDir, "metering", "ru",
		fmt.Sprintf("%d", testData.Timestamp), "test", "test-shared-pool-1",
		fmt.Sprintf("%s-0.json.gz", testData.SelfID))

	_, err = os.Stat(expectedPath)
	assert.False(t, os.IsNotExist(err), "Metering data file was not created at expected path: %s", expectedPath)

	t.Logf("Metering writer test passed. File created at: %s", expectedPath)
}

func TestMetaWriterWithLocalFS(t *testing.T) {
	// Create temporary directory
	tempDir := filepath.Join(os.TempDir(), "tidb-metering-test", "meta")
	defer os.RemoveAll(tempDir)

	// Create configuration
	cfg := config.DefaultConfig()
	providerConfig := &storage.ProviderConfig{
		Type: storage.ProviderTypeLocalFS,
		LocalFS: &storage.LocalFSConfig{
			BasePath:   tempDir,
			CreateDirs: true,
		},
	}

	// Create storage provider
	provider, err := storage.NewObjectStorageProvider(providerConfig)
	assert.NoError(t, err, "Failed to create LocalFS provider")

	// Create writer
	metaWriter := metawriter.NewMetaWriter(provider, cfg)
	defer metaWriter.Close()

	// Create test data
	now := time.Now()
	testData := &common.MetaData{
		ClusterID: "test-cluster",
		Type:      common.MetaTypeLogic,
		ModifyTS:  now.Unix(),
		Metadata: map[string]interface{}{
			"test_field": "test_value",
		},
	}

	// Write data
	err = metaWriter.Write(context.Background(), testData)
	assert.NoError(t, err, "Failed to write meta data")

	// Verify file is created
	expectedPath := filepath.Join(tempDir, "metering", "meta", string(testData.Type), testData.ClusterID,
		fmt.Sprintf("%d.json.gz", testData.ModifyTS))

	_, err = os.Stat(expectedPath)
	assert.False(t, os.IsNotExist(err), "Meta data file was not created at expected path: %s", expectedPath)

	t.Logf("Meta writer test passed. File created at: %s", expectedPath)
}

func TestProviderSwitching(t *testing.T) {
	// Create two different local storage configurations
	tempDir1 := filepath.Join(os.TempDir(), "tidb-metering-test", "switch1")
	tempDir2 := filepath.Join(os.TempDir(), "tidb-metering-test", "switch2")
	defer os.RemoveAll(tempDir1)
	defer os.RemoveAll(tempDir2)

	configs := []*storage.ProviderConfig{
		{
			Type: storage.ProviderTypeLocalFS,
			LocalFS: &storage.LocalFSConfig{
				BasePath:   tempDir1,
				CreateDirs: true,
			},
		},
		{
			Type: storage.ProviderTypeLocalFS,
			LocalFS: &storage.LocalFSConfig{
				BasePath:   tempDir2,
				CreateDirs: true,
			},
		},
	}

	// Test each configuration
	for i, providerConfig := range configs {
		provider, err := storage.NewObjectStorageProvider(providerConfig)
		assert.NoError(t, err, "Failed to create provider %d", i+1)

		// Test file upload
		testContent := []byte(fmt.Sprintf("test content %d", i+1))
		err = provider.Upload(context.Background(), "test.txt", bytes.NewReader(testContent))
		assert.NoError(t, err, "Failed to upload with provider %d", i+1)

		// Verify file exists
		expectedPath := filepath.Join(providerConfig.LocalFS.BasePath, "test.txt")
		_, err = os.Stat(expectedPath)
		assert.False(t, os.IsNotExist(err), "File was not created by provider %d at: %s", i+1, expectedPath)

		t.Logf("Provider %d test passed. File created at: %s", i+1, expectedPath)
	}
}

func TestProviderPrefix(t *testing.T) {
	// Create temporary directory
	tempDir := filepath.Join(os.TempDir(), "tidb-metering-test", "prefix")
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name         string
		prefix       string
		uploadPath   string
		expectedPath string
		description  string
	}{
		{
			name:         "no prefix",
			prefix:       "",
			uploadPath:   "test/file.txt",
			expectedPath: "test/file.txt",
			description:  "when prefix is empty, file should be stored directly at specified path",
		},
		{
			name:         "simple prefix",
			prefix:       "data",
			uploadPath:   "test/file.txt",
			expectedPath: "data/test/file.txt",
			description:  "prefix should be added before the path",
		},
		{
			name:         "prefix with slash",
			prefix:       "data/backup/",
			uploadPath:   "test/file.txt",
			expectedPath: "data/backup/test/file.txt",
			description:  "trailing slash in prefix should be handled correctly",
		},
		{
			name:         "multi-level prefix",
			prefix:       "env/staging/backup",
			uploadPath:   "metering/ru/123456/compute/cluster-001.json.gz",
			expectedPath: "env/staging/backup/metering/ru/123456/compute/cluster-001.json.gz",
			description:  "multi-level prefix should be combined correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create independent subdirectory for each test
			testDir := filepath.Join(tempDir, tt.name)
			defer os.RemoveAll(testDir)

			// Create configuration
			providerConfig := &storage.ProviderConfig{
				Type:   storage.ProviderTypeLocalFS,
				Prefix: tt.prefix,
				LocalFS: &storage.LocalFSConfig{
					BasePath:   testDir,
					CreateDirs: true,
				},
			}

			// Create storage provider
			provider, err := storage.NewObjectStorageProvider(providerConfig)
			assert.NoError(t, err, "Failed to create LocalFS provider")

			// Test file upload
			testContent := []byte("test content for prefix testing")
			err = provider.Upload(context.Background(), tt.uploadPath, bytes.NewReader(testContent))
			assert.NoError(t, err, "Failed to upload file")

			// Verify file is created at expected path
			expectedFullPath := filepath.Join(testDir, tt.expectedPath)
			_, err = os.Stat(expectedFullPath)
			assert.False(t, os.IsNotExist(err), "File was not created at expected path: %s", expectedFullPath)

			// Verify file content
			content, err := os.ReadFile(expectedFullPath)
			assert.NoError(t, err, "Failed to read file")
			assert.Equal(t, string(testContent), string(content), "File content mismatch")

			// Test Exists method
			exists, err := provider.Exists(context.Background(), tt.uploadPath)
			assert.NoError(t, err, "Failed to check file existence")
			assert.True(t, exists, "File should exist but doesn't")

			// Test Download method
			readCloser, err := provider.Download(context.Background(), tt.uploadPath)
			assert.NoError(t, err, "Failed to download file")
			defer readCloser.Close()

			// Test Delete method
			err = provider.Delete(context.Background(), tt.uploadPath)
			assert.NoError(t, err, "Failed to delete file")

			// Verify file has been deleted
			_, err = os.Stat(expectedFullPath)
			assert.True(t, os.IsNotExist(err), "File should be deleted but still exists")

			t.Logf("✓ %s: %s", tt.description, expectedFullPath)
		})
	}
}

func TestS3ProviderPrefix(t *testing.T) {
	// Test S3 provider prefix configuration (configuration validation only, no actual connection)
	tests := []struct {
		name   string
		prefix string
		bucket string
		region string
	}{
		{
			name:   "S3 no prefix",
			prefix: "",
			bucket: "test-bucket",
			region: "us-west-2",
		},
		{
			name:   "S3 with prefix",
			prefix: "production/metering",
			bucket: "my-data-bucket",
			region: "us-east-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerConfig := &storage.ProviderConfig{
				Type:   storage.ProviderTypeS3,
				Prefix: tt.prefix,
				Bucket: tt.bucket,
				Region: tt.region,
			}

			// Verify configuration structure is correct
			assert.Equal(t, storage.ProviderTypeS3, providerConfig.Type, "Provider type mismatch")
			assert.Equal(t, tt.prefix, providerConfig.Prefix, "Prefix mismatch")
			assert.Equal(t, tt.bucket, providerConfig.Bucket, "Bucket mismatch")

			t.Logf("✓ S3 configuration validated: prefix='%s', bucket='%s', region='%s'",
				tt.prefix, tt.bucket, tt.region)
		})
	}
}

func TestMeteringReaderWithLocalFS(t *testing.T) {
	// Create temporary directory
	tempDir := filepath.Join(os.TempDir(), "tidb-metering-test", "reader")
	defer os.RemoveAll(tempDir)

	// Create configuration
	cfg := config.DefaultConfig()
	providerConfig := &storage.ProviderConfig{
		Type: storage.ProviderTypeLocalFS,
		LocalFS: &storage.LocalFSConfig{
			BasePath:   tempDir,
			CreateDirs: true,
		},
	}

	// Create storage provider
	provider, err := storage.NewObjectStorageProvider(providerConfig)
	assert.NoError(t, err, "Failed to create LocalFS provider")

	// First write some test data using the writer
	meteringWriter := meteringwriter.NewMeteringWriterWithSharedPool(provider, cfg, "test-shared-pool-2")
	defer meteringWriter.Close()

	// Create test data
	testTimestamp := int64(1640995200) // 2022-01-01 00:00:00 UTC
	testData := &common.MeteringData{
		Timestamp: testTimestamp,
		Category:  "compute",
		SelfID:    "cluster001",
		Data: []map[string]interface{}{
			{
				"logical_cluster": "lc-test-1",
				"cpu_usage":       &common.MeteringValue{Value: 80, Unit: "percentage"},
				"memory_usage":    &common.MeteringValue{Value: 2048, Unit: "MB"},
			},
			{
				"logical_cluster": "lc-test-2",
				"cpu_usage":       &common.MeteringValue{Value: 60, Unit: "percentage"},
				"memory_usage":    &common.MeteringValue{Value: 1024, Unit: "MB"},
			},
		},
	}

	// Write test data
	err = meteringWriter.Write(context.Background(), testData)
	assert.NoError(t, err, "Failed to write test data")

	// Create another test data with different category
	testData2 := &common.MeteringData{
		Timestamp: testTimestamp,
		Category:  "storage",
		SelfID:    "storage001",
		Data: []map[string]interface{}{
			{
				"logical_cluster": "lc-test-1",
				"disk_usage":      &common.MeteringValue{Value: 500, Unit: "GB"},
			},
		},
	}

	// Write second test data
	err = meteringWriter.Write(context.Background(), testData2)
	assert.NoError(t, err, "Failed to write second test data")

	// Now test the reader
	meteringReader := meteringreader.NewMeteringReader(provider, cfg)

	// Test ListFilesByTimestamp
	timestampFiles, err := meteringReader.ListFilesByTimestamp(context.Background(), testTimestamp)
	assert.NoError(t, err, "Failed to list files by timestamp")
	assert.NotNil(t, timestampFiles, "TimestampFiles should not be nil")
	assert.Equal(t, testTimestamp, timestampFiles.Timestamp, "Timestamp mismatch")
	assert.Contains(t, timestampFiles.Files, "compute", "Compute category should exist")
	assert.Contains(t, timestampFiles.Files, "storage", "Storage category should exist")
	assert.Greater(t, len(timestampFiles.Files["compute"]), 0, "Compute category should have files")
	assert.Greater(t, len(timestampFiles.Files["storage"]), 0, "Storage category should have files")

	// Test GetCategories
	categories, err := meteringReader.GetCategories(context.Background(), testTimestamp)
	assert.NoError(t, err, "Failed to get categories")
	assert.Contains(t, categories, "compute", "Categories should contain compute")
	assert.Contains(t, categories, "storage", "Categories should contain storage")

	// Test GetFilesByCategory
	computeFiles, err := meteringReader.GetFilesByCategory(context.Background(), testTimestamp, "compute")
	assert.NoError(t, err, "Failed to get compute files")
	assert.Greater(t, len(computeFiles), 0, "Should have compute files")

	storageFiles, err := meteringReader.GetFilesByCategory(context.Background(), testTimestamp, "storage")
	assert.NoError(t, err, "Failed to get storage files")
	assert.Greater(t, len(storageFiles), 0, "Should have storage files")

	// Test GetFileInfo
	for _, filePath := range computeFiles {
		fileInfo, err := meteringReader.GetFileInfo(filePath)
		assert.NoError(t, err, "Failed to get file info for %s", filePath)
		assert.Equal(t, testTimestamp, fileInfo.Timestamp, "File timestamp mismatch")
		assert.Equal(t, "compute", fileInfo.Category, "File category mismatch")
		assert.Equal(t, "cluster001", fileInfo.SelfID, "File self_id mismatch")
		assert.GreaterOrEqual(t, fileInfo.Part, 0, "File part should be non-negative")
	}

	// Test ReadFile
	if len(computeFiles) > 0 {
		firstComputeFile := computeFiles[0]
		readData, err := meteringReader.ReadFile(context.Background(), firstComputeFile)
		assert.NoError(t, err, "Failed to read file %s", firstComputeFile)
		assert.NotNil(t, readData, "Read data should not be nil")
		assert.Equal(t, testTimestamp, readData.Timestamp, "Read data timestamp mismatch")
		assert.Equal(t, "compute", readData.Category, "Read data category mismatch")
		assert.Equal(t, "cluster001", readData.SelfID, "Read data self_id mismatch")
		assert.Equal(t, len(testData.Data), len(readData.Data), "Data entries count mismatch")

		// Verify data content
		for i, dataEntry := range readData.Data {
			originalEntry := testData.Data[i]
			assert.Equal(t, originalEntry["logical_cluster"], dataEntry["logical_cluster"], "Logical cluster mismatch")

			// Check metering values
			if originalCPU, ok := originalEntry["cpu_usage"].(*common.MeteringValue); ok {
				if readCPU, ok := dataEntry["cpu_usage"].(map[string]interface{}); ok {
					// JSON unmarshaling converts numbers to float64, so we need to compare accordingly
					expectedValue := float64(originalCPU.Value)
					assert.Equal(t, expectedValue, readCPU["value"], "CPU value mismatch")
					assert.Equal(t, originalCPU.Unit, readCPU["unit"], "CPU unit mismatch")
				} else {
					t.Errorf("CPU usage data format unexpected: %+v", dataEntry["cpu_usage"])
				}
			}
		}
	}

	// Test Read (interface method)
	if len(storageFiles) > 0 {
		firstStorageFile := storageFiles[0]
		readInterface, err := meteringReader.Read(context.Background(), firstStorageFile)
		assert.NoError(t, err, "Failed to read file via interface %s", firstStorageFile)

		readData, ok := readInterface.(*common.MeteringData)
		assert.True(t, ok, "Read interface should return *common.MeteringData")
		assert.Equal(t, testTimestamp, readData.Timestamp, "Interface read data timestamp mismatch")
		assert.Equal(t, "storage", readData.Category, "Interface read data category mismatch")
	}

	// Test error cases
	// Test reading non-existent file
	_, err = meteringReader.ReadFile(context.Background(), "metering/ru/999999/nonexistent/test-0.json.gz")
	assert.Error(t, err, "Should error when reading non-existent file")

	// Test listing files for non-existent timestamp
	emptyFiles, err := meteringReader.ListFilesByTimestamp(context.Background(), 999999)
	assert.NoError(t, err, "Should not error when listing non-existent timestamp")
	assert.Equal(t, int64(999999), emptyFiles.Timestamp, "Empty timestamp should match")
	assert.Equal(t, 0, len(emptyFiles.Files), "Should have no files for non-existent timestamp")

	// Test GetFileInfo with invalid path
	_, err = meteringReader.GetFileInfo("invalid/path")
	assert.Error(t, err, "Should error for invalid file path")

	t.Logf("Metering reader test passed. Files found: compute=%d, storage=%d",
		len(computeFiles), len(storageFiles))
}

func TestMeteringReadWriteRoundtripWithLocalFS(t *testing.T) {
	// Create temporary directory
	tempDir := filepath.Join(os.TempDir(), "tidb-metering-test", "roundtrip")
	defer os.RemoveAll(tempDir)

	// Create configuration
	cfg := config.DefaultConfig()
	providerConfig := &storage.ProviderConfig{
		Type: storage.ProviderTypeLocalFS,
		LocalFS: &storage.LocalFSConfig{
			BasePath:   tempDir,
			CreateDirs: true,
		},
	}

	// Create storage provider
	provider, err := storage.NewObjectStorageProvider(providerConfig)
	assert.NoError(t, err, "Failed to create LocalFS provider")

	// Create writer and reader
	meteringWriter := meteringwriter.NewMeteringWriterWithSharedPool(provider, cfg, "test-shared-pool")
	defer meteringWriter.Close()
	meteringReader := meteringreader.NewMeteringReader(provider, cfg)

	// Test multiple timestamps and categories
	testCases := []struct {
		timestamp int64
		category  string
		selfID    string
		data      []map[string]interface{}
	}{
		{
			timestamp: 1640995200, // 2022-01-01 00:00:00
			category:  "compute",
			selfID:    "tidb001",
			data: []map[string]interface{}{
				{
					"logical_cluster": "production",
					"cpu_cores":       &common.MeteringValue{Value: 16, Unit: "cores"},
					"memory_gb":       &common.MeteringValue{Value: 64, Unit: "GB"},
				},
			},
		},
		{
			timestamp: 1640995200,
			category:  "storage",
			selfID:    "tikv001",
			data: []map[string]interface{}{
				{
					"logical_cluster": "production",
					"disk_size":       &common.MeteringValue{Value: 1000, Unit: "GB"},
					"iops":            &common.MeteringValue{Value: 5000, Unit: "ops/s"},
				},
			},
		},
		{
			timestamp: 1640995260, // 1 minute later
			category:  "compute",
			selfID:    "tidb001",
			data: []map[string]interface{}{
				{
					"logical_cluster": "production",
					"cpu_cores":       &common.MeteringValue{Value: 16, Unit: "cores"},
					"memory_gb":       &common.MeteringValue{Value: 64, Unit: "GB"},
				},
				{
					"logical_cluster": "development",
					"cpu_cores":       &common.MeteringValue{Value: 4, Unit: "cores"},
					"memory_gb":       &common.MeteringValue{Value: 16, Unit: "GB"},
				},
			},
		},
	}

	// Write all test data
	writtenFiles := make(map[string]*common.MeteringData)
	for _, tc := range testCases {
		testData := &common.MeteringData{
			Timestamp: tc.timestamp,
			Category:  tc.category,
			SelfID:    tc.selfID,
			Data:      tc.data,
		}

		err = meteringWriter.Write(context.Background(), testData)
		assert.NoError(t, err, "Failed to write test data for %s/%s", tc.category, tc.selfID)

		// Expected file path for verification
		expectedPath := fmt.Sprintf("metering/ru/%d/%s/%s/%s-0.json.gz",
			tc.timestamp, tc.category, "test-shared-pool", tc.selfID)
		writtenFiles[expectedPath] = testData
	}

	// Test reading all written files
	for filePath, originalData := range writtenFiles {
		t.Run(fmt.Sprintf("read_%s", filePath), func(t *testing.T) {
			// Verify file exists
			exists, err := provider.Exists(context.Background(), filePath)
			assert.NoError(t, err, "Failed to check if file exists")
			assert.True(t, exists, "File should exist: %s", filePath)

			// Read via reader
			readData, err := meteringReader.ReadFile(context.Background(), filePath)
			assert.NoError(t, err, "Failed to read file: %s", filePath)

			// Verify metadata
			assert.Equal(t, originalData.Timestamp, readData.Timestamp, "Timestamp mismatch")
			assert.Equal(t, originalData.Category, readData.Category, "Category mismatch")
			assert.Equal(t, originalData.SelfID, readData.SelfID, "SelfID mismatch")
			assert.Equal(t, len(originalData.Data), len(readData.Data), "Data entries count mismatch")

			// Verify data content
			for i, originalEntry := range originalData.Data {
				readEntry := readData.Data[i]
				assert.Equal(t, originalEntry["logical_cluster"], readEntry["logical_cluster"],
					"Logical cluster mismatch in entry %d", i)

				// Check all metering values
				for key, originalValue := range originalEntry {
					if key == "logical_cluster" {
						continue // Already checked
					}

					if originalMV, ok := originalValue.(*common.MeteringValue); ok {
						readValue, ok := readEntry[key].(map[string]interface{})
						assert.True(t, ok, "Expected map[string]interface{} for key %s", key)
						// JSON unmarshaling converts numbers to float64, so we need to compare accordingly
						expectedValue := float64(originalMV.Value)
						assert.Equal(t, expectedValue, readValue["value"], "Value mismatch for %s", key)
						assert.Equal(t, originalMV.Unit, readValue["unit"], "Unit mismatch for %s", key)
					}
				}
			}
		})
	}

	// Test listing files by timestamp
	uniqueTimestamps := make(map[int64]bool)
	for _, tc := range testCases {
		uniqueTimestamps[tc.timestamp] = true
	}

	for timestamp := range uniqueTimestamps {
		t.Run(fmt.Sprintf("list_timestamp_%d", timestamp), func(t *testing.T) {
			timestampFiles, err := meteringReader.ListFilesByTimestamp(context.Background(), timestamp)
			assert.NoError(t, err, "Failed to list files for timestamp %d", timestamp)
			assert.Equal(t, timestamp, timestampFiles.Timestamp, "Timestamp mismatch")

			// Count expected files for this timestamp
			expectedCategories := make(map[string]int)
			for _, tc := range testCases {
				if tc.timestamp == timestamp {
					expectedCategories[tc.category]++
				}
			}

			// Verify categories exist
			for expectedCategory := range expectedCategories {
				assert.Contains(t, timestampFiles.Files, expectedCategory,
					"Category %s should exist for timestamp %d", expectedCategory, timestamp)
				assert.Greater(t, len(timestampFiles.Files[expectedCategory]), 0,
					"Category %s should have files for timestamp %d", expectedCategory, timestamp)
			}
		})
	}

	t.Logf("Metering read-write roundtrip test completed successfully")
}

func TestMetaReaderWithLocalFS(t *testing.T) {
	// Create temporary directory
	tempDir := filepath.Join(os.TempDir(), "tidb-metering-test", "meta-reader")
	defer os.RemoveAll(tempDir)

	// Create configuration
	cfg := config.DefaultConfig()
	providerConfig := &storage.ProviderConfig{
		Type: storage.ProviderTypeLocalFS,
		LocalFS: &storage.LocalFSConfig{
			BasePath:   tempDir,
			CreateDirs: true,
		},
	}

	// Create storage provider
	provider, err := storage.NewObjectStorageProvider(providerConfig)
	assert.NoError(t, err, "Failed to create LocalFS provider")

	// First write some test data using the writer
	metaWriter := metawriter.NewMetaWriter(provider, cfg)
	defer metaWriter.Close()

	// Create test metadata
	testClusterID := "test-cluster-001"
	testTimestamp := time.Now().Unix()
	testMetadata := &common.MetaData{
		ClusterID: testClusterID,
		Type:      common.MetaTypeLogic,
		ModifyTS:  testTimestamp,
		Metadata: map[string]interface{}{
			"version":      "v6.5.0",
			"node_count":   5,
			"region":       "us-west-2",
			"environment":  "production",
			"capabilities": []string{"ACID", "TiFlash", "TiCDC"},
		},
	}

	// Write test metadata
	err = metaWriter.Write(context.Background(), testMetadata)
	assert.NoError(t, err, "Failed to write test metadata")

	// Write another metadata with different timestamp
	testMetadata2 := &common.MetaData{
		ClusterID: testClusterID,
		Type:      common.MetaTypeSharedpool,
		ModifyTS:  testTimestamp + 60, // 1 minute later
		Metadata: map[string]interface{}{
			"version":      "v6.5.1",
			"node_count":   6,
			"region":       "us-west-2",
			"environment":  "production",
			"capabilities": []string{"ACID", "TiFlash", "TiCDC", "TiDB Lightning"},
		},
	}

	err = metaWriter.Write(context.Background(), testMetadata2)
	assert.NoError(t, err, "Failed to write second test metadata")

	// Now test the reader
	metaReader, err := metareader.NewMetaReader(provider, cfg, nil)
	assert.NoError(t, err, "Failed to create meta reader")

	// Test Read - read metadata by cluster ID and timestamp
	firstMeta, err := metaReader.ReadByType(context.Background(), testClusterID, common.MetaTypeLogic, testTimestamp)
	assert.NoError(t, err, "Failed to read metadata by timestamp")
	assert.NotNil(t, firstMeta, "First metadata should not be nil")
	assert.Equal(t, testClusterID, firstMeta.ClusterID, "Cluster ID mismatch")
	assert.Equal(t, testTimestamp, firstMeta.ModifyTS, "Timestamp should match")
	assert.Equal(t, "v6.5.0", firstMeta.Metadata["version"], "Version should be the first one")
	assert.Equal(t, float64(5), firstMeta.Metadata["node_count"], "Node count should be the first one")

	// Test Read with second timestamp
	secondMeta, err := metaReader.ReadByType(context.Background(), testClusterID, common.MetaTypeSharedpool, testTimestamp+60)
	assert.NoError(t, err, "Failed to read second metadata")
	assert.NotNil(t, secondMeta, "Second metadata should not be nil")
	assert.Equal(t, testClusterID, secondMeta.ClusterID, "Cluster ID mismatch")
	assert.Equal(t, testTimestamp+60, secondMeta.ModifyTS, "Timestamp should match")
	assert.Equal(t, "v6.5.1", secondMeta.Metadata["version"], "Version should be the second one")
	assert.Equal(t, float64(6), secondMeta.Metadata["node_count"], "Node count should be the second one")

	// Test ReadFile (interface method)
	expectedPath := fmt.Sprintf("metering/meta/%s/%s/%d.json.gz", string(common.MetaTypeLogic), testClusterID, testTimestamp)
	readInterface, err := metaReader.ReadFile(context.Background(), expectedPath)
	assert.NoError(t, err, "Failed to read via ReadFile")
	readMeta, ok := readInterface.(*common.MetaData)
	assert.True(t, ok, "ReadFile should return *common.MetaData")
	assert.Equal(t, testClusterID, readMeta.ClusterID, "ReadFile cluster ID mismatch")
	assert.Equal(t, testTimestamp, readMeta.ModifyTS, "ReadFile timestamp mismatch")

	// Test error cases
	// Test reading non-existent cluster/timestamp combination
	_, err = metaReader.Read(context.Background(), "non-existent-cluster", testTimestamp)
	assert.Error(t, err, "Should error when reading non-existent cluster")

	// Test reading non-existent timestamp for existing cluster
	_, err = metaReader.Read(context.Background(), testClusterID, 999999)
	assert.Error(t, err, "Should error when reading non-existent timestamp")

	// Test ReadFile with non-existent path
	_, err = metaReader.ReadFile(context.Background(), "metering/meta/non-existent/999999.json.gz")
	assert.Error(t, err, "Should error when reading non-existent file")

	t.Logf("Meta reader test passed. Cluster: %s, Timestamps: [%d, %d]",
		testClusterID, testTimestamp, testTimestamp+60)
}
