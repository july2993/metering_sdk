package provider

// ProviderType storage provider type
type ProviderType string

const (
	// ProviderTypeS3 AWS S3 storage provider
	ProviderTypeS3 ProviderType = "s3"
	// ProviderTypeGCS Google Cloud Storage provider
	ProviderTypeGCS ProviderType = "gcs"
	// ProviderTypeAzure Azure Blob Storage provider
	ProviderTypeAzure ProviderType = "azure"
	// ProviderTypeOSS Alibaba Cloud OSS storage provider
	ProviderTypeOSS ProviderType = "oss"
	// ProviderTypeCOS TencentCloud COS storage provider
	ProviderTypeCOS ProviderType = "cos"
	// ProviderTypeLocalFS local filesystem storage provider
	ProviderTypeLocalFS ProviderType = "localfs"
)

// ProviderConfig storage provider configuration
type ProviderConfig struct {
	Type     ProviderType `json:"type"`
	Prefix   string       `json:"prefix,omitempty"`   // path prefix, all write paths will add this prefix
	Region   string       `json:"region,omitempty"`   // common region configuration
	Bucket   string       `json:"bucket,omitempty"`   // common bucket/container name
	Endpoint string       `json:"endpoint,omitempty"` // common endpoint configuration

	// Specific provider configurations
	AWS     *AWSConfig     `json:"aws,omitempty"`     // AWS S3 specific configuration
	GCS     *GCSConfig     `json:"gcs,omitempty"`     // Google Cloud Storage specific configuration
	Azure   *AzureConfig   `json:"azure,omitempty"`   // Azure Blob Storage specific configuration
	OSS     *OSSConfig     `json:"oss,omitempty"`     // Alibaba Cloud OSS specific configuration
	COS     *COSConfig     `json:"cos,omitempty"`     // TencentCloud COS specific configuration
	LocalFS *LocalFSConfig `json:"localfs,omitempty"` // local filesystem specific configuration
}

// AWSConfig AWS S3 specific configuration
type AWSConfig struct {
	S3ForcePathStyle bool   `json:"s3_force_path_style,omitempty"`
	AssumeRoleARN    string `json:"assume_role_arn,omitempty"`
	AccessKey        string `json:"access_key,omitempty"`
	SecretAccessKey  string `json:"secret_access_key,omitempty"`
	SessionToken     string `json:"session_token,omitempty"`
	// Custom AWS Config object for aws-sdk-go-v2
	CustomConfig interface{} `json:"-"` // not serialized, used to pass aws.Config
}

// GCSConfig Google Cloud Storage specific configuration
type GCSConfig struct {
	ProjectID       string `json:"project_id,omitempty"`
	CredentialsFile string `json:"credentials_file,omitempty"`
}

// AzureConfig Azure Blob Storage specific configuration
type AzureConfig struct {
	AccountName string `json:"account_name,omitempty"`
	AccountKey  string `json:"account_key,omitempty"`
	SASToken    string `json:"sas_token,omitempty"`
}

// OSSConfig Alibaba Cloud OSS specific configuration
type OSSConfig struct {
	AssumeRoleARN   string `json:"assume_role_arn,omitempty"`
	AccessKey       string `json:"access_key,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	SessionToken    string `json:"session_token,omitempty"`
	// Custom OSS Config object for oss-sdk-go-v2
	CustomConfig interface{} `json:"-"` // not serialized, used to pass oss config
}

// COSConfig TencentCloud COS specific configuration
type COSConfig struct {
	AssumeRoleARN   string `json:"assume_role_arn,omitempty"`
	AccessKey       string `json:"access_key,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	SessionToken    string `json:"session_token,omitempty"`
}

// LocalFSConfig local filesystem specific configuration
type LocalFSConfig struct {
	BasePath    string `json:"base_path"`             // base path for local filesystem
	CreateDirs  bool   `json:"create_dirs,omitempty"` // whether to automatically create directories, default true
	Permissions string `json:"permissions,omitempty"` // file permissions, e.g. "0755"
}
