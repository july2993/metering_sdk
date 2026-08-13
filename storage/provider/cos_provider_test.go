package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentyun/cos-go-sdk-v5"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type recordingCOSCredentialsProvider struct {
	ctx   context.Context
	calls int
}

func (p *recordingCOSCredentialsProvider) GetCredential(ctx context.Context) (common.CredentialIface, error) {
	p.ctx = ctx
	p.calls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return common.NewTokenCredential("sid", "skey", "token"), nil
}

type recordingTencentCloudProvider struct {
	mu         sync.Mutex
	credential common.CredentialIface
	errs       []error
	calls      int
}

func (p *recordingTencentCloudProvider) GetCredential() (common.CredentialIface, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.calls++
	if p.calls <= len(p.errs) && p.errs[p.calls-1] != nil {
		return nil, p.errs[p.calls-1]
	}
	return p.credential, nil
}

func (p *recordingTencentCloudProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

type trackingTencentCloudCredential struct {
	mu              sync.Mutex
	individualCalls int
	tupleCalls      int
}

func (c *trackingTencentCloudCredential) GetSecretId() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.individualCalls++
	return "sid"
}

func (c *trackingTencentCloudCredential) GetSecretKey() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.individualCalls++
	return "skey"
}

func (c *trackingTencentCloudCredential) GetToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.individualCalls++
	return "token"
}

func (c *trackingTencentCloudCredential) GetCredential() (string, string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tupleCalls++
	return "sid", "skey", "token"
}

func (c *trackingTencentCloudCredential) callCounts() (int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.individualCalls, c.tupleCalls
}

func TestBuildCOSBucketURL(t *testing.T) {
	tests := []struct {
		name     string
		bucket   string
		region   string
		endpoint string
		want     string
	}{
		{
			name:   "builds regional endpoint",
			bucket: "metering-123456",
			region: "ap-beijing",
			want:   "https://metering-123456.cos.ap-beijing.myqcloud.com",
		},
		{
			name:     "adds bucket to endpoint",
			bucket:   "metering-123456",
			endpoint: "cos.ap-beijing.myqcloud.com",
			want:     "https://metering-123456.cos.ap-beijing.myqcloud.com",
		},
		{
			name:     "keeps bucket endpoint",
			bucket:   "metering-123456",
			endpoint: "https://metering-123456.cos.ap-beijing.myqcloud.com",
			want:     "https://metering-123456.cos.ap-beijing.myqcloud.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildCOSBucketURL(tt.bucket, tt.region, tt.endpoint)
			require.NoError(t, err)
			require.Equal(t, tt.want, got.String())
		})
	}
}

func TestBuildCOSBucketURLRequiresRegionWithoutEndpoint(t *testing.T) {
	_, err := buildCOSBucketURL("metering-123456", "", "")
	require.ErrorContains(t, err, "region is required for COS provider")
}

func TestIsCOSNotFound(t *testing.T) {
	newCOSError := func(statusCode int, rawURL, code string) error {
		req, err := http.NewRequest(http.MethodHead, rawURL, nil)
		require.NoError(t, err)
		return &cos.ErrorResponse{
			Response: &http.Response{
				StatusCode: statusCode,
				Header:     make(http.Header),
				Request:    req,
			},
			Code: code,
		}
	}

	require.True(t, isCOSNotFound(newCOSError(
		http.StatusNotFound,
		"https://metering-123456.cos.ap-beijing.myqcloud.com/missing",
		"NoSuchKey",
	)))
	require.False(t, isCOSNotFound(newCOSError(
		http.StatusForbidden,
		"https://metering-404.cos.ap-beijing.myqcloud.com/object",
		"AccessDenied",
	)))
	require.False(t, isCOSNotFound(errors.New("upstream returned 404")))
}

func TestTencentCloudCOSCredentialsProviderStatic(t *testing.T) {
	credentialProvider := newTencentCloudCOSCredentialsProvider(&COSConfig{
		AccessKey:       "sid",
		SecretAccessKey: "skey",
		SessionToken:    "token",
	})

	credential, err := credentialProvider.GetCredential(context.Background())
	require.NoError(t, err)
	require.Equal(t, "sid", credential.GetSecretId())
	require.Equal(t, "skey", credential.GetSecretKey())
	require.Equal(t, "token", credential.GetToken())
}

func TestTencentCloudDefaultCredentialsProviderCachesCredential(t *testing.T) {
	credential := common.NewTokenCredential("sid", "skey", "token")
	underlying := &recordingTencentCloudProvider{credential: credential}
	provider := &tencentCloudDefaultCredentialsProvider{provider: underlying}

	first, err := provider.GetCredential(context.Background())
	require.NoError(t, err)
	second, err := provider.GetCredential(context.Background())
	require.NoError(t, err)

	require.Same(t, credential, first)
	require.Same(t, first, second)
	require.Equal(t, 1, underlying.callCount())
}

func TestTencentCloudDefaultCredentialsProviderRetriesAfterError(t *testing.T) {
	temporaryErr := errors.New("temporary STS error")
	credential := common.NewTokenCredential("sid", "skey", "token")
	underlying := &recordingTencentCloudProvider{
		credential: credential,
		errs:       []error{temporaryErr},
	}
	provider := &tencentCloudDefaultCredentialsProvider{provider: underlying}

	_, err := provider.GetCredential(context.Background())
	require.ErrorIs(t, err, temporaryErr)

	got, err := provider.GetCredential(context.Background())
	require.NoError(t, err)
	require.Same(t, credential, got)
	require.Equal(t, 2, underlying.callCount())
}

func TestTencentCloudDefaultCredentialsProviderConcurrentInitialization(t *testing.T) {
	credential := common.NewTokenCredential("sid", "skey", "token")
	underlying := &recordingTencentCloudProvider{credential: credential}
	provider := &tencentCloudDefaultCredentialsProvider{provider: underlying}

	const goroutines = 100
	start := make(chan struct{})
	errs := make(chan error, goroutines)
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			got, err := provider.GetCredential(context.Background())
			if err == nil && got != credential {
				err = fmt.Errorf("unexpected credential: %T", got)
			}
			errs <- err
		}()
	}

	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, 1, underlying.callCount())
}

func TestTencentCloudCOSAuthorizationTransportReadsCredentialAtomically(t *testing.T) {
	credential := &trackingTencentCloudCredential{}
	transport := &tencentCloudCOSAuthorizationTransport{
		credentialProvider: &tencentCloudStaticCredentialsProvider{credential: credential},
	}

	secretID, secretKey, token, err := transport.getCredential(context.Background())
	require.NoError(t, err)
	require.Equal(t, "sid", secretID)
	require.Equal(t, "skey", secretKey)
	require.Equal(t, "token", token)

	individualCalls, tupleCalls := credential.callCounts()
	require.Zero(t, individualCalls)
	require.Equal(t, 1, tupleCalls)
}

func TestTencentCloudCOSCredentialsProviderAssumeRole(t *testing.T) {
	origAssumeRole := assumeTencentCloudRole
	t.Cleanup(func() {
		assumeTencentCloudRole = origAssumeRole
	})

	var assumeCalls int
	var gotRoleARN string
	var gotBase common.CredentialIface
	assumeTencentCloudRole = func(ctx context.Context, baseCred common.CredentialIface, roleARN, roleSessionName string, duration time.Duration) (*tencentCloudAssumeRoleResult, error) {
		require.NoError(t, ctx.Err())
		assumeCalls++
		gotBase = baseCred
		gotRoleARN = roleARN
		require.Equal(t, defaultCOSAssumeRoleSessionName, roleSessionName)
		require.Equal(t, cosAssumeRoleDuration, duration)
		return &tencentCloudAssumeRoleResult{
			tmpSecretID:  "tmp-id",
			tmpSecretKey: "tmp-key",
			token:        "tmp-token",
			expiresAt:    time.Now().Add(time.Hour),
		}, nil
	}

	credentialProvider := newTencentCloudCOSCredentialsProvider(&COSConfig{
		AccessKey:       "base-id",
		SecretAccessKey: "base-key",
		SessionToken:    "base-token",
		AssumeRoleARN:   "qcs::cam::uin/123456:roleName/metering",
	})

	credential, err := credentialProvider.GetCredential(context.Background())
	require.NoError(t, err)
	require.Equal(t, "tmp-id", credential.GetSecretId())
	require.Equal(t, "tmp-key", credential.GetSecretKey())
	require.Equal(t, "tmp-token", credential.GetToken())

	credential, err = credentialProvider.GetCredential(context.Background())
	require.NoError(t, err)
	require.Equal(t, "tmp-id", credential.GetSecretId())

	require.Equal(t, 1, assumeCalls)
	require.Equal(t, "base-id", gotBase.GetSecretId())
	require.Equal(t, "qcs::cam::uin/123456:roleName/metering", gotRoleARN)
}

func TestTencentCloudAssumeRoleCredentialsProviderUsesValidCredentialWhenRefreshFails(t *testing.T) {
	origAssumeRole := assumeTencentCloudRole
	t.Cleanup(func() {
		assumeTencentCloudRole = origAssumeRole
	})

	refreshErr := errors.New("temporary STS error")
	assumeCalls := 0
	assumeTencentCloudRole = func(context.Context, common.CredentialIface, string, string, time.Duration) (*tencentCloudAssumeRoleResult, error) {
		assumeCalls++
		if assumeCalls > 1 {
			return nil, refreshErr
		}
		return &tencentCloudAssumeRoleResult{
			tmpSecretID:  "tmp-id",
			tmpSecretKey: "tmp-key",
			token:        "tmp-token",
			// Within the refresh window, but still valid.
			expiresAt: time.Now().Add(5 * time.Minute),
		}, nil
	}

	provider := &tencentCloudAssumeRoleCredentialsProvider{
		baseProvider: &tencentCloudStaticCredentialsProvider{
			credential: common.NewCredential("base-id", "base-key"),
		},
		roleARN:  "qcs::cam::uin/123456:roleName/metering",
		duration: cosAssumeRoleDuration,
	}

	first, err := provider.GetCredential(context.Background())
	require.NoError(t, err)
	second, err := provider.GetCredential(context.Background())
	require.NoError(t, err)
	require.Same(t, first, second)
	require.Equal(t, 2, assumeCalls)
}

func TestTencentCloudAssumeRoleCredentialsProviderReturnsRefreshErrorAfterExpiration(t *testing.T) {
	origAssumeRole := assumeTencentCloudRole
	t.Cleanup(func() {
		assumeTencentCloudRole = origAssumeRole
	})

	refreshErr := errors.New("temporary STS error")
	assumeTencentCloudRole = func(context.Context, common.CredentialIface, string, string, time.Duration) (*tencentCloudAssumeRoleResult, error) {
		return nil, refreshErr
	}

	provider := &tencentCloudAssumeRoleCredentialsProvider{
		baseProvider: &tencentCloudStaticCredentialsProvider{
			credential: common.NewCredential("base-id", "base-key"),
		},
		roleARN:    "qcs::cam::uin/123456:roleName/metering",
		duration:   cosAssumeRoleDuration,
		credential: common.NewTokenCredential("expired-id", "expired-key", "expired-token"),
		expiresAt:  time.Now().Add(-time.Minute),
	}

	credential, err := provider.GetCredential(context.Background())
	require.ErrorIs(t, err, refreshErr)
	require.Nil(t, credential)
}

func TestTencentCloudAssumeRoleCredentialsProviderFallsBackForUnixEpochExpiration(t *testing.T) {
	origAssumeRole := assumeTencentCloudRole
	t.Cleanup(func() {
		assumeTencentCloudRole = origAssumeRole
	})

	assumeTencentCloudRole = func(context.Context, common.CredentialIface, string, string, time.Duration) (*tencentCloudAssumeRoleResult, error) {
		return &tencentCloudAssumeRoleResult{
			tmpSecretID:  "tmp-id",
			tmpSecretKey: "tmp-key",
			token:        "tmp-token",
			expiresAt:    time.Unix(0, 0),
		}, nil
	}

	provider := &tencentCloudAssumeRoleCredentialsProvider{
		baseProvider: &tencentCloudStaticCredentialsProvider{
			credential: common.NewCredential("base-id", "base-key"),
		},
		roleARN:  "qcs::cam::uin/123456:roleName/metering",
		duration: cosAssumeRoleDuration,
	}

	startedAt := time.Now()
	_, err := provider.GetCredential(context.Background())
	require.NoError(t, err)
	require.WithinDuration(t, startedAt.Add(cosAssumeRoleDuration), provider.expiresAt, time.Second)
}

func TestTencentCloudCOSAuthorizationTransportUsesRequestContext(t *testing.T) {
	type contextKey struct{}
	provider := &recordingCOSCredentialsProvider{}
	transport := &tencentCloudCOSAuthorizationTransport{
		credentialProvider: provider,
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatal("underlying transport should not be called when request context is canceled")
			return nil, nil
		}),
	}

	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), contextKey{}, "request-context"))
	cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://metering-123456.cos.ap-beijing.myqcloud.com/object", nil)
	require.NoError(t, err)

	resp, err := transport.RoundTrip(req)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, provider.calls)
	require.Equal(t, "request-context", provider.ctx.Value(contextKey{}))
}

func TestCOSProviderObjectOperationsUsePrefix(t *testing.T) {
	objects := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/")
		switch r.Method {
		case http.MethodPut:
			data, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			objects[key] = data
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			if r.URL.Path == "/" {
				prefix := r.URL.Query().Get("prefix")
				w.Header().Set("Content-Type", "application/xml")
				_, _ = fmt.Fprintf(w, `<ListBucketResult><IsTruncated>false</IsTruncated><Contents><Key>%sfile.txt</Key></Contents></ListBucketResult>`, prefix)
				return
			}
			data, ok := objects[key]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(data)
		case http.MethodHead:
			if _, ok := objects[key]; !ok {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			delete(objects, key)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cosClient := cos.NewClient(&cos.BaseURL{BucketURL: mustParseURL(t, server.URL)}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  "sid",
			SecretKey: "skey",
		},
	})
	cosClient.Conf.EnableCRC = false
	provider := &COSProvider{
		client: cosClient,
		prefix: "metering",
	}

	ctx := context.Background()
	require.NoError(t, provider.Upload(ctx, "file.txt", bytes.NewReader([]byte("hello"))))
	require.Equal(t, []byte("hello"), objects["metering/file.txt"])

	body, err := provider.Download(ctx, "file.txt")
	require.NoError(t, err)
	defer body.Close()
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, []byte("hello"), data)

	exists, err := provider.Exists(ctx, "file.txt")
	require.NoError(t, err)
	require.True(t, exists)

	keys, err := provider.List(ctx, "")
	require.NoError(t, err)
	require.Equal(t, []string{"metering/file.txt"}, keys)

	require.NoError(t, provider.Delete(ctx, "file.txt"))
	exists, err = provider.Exists(ctx, "file.txt")
	require.NoError(t, err)
	require.False(t, exists)
}

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return u
}
