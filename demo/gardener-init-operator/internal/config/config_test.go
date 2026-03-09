package config

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOperatorConfig_Defaults(t *testing.T) {
	cfg := NewOperatorConfig()

	assert.True(t, cfg.Subroutines.CreateGardenerProject.Enabled)
	assert.True(t, cfg.Subroutines.SetupGardenerAccess.Enabled)
	assert.Equal(t, "gardener.cloud", cfg.KCP.APIExportEndpointSliceName)
	assert.Equal(t, "default", cfg.RuntimeNamespace)
	assert.Empty(t, cfg.KCP.Kubeconfig)
	assert.Empty(t, cfg.KCP.PlatformMeshIP)
	assert.Empty(t, cfg.KCP.HostOverride)
	assert.Empty(t, cfg.Gardener.Kubeconfig)
	assert.Empty(t, cfg.Gardener.IP)
}

func TestOperatorConfig_AddFlags(t *testing.T) {
	cfg := NewOperatorConfig()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	cfg.AddFlags(fs)

	err := fs.Parse([]string{
		"--subroutines-create-gardener-project-enabled=false",
		"--kcp-kubeconfig=/path/to/kcp",
		"--kcp-platform-mesh-ip=172.18.0.2",
		"--gardener-kubeconfig=/path/to/gardener",
		"--gardener-ip=172.18.0.5",
		"--runtime-namespace=test-ns",
	})
	require.NoError(t, err)

	assert.False(t, cfg.Subroutines.CreateGardenerProject.Enabled)
	assert.True(t, cfg.Subroutines.SetupGardenerAccess.Enabled)
	assert.Equal(t, "/path/to/kcp", cfg.KCP.Kubeconfig)
	assert.Equal(t, "172.18.0.2", cfg.KCP.PlatformMeshIP)
	assert.Equal(t, "/path/to/gardener", cfg.Gardener.Kubeconfig)
	assert.Equal(t, "172.18.0.5", cfg.Gardener.IP)
	assert.Equal(t, "test-ns", cfg.RuntimeNamespace)
}
