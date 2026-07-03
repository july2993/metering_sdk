package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
			name:     "keeps service endpoint unchanged",
			bucket:   "metering-123456",
			endpoint: "cos.ap-beijing.myqcloud.com",
			want:     "https://cos.ap-beijing.myqcloud.com",
		},
		{
			name:     "keeps custom endpoint unchanged",
			bucket:   "metering-123456",
			endpoint: "http://127.0.0.1:9000",
			want:     "http://127.0.0.1:9000",
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

	_, err = transport.RoundTrip(req)
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
