package storage

import (
	"context"
	"io"

	"github.com/pingcap/metering_sdk/storage/provider"
)

// ObjectStorageProvider defines the object storage provider interface
type ObjectStorageProvider interface {
	// Upload uploads data to specified path
	Upload(ctx context.Context, path string, data io.Reader) error
	// Download downloads data from specified path
	Download(ctx context.Context, path string) (io.ReadCloser, error)
	// Delete deletes data at specified path
	Delete(ctx context.Context, path string) error
	// Exists checks if data exists at specified path
	Exists(ctx context.Context, path string) (bool, error)
	// List lists all objects under specified prefix
	List(ctx context.Context, prefix string) ([]string, error)
}

// Re-export types from provider package for external use
type (
	ProviderType   = provider.ProviderType
	ProviderConfig = provider.ProviderConfig
	AWSConfig      = provider.AWSConfig
	GCSConfig      = provider.GCSConfig
	AzureConfig    = provider.AzureConfig
	OSSConfig      = provider.OSSConfig
	COSConfig      = provider.COSConfig
	LocalFSConfig  = provider.LocalFSConfig
)

// Re-export constants
const (
	ProviderTypeS3      = provider.ProviderTypeS3
	ProviderTypeGCS     = provider.ProviderTypeGCS
	ProviderTypeAzure   = provider.ProviderTypeAzure
	ProviderTypeOSS     = provider.ProviderTypeOSS
	ProviderTypeCOS     = provider.ProviderTypeCOS
	ProviderTypeLocalFS = provider.ProviderTypeLocalFS
)
