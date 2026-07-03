package config

import (
	"testing"

	"github.com/pingcap/metering_sdk/storage"
	"github.com/stretchr/testify/require"
)

func TestCOSConfigToProviderConfig(t *testing.T) {
	cfg := NewMeteringConfig().
		WithCOSAssumeRole("ap-beijing", "metering-123456", "qcs::cam::uin/123456:roleName/metering").
		WithPrefix("premium")

	providerCfg := cfg.ToProviderConfig()
	require.Equal(t, storage.ProviderTypeCOS, providerCfg.Type)
	require.Equal(t, "ap-beijing", providerCfg.Region)
	require.Equal(t, "metering-123456", providerCfg.Bucket)
	require.Equal(t, "premium", providerCfg.Prefix)
	require.NotNil(t, providerCfg.COS)
	require.Equal(t, "qcs::cam::uin/123456:roleName/metering", providerCfg.COS.AssumeRoleARN)
}

func TestCOSConfigURI(t *testing.T) {
	cfg, err := NewFromURI("cos://metering-123456/premium?region-id=ap-beijing&assume-role-arn=qcs::cam::uin/123456:roleName/metering")
	require.NoError(t, err)
	require.Equal(t, storage.ProviderTypeCOS, cfg.Type)
	require.Equal(t, "metering-123456", cfg.Bucket)
	require.Equal(t, "premium", cfg.Prefix)
	require.Equal(t, "ap-beijing", cfg.Region)
	require.NotNil(t, cfg.COS)
	require.Equal(t, "qcs::cam::uin/123456:roleName/metering", cfg.COS.AssumeRoleARN)

	require.Equal(t, "cos://metering-123456/premium?assume-role-arn=qcs%3A%3Acam%3A%3Auin%2F123456%3AroleName%2Fmetering&region-id=ap-beijing", cfg.ToURI())
}
