package config

import "github.com/spf13/pflag"

type SubroutineToggle struct {
	Enabled bool
}

type SubroutinesConfig struct {
	CreateMCP                    SubroutineToggle
	SetupSyncAgent               SubroutineToggle
	InitializePublishedResources SubroutineToggle
	CreateCrossplane             SubroutineToggle
	DeployContentConfigurations  SubroutineToggle
}

type KCPConfig struct {
	Kubeconfig                 string
	APIExportEndpointSliceName string
	PlatformMeshIP             string
	HostOverride               string
}

type MCPConfig struct {
	Kubeconfig     string
	IssuerURL      string
	ServiceAccount string
	Namespace      string
	HostOverride   string
}

type OperatorConfig struct {
	Subroutines      SubroutinesConfig
	KCP              KCPConfig
	MCP              MCPConfig
	RuntimeNamespace string
}

func NewOperatorConfig() OperatorConfig {
	return OperatorConfig{
		Subroutines: SubroutinesConfig{
			CreateMCP:                    SubroutineToggle{Enabled: true},
			SetupSyncAgent:               SubroutineToggle{Enabled: true},
			InitializePublishedResources: SubroutineToggle{Enabled: true},
			CreateCrossplane:             SubroutineToggle{Enabled: true},
			DeployContentConfigurations:  SubroutineToggle{Enabled: true},
		},
		KCP: KCPConfig{
			APIExportEndpointSliceName: "openmcp.cloud",
		},
		MCP: MCPConfig{
			ServiceAccount: "hsp",
			Namespace:      "default",
		},
		RuntimeNamespace: "account",
	}
}

func (c *OperatorConfig) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&c.Subroutines.CreateMCP.Enabled, "subroutines-create-mcp-enabled", c.Subroutines.CreateMCP.Enabled, "Enable CreateMCP subroutine")
	fs.BoolVar(&c.Subroutines.SetupSyncAgent.Enabled, "subroutines-setup-sync-agent-enabled", c.Subroutines.SetupSyncAgent.Enabled, "Enable SetupSyncAgent subroutine")
	fs.BoolVar(&c.Subroutines.InitializePublishedResources.Enabled, "subroutines-initialize-published-resources-enabled", c.Subroutines.InitializePublishedResources.Enabled, "Enable InitializePublishedResources subroutine")
	fs.BoolVar(&c.Subroutines.CreateCrossplane.Enabled, "subroutines-create-crossplane-enabled", c.Subroutines.CreateCrossplane.Enabled, "Enable CreateCrossplane subroutine")
	fs.BoolVar(&c.Subroutines.DeployContentConfigurations.Enabled, "subroutines-deploy-content-configurations-enabled", c.Subroutines.DeployContentConfigurations.Enabled, "Enable DeployContentConfigurations subroutine")
	fs.StringVar(&c.KCP.Kubeconfig, "kcp-kubeconfig", c.KCP.Kubeconfig, "Path to the KCP kubeconfig file")
	fs.StringVar(&c.KCP.APIExportEndpointSliceName, "kcp-api-export-endpoint-slice-name", c.KCP.APIExportEndpointSliceName, "APIExportEndpointSlice name to watch")
	fs.StringVar(&c.KCP.PlatformMeshIP, "kcp-platform-mesh-ip", c.KCP.PlatformMeshIP, "Docker IP of platform-mesh control plane")
	fs.StringVar(&c.KCP.HostOverride, "kcp-host-override", c.KCP.HostOverride, "Override host:port in KCP endpoint URLs")
	fs.StringVar(&c.MCP.Kubeconfig, "mcp-kubeconfig", c.MCP.Kubeconfig, "Path to the MCP kubeconfig file")
	fs.StringVar(&c.MCP.IssuerURL, "mcp-issuer-url", c.MCP.IssuerURL, "OIDC issuer URL for MCP authentication")
	fs.StringVar(&c.MCP.ServiceAccount, "mcp-service-account", c.MCP.ServiceAccount, "Service account name for MCP operations")
	fs.StringVar(&c.MCP.Namespace, "mcp-namespace", c.MCP.Namespace, "Namespace for MCP resources")
	fs.StringVar(&c.MCP.HostOverride, "mcp-host-override", c.MCP.HostOverride, "Override host in MCP kubeconfig")
	fs.StringVar(&c.RuntimeNamespace, "runtime-namespace", c.RuntimeNamespace, "Namespace for runtime resources")
}
