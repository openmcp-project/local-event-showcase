package config

// OperatorConfig struct to hold the operator-specific config
type OperatorConfig struct {
	Subroutines struct {
		CreateMCP struct {
			Enabled bool `mapstructure:"subroutines-create-mcp-enabled" default:"true"`
		} `mapstructure:",squash"`
		SetupSyncAgent struct {
			Enabled bool `mapstructure:"subroutines-setup-sync-agent-enabled" default:"true"`
		} `mapstructure:",squash"`
		InitializePublishedResources struct {
			Enabled bool `mapstructure:"subroutines-initialize-published-resources-enabled" default:"true"`
		} `mapstructure:",squash"`
		CreateCrossplane struct {
			Enabled bool `mapstructure:"subroutines-create-crossplane-enabled" default:"true"`
		} `mapstructure:",squash"`
	} `mapstructure:",squash"`
	KCP struct {
		Kubeconfig                 string `mapstructure:"kcp-kubeconfig" description:"Path to the KCP kubeconfig file"`
		APIExportEndpointSliceName string `mapstructure:"kcp-api-export-endpoint-slice-name" default:"openmcp.cloud"`
		PlatformMeshIP             string `mapstructure:"kcp-platform-mesh-ip" required:"true" description:"Docker IP of platform-mesh control plane for cross-cluster access"`
		HostOverride               string `mapstructure:"kcp-host-override" description:"Override host:port in KCP endpoint URLs (for local kind testing)"`
	} `mapstructure:",squash"`
	MCP struct {
		Kubeconfig     string `mapstructure:"mcp-kubeconfig" description:"Path to the MCP kubeconfig file"`
		IssuerURL      string `mapstructure:"mcp-issuer-url" description:"OIDC issuer URL for MCP authentication"`
		ServiceAccount string `mapstructure:"mcp-service-account" default:"hsp" description:"Service account name for MCP operations"`
		Namespace      string `mapstructure:"mcp-namespace" default:"default" description:"Namespace for MCP resources"`
		HostOverride   string `mapstructure:"mcp-host-override" description:"Override host in MCP kubeconfig (for local testing outside cluster)"`
	} `mapstructure:",squash"`
	RuntimeNamespace string `mapstructure:"runtime-namespace" default:"account" description:"Namespace for runtime resources"`
}
