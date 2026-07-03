package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/pingcap/metering_sdk/storage"
	"go.uber.org/zap"
)

// Config contains SDK common configuration
type Config struct {
	// Logger log instance, if nil will use default nop logger
	Logger *zap.Logger
	// Debug whether to enable debug mode
	Debug bool
	// OverwriteExisting whether to overwrite existing files, default false
	// When false, returns error if file already exists
	// When true, directly overwrites existing file
	OverwriteExisting bool
	// PageSizeBytes page size in bytes, when serialized data exceeds this size, pagination is performed
	// Default 0 means no pagination. Recommended value like 50MB = 50 * 1024 * 1024
	PageSizeBytes int64
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		Logger: zap.NewNop(), // default use nop logger
		Debug:  false,
	}
}

// NewDebugConfig returns configuration with debug mode enabled
func NewDebugConfig() *Config {
	debugLogger, err := zap.NewDevelopment()
	if err != nil {
		// If creation fails, use nop logger
		debugLogger = zap.NewNop()
	}

	return &Config{
		Logger: debugLogger,
		Debug:  true,
	}
}

// WithLogger sets custom logger
func (c *Config) WithLogger(logger *zap.Logger) *Config {
	c.Logger = logger
	return c
}

// WithProductionLogger sets production environment logger
func (c *Config) WithProductionLogger() *Config {
	logger, err := zap.NewProduction()
	if err != nil {
		// If creation fails, use nop logger
		c.Logger = zap.NewNop()
	} else {
		c.Logger = logger
	}
	return c
}

// WithDevelopmentLogger set debug logger
func (c *Config) WithDevelopmentLogger() *Config {
	devLogger, err := zap.NewDevelopment()
	if err != nil {
		return c
	}
	c.Logger = devLogger
	c.Debug = true
	return c
}

// WithDebug sets debug mode
func (c *Config) WithDebug(debug bool) *Config {
	c.Debug = debug

	// If debug mode is enabled and current logger is nop logger, create a debug level logger
	if debug && (c.Logger == nil || c.Logger == zap.NewNop()) {
		debugLogger, err := zap.NewDevelopment()
		if err != nil {
			// If creation fails, keep the original logger
			return c
		}
		c.Logger = debugLogger
	} else if !debug && c.Logger != nil {
		// If debug mode is disabled, we can choose to reset to nop logger (but keep user explicitly set logger)
		// Here we don't do automatic reset, let user control manually
	}

	return c
}

// GetLogger gets logger instance
func (c *Config) GetLogger() *zap.Logger {
	if c.Logger == nil {
		return zap.NewNop()
	}
	return c.Logger
}

// WithOverwriteExisting sets whether to overwrite existing files
func (c *Config) WithOverwriteExisting(overwrite bool) *Config {
	c.OverwriteExisting = overwrite
	return c
}

// WithPageSize sets page size (bytes)
func (c *Config) WithPageSize(sizeBytes int64) *Config {
	c.PageSizeBytes = sizeBytes
	return c
}

// WithPageSizeMB sets page size (MB)
func (c *Config) WithPageSizeMB(sizeMB int64) *Config {
	c.PageSizeBytes = sizeMB * 1024 * 1024
	return c
}

// MeteringAWSConfig AWS S3 specific configuration for high-level config
type MeteringAWSConfig struct {
	AssumeRoleARN    string `yaml:"assume-role-arn,omitempty" toml:"assume-role-arn,omitempty" json:"assume-role-arn,omitempty" reloadable:"false"`
	S3ForcePathStyle bool   `yaml:"s3-force-path-style,omitempty" toml:"s3-force-path-style,omitempty" json:"s3-force-path-style,omitempty" reloadable:"false"`
	AccessKey        string `yaml:"access-key,omitempty" toml:"access-key,omitempty" json:"access-key,omitempty" reloadable:"false"`
	SecretAccessKey  string `yaml:"secret-access-key,omitempty" toml:"secret-access-key,omitempty" json:"secret-access-key,omitempty" reloadable:"false"`
	SessionToken     string `yaml:"session-token,omitempty" toml:"session-token,omitempty" json:"session-token,omitempty" reloadable:"false"`
}

// MeteringOSSConfig Alibaba Cloud OSS specific configuration for high-level config
type MeteringOSSConfig struct {
	AssumeRoleARN   string `yaml:"assume-role-arn,omitempty" toml:"assume-role-arn,omitempty" json:"assume-role-arn,omitempty" reloadable:"false"`
	AccessKey       string `yaml:"access-key,omitempty" toml:"access-key,omitempty" json:"access-key,omitempty" reloadable:"false"`
	SecretAccessKey string `yaml:"secret-access-key,omitempty" toml:"secret-access-key,omitempty" json:"secret-access-key,omitempty" reloadable:"false"`
	SessionToken    string `yaml:"session-token,omitempty" toml:"session-token,omitempty" json:"session-token,omitempty" reloadable:"false"`
}

// MeteringCOSConfig TencentCloud COS specific configuration for high-level config
type MeteringCOSConfig struct {
	AssumeRoleARN   string `yaml:"assume-role-arn,omitempty" toml:"assume-role-arn,omitempty" json:"assume-role-arn,omitempty" reloadable:"false"`
	AccessKey       string `yaml:"access-key,omitempty" toml:"access-key,omitempty" json:"access-key,omitempty" reloadable:"false"`
	SecretAccessKey string `yaml:"secret-access-key,omitempty" toml:"secret-access-key,omitempty" json:"secret-access-key,omitempty" reloadable:"false"`
	SessionToken    string `yaml:"session-token,omitempty" toml:"session-token,omitempty" json:"session-token,omitempty" reloadable:"false"`
}

// MeteringAzureConfig Azure Blob Storage specific configuration for high-level config
type MeteringAzureConfig struct {
	AccountName string `yaml:"account-name,omitempty" toml:"account-name,omitempty" json:"account-name,omitempty" reloadable:"false"`
	AccountKey  string `yaml:"account-key,omitempty" toml:"account-key,omitempty" json:"account-key,omitempty" reloadable:"false"`
	SASToken    string `yaml:"sas-token,omitempty" toml:"sas-token,omitempty" json:"sas-token,omitempty" reloadable:"false"`
}

// MeteringLocalFSConfig local filesystem specific configuration for high-level config
type MeteringLocalFSConfig struct {
	BasePath    string `yaml:"base-path,omitempty" toml:"base-path,omitempty" json:"base-path,omitempty" reloadable:"false"`
	CreateDirs  bool   `yaml:"create-dirs,omitempty" toml:"create-dirs,omitempty" json:"create-dirs,omitempty" reloadable:"false"`
	Permissions string `yaml:"permissions,omitempty" toml:"permissions,omitempty" json:"permissions,omitempty" reloadable:"false"`
}

// MeteringConfig represents a high-level configuration for metering SDK
// It combines storage provider configuration with business-specific settings
type MeteringConfig struct {
	// Storage provider type: s3, oss, localfs, etc.
	Type storage.ProviderType `yaml:"type,omitempty" toml:"type,omitempty" json:"type,omitempty" reloadable:"false"`
	// Storage region
	Region string `yaml:"region,omitempty" toml:"region,omitempty" json:"region,omitempty" reloadable:"false"`
	// Storage bucket/container name
	Bucket string `yaml:"bucket,omitempty" toml:"bucket,omitempty" json:"bucket,omitempty" reloadable:"false"`
	// Path prefix for all stored files
	Prefix string `yaml:"prefix,omitempty" toml:"prefix,omitempty" json:"prefix,omitempty" reloadable:"false"`
	// Custom endpoint for S3-compatible services
	Endpoint string `yaml:"endpoint,omitempty" toml:"endpoint,omitempty" json:"endpoint,omitempty" reloadable:"false"`

	// Cloud-specific configurations
	AWS     *MeteringAWSConfig     `yaml:"aws,omitempty" toml:"aws,omitempty" json:"aws,omitempty" reloadable:"false"`
	OSS     *MeteringOSSConfig     `yaml:"oss,omitempty" toml:"oss,omitempty" json:"oss,omitempty" reloadable:"false"`
	COS     *MeteringCOSConfig     `yaml:"cos,omitempty" toml:"cos,omitempty" json:"cos,omitempty" reloadable:"false"`
	Azure   *MeteringAzureConfig   `yaml:"azure,omitempty" toml:"azure,omitempty" json:"azure,omitempty" reloadable:"false"`
	LocalFS *MeteringLocalFSConfig `yaml:"localfs,omitempty" toml:"localfs,omitempty" json:"localfs,omitempty" reloadable:"false"`

	// Business-specific configurations
	// Shared pool cluster ID for sharedpool type metadata
	SharedPoolID string `yaml:"shared-pool-id,omitempty" toml:"shared-pool-id,omitempty" json:"shared-pool-id,omitempty" reloadable:"false"`
}

// ToProviderConfig converts MeteringConfig to storage.ProviderConfig
func (mc *MeteringConfig) ToProviderConfig() *storage.ProviderConfig {
	config := &storage.ProviderConfig{
		Type:     mc.Type,
		Region:   mc.Region,
		Bucket:   mc.Bucket,
		Prefix:   mc.Prefix,
		Endpoint: mc.Endpoint,
	}

	switch mc.Type {
	case storage.ProviderTypeS3:
		if mc.AWS != nil {
			config.AWS = &storage.AWSConfig{
				AssumeRoleARN:    mc.AWS.AssumeRoleARN,
				S3ForcePathStyle: mc.AWS.S3ForcePathStyle,
				AccessKey:        mc.AWS.AccessKey,
				SecretAccessKey:  mc.AWS.SecretAccessKey,
				SessionToken:     mc.AWS.SessionToken,
			}
		}
	case storage.ProviderTypeOSS:
		if mc.OSS != nil {
			config.OSS = &storage.OSSConfig{
				AssumeRoleARN:   mc.OSS.AssumeRoleARN,
				AccessKey:       mc.OSS.AccessKey,
				SecretAccessKey: mc.OSS.SecretAccessKey,
				SessionToken:    mc.OSS.SessionToken,
			}
		}
	case storage.ProviderTypeCOS:
		if mc.COS != nil {
			config.COS = &storage.COSConfig{
				AssumeRoleARN:   mc.COS.AssumeRoleARN,
				AccessKey:       mc.COS.AccessKey,
				SecretAccessKey: mc.COS.SecretAccessKey,
				SessionToken:    mc.COS.SessionToken,
			}
		}
	case storage.ProviderTypeAzure:
		if mc.Azure != nil {
			config.Azure = &storage.AzureConfig{
				AccountName: mc.Azure.AccountName,
				AccountKey:  mc.Azure.AccountKey,
				SASToken:    mc.Azure.SASToken,
			}
		}
	case storage.ProviderTypeLocalFS:
		if mc.LocalFS != nil {
			config.LocalFS = &storage.LocalFSConfig{
				BasePath:    mc.LocalFS.BasePath,
				CreateDirs:  mc.LocalFS.CreateDirs,
				Permissions: mc.LocalFS.Permissions,
			}
		}
	}

	return config
}

// NewMeteringConfig creates a new MeteringConfig with default values
func NewMeteringConfig() *MeteringConfig {
	return &MeteringConfig{}
}

// WithS3 configures for AWS S3 storage
func (mc *MeteringConfig) WithS3(region, bucket string) *MeteringConfig {
	mc.Type = storage.ProviderTypeS3
	mc.Region = region
	mc.Bucket = bucket
	return mc
}

// WithS3AssumeRole configures for AWS S3 storage with assume role
func (mc *MeteringConfig) WithS3AssumeRole(region, bucket, roleARN string) *MeteringConfig {
	mc.Type = storage.ProviderTypeS3
	mc.Region = region
	mc.Bucket = bucket
	if mc.AWS == nil {
		mc.AWS = &MeteringAWSConfig{}
	}
	mc.AWS.AssumeRoleARN = roleARN
	return mc
}

// WithOSS configures for Alibaba Cloud OSS storage
func (mc *MeteringConfig) WithOSS(region, bucket string) *MeteringConfig {
	mc.Type = storage.ProviderTypeOSS
	mc.Region = region
	mc.Bucket = bucket
	return mc
}

// WithOSSAssumeRole configures for Alibaba Cloud OSS storage with assume role
func (mc *MeteringConfig) WithOSSAssumeRole(region, bucket, roleARN string) *MeteringConfig {
	mc.Type = storage.ProviderTypeOSS
	mc.Region = region
	mc.Bucket = bucket
	if mc.OSS == nil {
		mc.OSS = &MeteringOSSConfig{}
	}
	mc.OSS.AssumeRoleARN = roleARN
	return mc
}

// WithCOS configures for TencentCloud COS storage
func (mc *MeteringConfig) WithCOS(region, bucket string) *MeteringConfig {
	mc.Type = storage.ProviderTypeCOS
	mc.Region = region
	mc.Bucket = bucket
	return mc
}

// WithCOSAssumeRole configures for TencentCloud COS storage with assume role
func (mc *MeteringConfig) WithCOSAssumeRole(region, bucket, roleARN string) *MeteringConfig {
	mc.Type = storage.ProviderTypeCOS
	mc.Region = region
	mc.Bucket = bucket
	if mc.COS == nil {
		mc.COS = &MeteringCOSConfig{}
	}
	mc.COS.AssumeRoleARN = roleARN
	return mc
}

// WithAzure configures for Azure Blob Storage
func (mc *MeteringConfig) WithAzure(accountName, container string) *MeteringConfig {
	mc.Type = storage.ProviderTypeAzure
	mc.Bucket = container
	if mc.Azure == nil {
		mc.Azure = &MeteringAzureConfig{}
	}
	mc.Azure.AccountName = accountName
	return mc
}

// WithAzureAccountKey configures for Azure Blob Storage with account key
func (mc *MeteringConfig) WithAzureAccountKey(accountName, container, accountKey string) *MeteringConfig {
	mc.WithAzure(accountName, container)
	mc.Azure.AccountKey = accountKey
	return mc
}

// WithAzureSASToken configures for Azure Blob Storage with SAS token
func (mc *MeteringConfig) WithAzureSASToken(accountName, container, sasToken string) *MeteringConfig {
	mc.WithAzure(accountName, container)
	mc.Azure.SASToken = sasToken
	return mc
}

// WithLocalFS configures for local filesystem storage
func (mc *MeteringConfig) WithLocalFS(basePath string) *MeteringConfig {
	mc.Type = storage.ProviderTypeLocalFS
	if mc.LocalFS == nil {
		mc.LocalFS = &MeteringLocalFSConfig{}
	}
	mc.LocalFS.BasePath = basePath
	mc.LocalFS.CreateDirs = true
	return mc
}

// WithAWSConfig sets AWS specific configuration
func (mc *MeteringConfig) WithAWSConfig(awsConfig *MeteringAWSConfig) *MeteringConfig {
	mc.AWS = awsConfig
	return mc
}

// WithOSSConfig sets OSS specific configuration
func (mc *MeteringConfig) WithOSSConfig(ossConfig *MeteringOSSConfig) *MeteringConfig {
	mc.OSS = ossConfig
	return mc
}

// WithCOSConfig sets COS specific configuration
func (mc *MeteringConfig) WithCOSConfig(cosConfig *MeteringCOSConfig) *MeteringConfig {
	mc.COS = cosConfig
	return mc
}

// WithAzureConfig sets Azure specific configuration
func (mc *MeteringConfig) WithAzureConfig(azureConfig *MeteringAzureConfig) *MeteringConfig {
	mc.Azure = azureConfig
	return mc
}

// WithLocalFSConfig sets LocalFS specific configuration
func (mc *MeteringConfig) WithLocalFSConfig(localConfig *MeteringLocalFSConfig) *MeteringConfig {
	mc.LocalFS = localConfig
	return mc
}

// WithPrefix sets the path prefix
func (mc *MeteringConfig) WithPrefix(prefix string) *MeteringConfig {
	mc.Prefix = prefix
	return mc
}

// WithEndpoint sets the custom endpoint
func (mc *MeteringConfig) WithEndpoint(endpoint string) *MeteringConfig {
	mc.Endpoint = endpoint
	return mc
}

// WithSharedPoolID sets the shared pool cluster ID
func (mc *MeteringConfig) WithSharedPoolID(poolID string) *MeteringConfig {
	mc.SharedPoolID = poolID
	return mc
}

// GetSharedPoolID gets the shared pool cluster ID
func (mc *MeteringConfig) GetSharedPoolID() string {
	return mc.SharedPoolID
}

// WithAWSRoleARN sets the AWS IAM role ARN for assume role
func (mc *MeteringConfig) WithAWSRoleARN(roleARN string) *MeteringConfig {
	if mc.AWS == nil {
		mc.AWS = &MeteringAWSConfig{}
	}
	mc.AWS.AssumeRoleARN = roleARN
	return mc
}

// WithOSSRoleARN sets the Alibaba Cloud OSS role ARN for assume role
func (mc *MeteringConfig) WithOSSRoleARN(roleARN string) *MeteringConfig {
	if mc.OSS == nil {
		mc.OSS = &MeteringOSSConfig{}
	}
	mc.OSS.AssumeRoleARN = roleARN
	return mc
}

// WithCOSRoleARN sets the TencentCloud COS role ARN for assume role
func (mc *MeteringConfig) WithCOSRoleARN(roleARN string) *MeteringConfig {
	if mc.COS == nil {
		mc.COS = &MeteringCOSConfig{}
	}
	mc.COS.AssumeRoleARN = roleARN
	return mc
}

// NewFromURI creates a new MeteringConfig from a URI string.
// URI format: [scheme]://[bucket]/[prefix]?[parameters]
// Examples:
//   - s3://my-bucket/data?region-id=us-east-1&endpoint=https://s3.example.com
//   - oss://my-bucket/logs?region-id=oss-ap-southeast-1&access-key=AKSKEXAMPLE
//   - cos://my-bucket/logs?region-id=ap-beijing&assume-role-arn=qcs::cam::uin/123:roleName/metering
//   - azure://my-container/prefix?account-name=acct&account-key=key&endpoint=https://acct.blob.core.windows.net
//   - localfs:///data/storage/logs?create-dirs=true&permissions=0755
//
// Supported schemes: s3, oss, cos, azure (alias: azblob), localfs, file
// Common parameters: region-id/region, endpoint, shared-pool-id
// AWS/S3 parameters: access-key, secret-access-key, session-token, assume-role-arn/role-arn, s3-force-path-style/force-path-style
// OSS parameters: access-key, secret-access-key, session-token, assume-role-arn/role-arn
// COS parameters: access-key, secret-access-key, session-token, assume-role-arn/role-arn
// Azure parameters: account-name, account-key, sas-token
// LocalFS parameters: create-dirs, permissions
func NewFromURI(uriStr string) (*MeteringConfig, error) {
	parsedURL, err := url.Parse(uriStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URI: %w", err)
	}

	config := NewMeteringConfig()

	// Parse scheme to determine provider type
	switch strings.ToLower(parsedURL.Scheme) {
	case "s3":
		config.Type = storage.ProviderTypeS3
	case "oss":
		config.Type = storage.ProviderTypeOSS
	case "cos":
		config.Type = storage.ProviderTypeCOS
	case "azure", "azblob":
		config.Type = storage.ProviderTypeAzure
	case "localfs", "file":
		config.Type = storage.ProviderTypeLocalFS
	default:
		return nil, fmt.Errorf("unsupported URI scheme: %s", parsedURL.Scheme)
	}

	// Parse host and path based on provider type
	if config.Type == storage.ProviderTypeLocalFS {
		// For localfs, handle different path formats
		var basePath string
		if parsedURL.Host != "" {
			// For URI like "localfs://host/path", combine host and path
			// Ensure proper path construction without double slashes
			hostPath := "/" + parsedURL.Host
			if parsedURL.Path != "" && parsedURL.Path != "/" {
				// Remove leading slash from path to avoid double slashes
				cleanPath := strings.TrimPrefix(parsedURL.Path, "/")
				basePath = hostPath + "/" + cleanPath
			} else {
				basePath = hostPath
			}
		} else {
			// For URI like "file:///path" or "localfs:///path", use path directly
			basePath = parsedURL.Path
		}
		config.LocalFS = &MeteringLocalFSConfig{
			BasePath:   basePath,
			CreateDirs: true, // default
		}
	} else {
		// For cloud providers, host is bucket name
		if parsedURL.Host != "" {
			config.Bucket = parsedURL.Host
		}

		// Path is the prefix (remove leading slash)
		if parsedURL.Path != "" {
			config.Prefix = strings.TrimPrefix(parsedURL.Path, "/")
		}
	}

	// Parse query parameters
	queryParams := parsedURL.Query()

	// Common parameters
	regionID := queryParams.Get("region-id")
	if regionID == "" {
		regionID = queryParams.Get("region")
	}
	if regionID != "" {
		config.Region = regionID
	}
	if prefix := queryParams.Get("prefix"); prefix != "" {
		config.Prefix = prefix
	}
	if endpoint := queryParams.Get("endpoint"); endpoint != "" {
		config.Endpoint = endpoint
	}
	if sharedPoolID := queryParams.Get("shared-pool-id"); sharedPoolID != "" {
		config.SharedPoolID = sharedPoolID
	}

	// Provider-specific parameters
	switch config.Type {
	case storage.ProviderTypeS3:
		awsConfig := &MeteringAWSConfig{}
		hasAWSConfig := false

		if accessKey := queryParams.Get("access-key"); accessKey != "" {
			awsConfig.AccessKey = accessKey
			hasAWSConfig = true
		}
		if secretKey := queryParams.Get("secret-access-key"); secretKey != "" {
			awsConfig.SecretAccessKey = secretKey
			hasAWSConfig = true
		}
		if sessionToken := queryParams.Get("session-token"); sessionToken != "" {
			awsConfig.SessionToken = sessionToken
			hasAWSConfig = true
		}
		// Support both "assume-role-arn" and "role-arn" parameter names
		roleARN := queryParams.Get("assume-role-arn")
		if roleARN == "" {
			roleARN = queryParams.Get("role-arn")
		}
		if roleARN != "" {
			awsConfig.AssumeRoleARN = roleARN
			hasAWSConfig = true
		}
		// Support both "s3-force-path-style" and "force-path-style" parameter names
		forcePathStyle := queryParams.Get("s3-force-path-style")
		if forcePathStyle == "" {
			forcePathStyle = queryParams.Get("force-path-style")
		}
		if forcePathStyle == "true" {
			awsConfig.S3ForcePathStyle = true
			hasAWSConfig = true
		}

		if hasAWSConfig {
			config.AWS = awsConfig
		}

	case storage.ProviderTypeOSS:
		ossConfig := &MeteringOSSConfig{}
		hasOSSConfig := false

		if accessKey := queryParams.Get("access-key"); accessKey != "" {
			ossConfig.AccessKey = accessKey
			hasOSSConfig = true
		}
		if secretKey := queryParams.Get("secret-access-key"); secretKey != "" {
			ossConfig.SecretAccessKey = secretKey
			hasOSSConfig = true
		}
		if sessionToken := queryParams.Get("session-token"); sessionToken != "" {
			ossConfig.SessionToken = sessionToken
			hasOSSConfig = true
		}
		// Support both "assume-role-arn" and "role-arn" parameter names
		roleARN := queryParams.Get("assume-role-arn")
		if roleARN == "" {
			roleARN = queryParams.Get("role-arn")
		}
		if roleARN != "" {
			ossConfig.AssumeRoleARN = roleARN
			hasOSSConfig = true
		}

		if hasOSSConfig {
			config.OSS = ossConfig
		}

	case storage.ProviderTypeCOS:
		cosConfig := &MeteringCOSConfig{}
		hasCOSConfig := false

		if accessKey := queryParams.Get("access-key"); accessKey != "" {
			cosConfig.AccessKey = accessKey
			hasCOSConfig = true
		}
		if secretKey := queryParams.Get("secret-access-key"); secretKey != "" {
			cosConfig.SecretAccessKey = secretKey
			hasCOSConfig = true
		}
		if sessionToken := queryParams.Get("session-token"); sessionToken != "" {
			cosConfig.SessionToken = sessionToken
			hasCOSConfig = true
		}
		roleARN := queryParams.Get("assume-role-arn")
		if roleARN == "" {
			roleARN = queryParams.Get("role-arn")
		}
		if roleARN != "" {
			cosConfig.AssumeRoleARN = roleARN
			hasCOSConfig = true
		}

		if hasCOSConfig {
			config.COS = cosConfig
		}

	case storage.ProviderTypeAzure:
		azureConfig := &MeteringAzureConfig{}
		hasAzureConfig := false

		if accountName := queryParams.Get("account-name"); accountName != "" {
			azureConfig.AccountName = accountName
			hasAzureConfig = true
		}
		if accountKey := queryParams.Get("account-key"); accountKey != "" {
			azureConfig.AccountKey = accountKey
			hasAzureConfig = true
		}
		if sasToken := queryParams.Get("sas-token"); sasToken != "" {
			azureConfig.SASToken = sasToken
			hasAzureConfig = true
		}

		if hasAzureConfig {
			config.Azure = azureConfig
		}

	case storage.ProviderTypeLocalFS:
		if config.LocalFS == nil {
			config.LocalFS = &MeteringLocalFSConfig{CreateDirs: true}
		}

		if createDirs := queryParams.Get("create-dirs"); createDirs == "false" {
			config.LocalFS.CreateDirs = false
		}
		if permissions := queryParams.Get("permissions"); permissions != "" {
			config.LocalFS.Permissions = permissions
		}
	}

	return config, nil
}

// ToURI converts MeteringConfig to a URI string.
// URI format: [scheme]://[bucket]/[prefix]?[parameters]
// Examples:
//   - s3://my-bucket/data?region-id=us-east-1&endpoint=https://s3.example.com
//   - oss://my-bucket/logs?region-id=oss-ap-southeast-1&access-key=AKSKEXAMPLE
//   - cos://my-bucket/logs?region-id=ap-beijing
//   - azure://my-container/prefix?account-name=acct&account-key=key&endpoint=https://acct.blob.core.windows.net
//   - localfs:///data/storage/logs?create-dirs=true&permissions=0755
func (mc *MeteringConfig) ToURI() string {
	var uri strings.Builder
	var params url.Values = make(url.Values)

	// Determine scheme based on provider type
	switch mc.Type {
	case storage.ProviderTypeS3:
		uri.WriteString("s3://")
	case storage.ProviderTypeOSS:
		uri.WriteString("oss://")
	case storage.ProviderTypeCOS:
		uri.WriteString("cos://")
	case storage.ProviderTypeAzure:
		uri.WriteString("azure://")
	case storage.ProviderTypeLocalFS:
		uri.WriteString("localfs://")
	default:
		return ""
	}

	// Build host and path based on provider type
	if mc.Type == storage.ProviderTypeLocalFS {
		// For localfs, handle the base path properly
		if mc.LocalFS != nil && mc.LocalFS.BasePath != "" {
			// Remove any leading slash to avoid double slashes with scheme
			basePath := strings.TrimPrefix(mc.LocalFS.BasePath, "/")
			uri.WriteString("/")
			uri.WriteString(basePath)
		}
	} else {
		// For cloud providers, host is bucket name
		if mc.Bucket != "" {
			uri.WriteString(mc.Bucket)
		}

		// Path is the prefix
		if mc.Prefix != "" {
			uri.WriteString("/")
			uri.WriteString(mc.Prefix)
		}
	}

	// Add common parameters
	if mc.Region != "" {
		params.Set("region-id", mc.Region)
	}
	if mc.Endpoint != "" {
		params.Set("endpoint", mc.Endpoint)
	}
	if mc.SharedPoolID != "" {
		params.Set("shared-pool-id", mc.SharedPoolID)
	}

	// Add provider-specific parameters
	switch mc.Type {
	case storage.ProviderTypeS3:
		if mc.AWS != nil {
			if mc.AWS.AccessKey != "" {
				params.Set("access-key", mc.AWS.AccessKey)
			}
			if mc.AWS.SecretAccessKey != "" {
				params.Set("secret-access-key", mc.AWS.SecretAccessKey)
			}
			if mc.AWS.SessionToken != "" {
				params.Set("session-token", mc.AWS.SessionToken)
			}
			if mc.AWS.AssumeRoleARN != "" {
				params.Set("assume-role-arn", mc.AWS.AssumeRoleARN)
			}
			if mc.AWS.S3ForcePathStyle {
				params.Set("s3-force-path-style", "true")
			}
		}

	case storage.ProviderTypeOSS:
		if mc.OSS != nil {
			if mc.OSS.AccessKey != "" {
				params.Set("access-key", mc.OSS.AccessKey)
			}
			if mc.OSS.SecretAccessKey != "" {
				params.Set("secret-access-key", mc.OSS.SecretAccessKey)
			}
			if mc.OSS.SessionToken != "" {
				params.Set("session-token", mc.OSS.SessionToken)
			}
			if mc.OSS.AssumeRoleARN != "" {
				params.Set("assume-role-arn", mc.OSS.AssumeRoleARN)
			}
		}

	case storage.ProviderTypeCOS:
		if mc.COS != nil {
			if mc.COS.AccessKey != "" {
				params.Set("access-key", mc.COS.AccessKey)
			}
			if mc.COS.SecretAccessKey != "" {
				params.Set("secret-access-key", mc.COS.SecretAccessKey)
			}
			if mc.COS.SessionToken != "" {
				params.Set("session-token", mc.COS.SessionToken)
			}
			if mc.COS.AssumeRoleARN != "" {
				params.Set("assume-role-arn", mc.COS.AssumeRoleARN)
			}
		}

	case storage.ProviderTypeAzure:
		if mc.Azure != nil {
			if mc.Azure.AccountName != "" {
				params.Set("account-name", mc.Azure.AccountName)
			}
			if mc.Azure.AccountKey != "" {
				params.Set("account-key", mc.Azure.AccountKey)
			}
			if mc.Azure.SASToken != "" {
				params.Set("sas-token", mc.Azure.SASToken)
			}
		}

	case storage.ProviderTypeLocalFS:
		if mc.LocalFS != nil {
			if !mc.LocalFS.CreateDirs {
				params.Set("create-dirs", "false")
			}
			if mc.LocalFS.Permissions != "" {
				params.Set("permissions", mc.LocalFS.Permissions)
			}
		}
	}

	// Add query parameters if any exist
	if len(params) > 0 {
		uri.WriteString("?")
		uri.WriteString(params.Encode())
	}

	return uri.String()
}
