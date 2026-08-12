package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tchttp "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/http"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/regions"
	"github.com/tencentyun/cos-go-sdk-v5"
)

const (
	defaultCOSAssumeRoleSessionName = "metering-writer"
	cosAssumeRoleDuration           = 7200 * time.Second
)

var (
	assumeTencentCloudRole = assumeTencentCloudRoleWithSTS
)

func newTencentCloudDefaultProviderChain() common.Provider {
	providers := []common.Provider{
		common.DefaultEnvProvider(),
	}

	oidc, err := common.DefaultTkeOIDCRoleArnProvider()
	if err == nil {
		providers = append(providers, oidc)
	}
	providers = append(providers,
		common.DefaultProfileProvider(),
		common.DefaultCvmRoleProvider(),
	)

	return common.NewProviderChain(providers)
}

// COSProvider TencentCloud COS storage provider implementation.
type COSProvider struct {
	client *cos.Client
	prefix string
}

// NewCOSProvider creates a new COS storage provider.
func NewCOSProvider(providerConfig *ProviderConfig) (*COSProvider, error) {
	if providerConfig.Type != ProviderTypeCOS {
		return nil, fmt.Errorf("invalid provider type: %s, expected: %s", providerConfig.Type, ProviderTypeCOS)
	}
	if providerConfig.Bucket == "" {
		return nil, fmt.Errorf("bucket name is required for COS provider")
	}

	bucketURL, err := buildCOSBucketURL(providerConfig.Bucket, providerConfig.Region, providerConfig.Endpoint)
	if err != nil {
		return nil, err
	}

	credentialProvider := newTencentCloudCOSCredentialsProvider(providerConfig.COS)
	client := cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Transport: &tencentCloudCOSAuthorizationTransport{
			credentialProvider: credentialProvider,
		},
	})

	return &COSProvider{
		client: client,
		prefix: providerConfig.Prefix,
	}, nil
}

func buildCOSBucketURL(bucket, region, endpoint string) (*url.URL, error) {
	if endpoint == "" {
		if region == "" {
			return nil, fmt.Errorf("region is required for COS provider")
		}
		endpoint = fmt.Sprintf("https://%s.cos.%s.myqcloud.com", bucket, region)
	} else if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid COS endpoint: %w", err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid COS endpoint: host is empty")
	}
	if !strings.HasPrefix(u.Host, bucket+".") {
		u.Host = bucket + "." + u.Host
	}
	return u, nil
}

// buildPath builds the complete path with prefix.
func (c *COSProvider) buildPath(path string) string {
	if c.prefix == "" {
		return path
	}
	prefix := strings.TrimSuffix(c.prefix, "/")
	path = strings.TrimPrefix(path, "/")
	return prefix + "/" + path
}

// Upload implements ObjectStorageProvider interface.
func (c *COSProvider) Upload(ctx context.Context, path string, data io.Reader) error {
	fullPath := c.buildPath(path)
	_, err := c.client.Object.Put(ctx, fullPath, data, nil)
	return err
}

// Download implements ObjectStorageProvider interface.
func (c *COSProvider) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath := c.buildPath(path)
	result, err := c.client.Object.Get(ctx, fullPath, nil)
	if err != nil {
		return nil, err
	}
	return result.Body, nil
}

// Delete implements ObjectStorageProvider interface.
func (c *COSProvider) Delete(ctx context.Context, path string) error {
	fullPath := c.buildPath(path)
	_, err := c.client.Object.Delete(ctx, fullPath)
	return err
}

// Exists implements ObjectStorageProvider interface.
func (c *COSProvider) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := c.buildPath(path)
	_, err := c.client.Object.Head(ctx, fullPath, nil)
	if err != nil {
		if isCOSNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// List implements ObjectStorageProvider interface.
func (c *COSProvider) List(ctx context.Context, prefix string) ([]string, error) {
	fullPrefix := c.buildPath(prefix)
	var marker string
	var objects []string
	for {
		result, _, err := c.client.Bucket.Get(ctx, &cos.BucketGetOptions{
			Prefix: fullPrefix,
			Marker: marker,
		})
		if err != nil {
			return nil, err
		}
		for _, object := range result.Contents {
			objects = append(objects, object.Key)
		}
		if !result.IsTruncated {
			break
		}
		marker = result.NextMarker
	}
	return objects, nil
}

func isCOSNotFound(err error) bool {
	var cosErr *cos.ErrorResponse
	return errors.As(err, &cosErr) && cosErr.Response != nil && cosErr.Response.StatusCode == http.StatusNotFound
}

type tencentCloudCOSCredentialsProvider interface {
	GetCredential(ctx context.Context) (common.CredentialIface, error)
}

type tencentCloudStaticCredentialsProvider struct {
	credential common.CredentialIface
}

func (p *tencentCloudStaticCredentialsProvider) GetCredential(ctx context.Context) (common.CredentialIface, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return p.credential, nil
}

type tencentCloudDefaultCredentialsProvider struct {
	mu         sync.Mutex
	provider   common.Provider
	credential common.CredentialIface
}

func newTencentCloudCOSCredentialsProvider(cfg *COSConfig) tencentCloudCOSCredentialsProvider {
	var baseProvider tencentCloudCOSCredentialsProvider
	if cfg != nil && cfg.AccessKey != "" && cfg.SecretAccessKey != "" {
		baseProvider = &tencentCloudStaticCredentialsProvider{
			credential: common.NewTokenCredential(cfg.AccessKey, cfg.SecretAccessKey, cfg.SessionToken),
		}
	} else {
		baseProvider = &tencentCloudDefaultCredentialsProvider{
			provider: newTencentCloudDefaultProviderChain(),
		}
	}

	if cfg == nil || cfg.AssumeRoleARN == "" {
		return baseProvider
	}

	return &tencentCloudAssumeRoleCredentialsProvider{
		baseProvider: baseProvider,
		roleARN:      cfg.AssumeRoleARN,
		duration:     cosAssumeRoleDuration,
	}
}

func (p *tencentCloudDefaultCredentialsProvider) GetCredential(ctx context.Context) (common.CredentialIface, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.credential != nil {
		return p.credential, nil
	}

	cred, err := p.provider.GetCredential()
	if err != nil {
		return nil, err
	}
	if cred == nil {
		return nil, fmt.Errorf("credential provider returned nil credentials")
	}

	// Dynamic credentials returned by the TencentCloud SDK refresh themselves
	// before expiration. Cache the credential object so each COS request does
	// not resolve the provider chain and call STS again.
	p.credential = cred
	return p.credential, nil
}

type tencentCloudAssumeRoleCredentialsProvider struct {
	mu           sync.Mutex
	baseProvider tencentCloudCOSCredentialsProvider
	roleARN      string
	duration     time.Duration

	credential common.CredentialIface
	expiresAt  time.Time
}

func (p *tencentCloudAssumeRoleCredentialsProvider) GetCredential(ctx context.Context) (common.CredentialIface, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	refreshAt := p.expiresAt.Add(-p.duration / 10)
	if p.credential != nil && time.Now().Before(refreshAt) {
		return p.credential, nil
	}

	baseCred, err := p.baseProvider.GetCredential(ctx)
	if err != nil {
		return p.credentialAfterRefreshError(ctx, err)
	}

	result, err := assumeTencentCloudRole(ctx, baseCred, p.roleARN, defaultCOSAssumeRoleSessionName, p.duration)
	if err != nil {
		return p.credentialAfterRefreshError(ctx, err)
	}

	p.credential = common.NewTokenCredential(result.tmpSecretID, result.tmpSecretKey, result.token)
	p.expiresAt = result.expiresAt
	if p.expiresAt.IsZero() || p.expiresAt.Unix() <= 0 {
		p.expiresAt = time.Now().Add(p.duration)
	}
	return p.credential, nil
}

// credentialAfterRefreshError returns an existing credential while it is
// still valid. Once it has expired, the refresh error must be surfaced.
func (p *tencentCloudAssumeRoleCredentialsProvider) credentialAfterRefreshError(ctx context.Context, refreshErr error) (common.CredentialIface, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.credential != nil && time.Now().Before(p.expiresAt) {
		return p.credential, nil
	}
	return nil, refreshErr
}

type tencentCloudAssumeRoleResult struct {
	tmpSecretID  string
	tmpSecretKey string
	token        string
	expiresAt    time.Time
}

func assumeTencentCloudRoleWithSTS(ctx context.Context, baseCred common.CredentialIface, roleARN, roleSessionName string, duration time.Duration) (*tencentCloudAssumeRoleResult, error) {
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "sts.tencentcloudapi.com"
	cpf.HttpProfile.ReqMethod = "POST"

	client := common.NewCommonClient(baseCred, regions.Guangzhou, cpf)
	request := tchttp.NewCommonRequest("sts", "2018-08-13", "AssumeRole")
	request.SetContext(ctx)
	if err := request.SetActionParameters(map[string]interface{}{
		"RoleArn":         roleARN,
		"RoleSessionName": roleSessionName,
		"DurationSeconds": int64(duration / time.Second),
	}); err != nil {
		return nil, err
	}

	response := tchttp.NewCommonResponse()
	if err := client.Send(request, response); err != nil {
		return nil, err
	}

	var stsResponse struct {
		Response struct {
			Credentials struct {
				Token        string `json:"Token"`
				TmpSecretID  string `json:"TmpSecretId"`
				TmpSecretKey string `json:"TmpSecretKey"`
			} `json:"Credentials"`
			ExpiredTime int64 `json:"ExpiredTime"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(response.GetBody(), &stsResponse); err != nil {
		return nil, err
	}

	credentials := stsResponse.Response.Credentials
	if credentials.TmpSecretID == "" || credentials.TmpSecretKey == "" || credentials.Token == "" {
		return nil, fmt.Errorf("TencentCloud STS returned empty credentials")
	}

	return &tencentCloudAssumeRoleResult{
		tmpSecretID:  credentials.TmpSecretID,
		tmpSecretKey: credentials.TmpSecretKey,
		token:        credentials.Token,
		expiresAt:    time.Unix(stsResponse.Response.ExpiredTime, 0),
	}, nil
}

type tencentCloudCOSAuthorizationTransport struct {
	credentialProvider tencentCloudCOSCredentialsProvider
	Transport          http.RoundTripper
}

func (t *tencentCloudCOSAuthorizationTransport) GetCredential() (string, string, string, error) {
	return t.getCredential(context.Background())
}

func (t *tencentCloudCOSAuthorizationTransport) getCredential(ctx context.Context) (string, string, string, error) {
	cred, err := t.credentialProvider.GetCredential(ctx)
	if err != nil {
		return "", "", "", err
	}
	secretID, secretKey, token := cred.GetCredential()
	return secretID, secretKey, token, nil
}

func (t *tencentCloudCOSAuthorizationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	secretID, secretKey, token, err := t.getCredential(req.Context())
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(secretID, " ") || strings.HasSuffix(secretID, " ") {
		return nil, fmt.Errorf("SecretID is invalid")
	}
	if strings.HasPrefix(secretKey, " ") || strings.HasSuffix(secretKey, " ") {
		return nil, fmt.Errorf("SecretKey is invalid")
	}

	clone := req.Clone(req.Context())
	cos.AddAuthorizationHeader(secretID, secretKey, token, clone, cos.NewAuthTime(time.Hour))
	return t.transport().RoundTrip(clone)
}

func (t *tencentCloudCOSAuthorizationTransport) transport() http.RoundTripper {
	if t.Transport != nil {
		return t.Transport
	}
	return http.DefaultTransport
}
