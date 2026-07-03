package storage

import (
	"fmt"

	"github.com/pingcap/metering_sdk/storage/provider"
)

// NewObjectStorageProvider creates object storage provider based on configuration
func NewObjectStorageProvider(config *ProviderConfig) (ObjectStorageProvider, error) {
	// Directly use provider.ProviderConfig since they are now the same type
	switch config.Type {
	case provider.ProviderTypeS3:
		return provider.NewS3Provider(config)
	case provider.ProviderTypeOSS:
		return provider.NewOSSProvider(config)
	case provider.ProviderTypeCOS:
		return provider.NewCOSProvider(config)
	case provider.ProviderTypeLocalFS:
		return provider.NewLocalFSProvider(config)
	case provider.ProviderTypeGCS:
		return nil, fmt.Errorf("provider GCS  not implemented yet")
	case provider.ProviderTypeAzure:
		return provider.NewAzureProvider(config)
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", config.Type)
	}
}
