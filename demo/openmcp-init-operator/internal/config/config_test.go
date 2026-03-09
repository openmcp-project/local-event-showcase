package config

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOperatorConfig_Defaults(t *testing.T) {
	cfg := NewOperatorConfig()

	assert.True(t, cfg.Subroutines.CreateMCP.Enabled)
	assert.True(t, cfg.Subroutines.SetupSyncAgent.Enabled)
	assert.True(t, cfg.Subroutines.InitializePublishedResources.Enabled)
	assert.True(t, cfg.Subroutines.CreateCrossplane.Enabled)
	assert.True(t, cfg.Subroutines.DeployContentConfigurations.Enabled)
	assert.Equal(t, "openmcp.cloud", cfg.KCP.APIExportEndpointSliceName)
	assert.Equal(t, "hsp", cfg.MCP.ServiceAccount)
	assert.Equal(t, "default", cfg.MCP.Namespace)
	assert.Equal(t, "account", cfg.RuntimeNamespace)
	assert.Empty(t, cfg.KCP.Kubeconfig)
	assert.Empty(t, cfg.KCP.PlatformMeshIP)
	assert.Empty(t, cfg.KCP.HostOverride)
	assert.Empty(t, cfg.MCP.Kubeconfig)
	assert.Empty(t, cfg.MCP.IssuerURL)
	assert.Empty(t, cfg.MCP.HostOverride)
}

func TestOperatorConfig_AddFlags(t *testing.T) {
	cfg := NewOperatorConfig()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	cfg.AddFlags(fs)

	err := fs.Parse([]string{
		"--subroutines-create-mcp-enabled=false",
		"--kcp-kubeconfig=/path/to/kcp",
		"--kcp-platform-mesh-ip=172.18.0.2",
		"--mcp-namespace=custom",
		"--runtime-namespace=test-ns",
	})
	require.NoError(t, err)

	assert.False(t, cfg.Subroutines.CreateMCP.Enabled)
	assert.Equal(t, "/path/to/kcp", cfg.KCP.Kubeconfig)
	assert.Equal(t, "172.18.0.2", cfg.KCP.PlatformMeshIP)
	assert.Equal(t, "custom", cfg.MCP.Namespace)
	assert.Equal(t, "test-ns", cfg.RuntimeNamespace)
	// Unchanged defaults
	assert.True(t, cfg.Subroutines.SetupSyncAgent.Enabled)
}
