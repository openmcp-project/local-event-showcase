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
	DeployKROCRDs                SubroutineToggle
	InstallKRO                   SubroutineToggle
	DeployFluxCRDs               SubroutineToggle
	InstallFlux                  SubroutineToggle
	DeployOCMCRDs                SubroutineToggle
	InstallOCM                   SubroutineToggle
}

type SyncAgentConfig struct {
	ChartURL                   string
	ImageRepository            string
	ImageTag                   string
	APIExportHostPortOverrides []string
}

type KCPConfig struct {
	Kubeconfig                 string
	APIExportEndpointSliceName string
	PlatformMeshIP             string
	HostOverride               string
}

type GardenerConfig struct {
	IP string
}

type MCPConfig struct {
	Kubeconfig     string
	IssuerURL      string
	ServiceAccount string
	Namespace      string
	HostOverride   string
}

type ToolHelmConfig struct {
	ChartURL        string
	ImageRepository string
	ImageTag        string
}

type OperatorConfig struct {
	Subroutines      SubroutinesConfig
	KCP              KCPConfig
	MCP              MCPConfig
	Gardener         GardenerConfig
	SyncAgent        SyncAgentConfig
	KRO              ToolHelmConfig
	Flux             ToolHelmConfig
	OCM              ToolHelmConfig
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
			DeployKROCRDs:                SubroutineToggle{Enabled: true},
			InstallKRO:                   SubroutineToggle{Enabled: true},
			DeployFluxCRDs:               SubroutineToggle{Enabled: true},
			InstallFlux:                  SubroutineToggle{Enabled: true},
			DeployOCMCRDs:                SubroutineToggle{Enabled: true},
			InstallOCM:                   SubroutineToggle{Enabled: true},
		},
		KCP: KCPConfig{
			APIExportEndpointSliceName: "opencp.cloud",
		},
		MCP: MCPConfig{
			ServiceAccount: "hsp",
			Namespace:      "default",
		},
		SyncAgent: SyncAgentConfig{
			ChartURL: "https://github.com/kcp-dev/helm-charts/releases/download/api-syncagent-0.5.0/api-syncagent-0.5.0.tgz",
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
	fs.BoolVar(&c.Subroutines.DeployKROCRDs.Enabled, "subroutines-deploy-kro-crds-enabled", c.Subroutines.DeployKROCRDs.Enabled, "Enable DeployKROCRDs subroutine")
	fs.BoolVar(&c.Subroutines.InstallKRO.Enabled, "subroutines-install-kro-enabled", c.Subroutines.InstallKRO.Enabled, "Enable InstallKRO subroutine")
	fs.BoolVar(&c.Subroutines.DeployFluxCRDs.Enabled, "subroutines-deploy-flux-crds-enabled", c.Subroutines.DeployFluxCRDs.Enabled, "Enable DeployFluxCRDs subroutine")
	fs.BoolVar(&c.Subroutines.InstallFlux.Enabled, "subroutines-install-flux-enabled", c.Subroutines.InstallFlux.Enabled, "Enable InstallFlux subroutine")
	fs.BoolVar(&c.Subroutines.DeployOCMCRDs.Enabled, "subroutines-deploy-ocm-crds-enabled", c.Subroutines.DeployOCMCRDs.Enabled, "Enable DeployOCMCRDs subroutine")
	fs.BoolVar(&c.Subroutines.InstallOCM.Enabled, "subroutines-install-ocm-enabled", c.Subroutines.InstallOCM.Enabled, "Enable InstallOCM subroutine")
	fs.StringVar(&c.KCP.Kubeconfig, "kcp-kubeconfig", c.KCP.Kubeconfig, "Path to the KCP kubeconfig file")
	fs.StringVar(&c.KCP.APIExportEndpointSliceName, "kcp-api-export-endpoint-slice-name", c.KCP.APIExportEndpointSliceName, "APIExportEndpointSlice name to watch")
	fs.StringVar(&c.KCP.PlatformMeshIP, "kcp-platform-mesh-ip", c.KCP.PlatformMeshIP, "Docker IP of platform-mesh control plane")
	fs.StringVar(&c.KCP.HostOverride, "kcp-host-override", c.KCP.HostOverride, "Override host:port in KCP endpoint URLs")
	fs.StringVar(&c.MCP.Kubeconfig, "mcp-kubeconfig", c.MCP.Kubeconfig, "Path to the MCP kubeconfig file")
	fs.StringVar(&c.MCP.IssuerURL, "mcp-issuer-url", c.MCP.IssuerURL, "OIDC issuer URL for MCP authentication")
	fs.StringVar(&c.MCP.ServiceAccount, "mcp-service-account", c.MCP.ServiceAccount, "Service account name for MCP operations")
	fs.StringVar(&c.MCP.Namespace, "mcp-namespace", c.MCP.Namespace, "Namespace for MCP resources")
	fs.StringVar(&c.MCP.HostOverride, "mcp-host-override", c.MCP.HostOverride, "Override host in MCP kubeconfig")
	fs.StringVar(&c.Gardener.IP, "gardener-ip", c.Gardener.IP, "Docker IP of gardener-local control plane")
	fs.StringVar(&c.SyncAgent.ChartURL, "sync-agent-chart-url", c.SyncAgent.ChartURL, "Helm chart URL for the api-syncagent")
	fs.StringVar(&c.SyncAgent.ImageRepository, "sync-agent-image-repository", c.SyncAgent.ImageRepository, "Override image repository for the api-syncagent")
	fs.StringVar(&c.SyncAgent.ImageTag, "sync-agent-image-tag", c.SyncAgent.ImageTag, "Override image tag for the api-syncagent")
	fs.StringSliceVar(&c.SyncAgent.APIExportHostPortOverrides, "sync-agent-apiexport-hostport-override", c.SyncAgent.APIExportHostPortOverrides, "Override host:port in APIExportEndpointSlice URLs (format: original=new, can be specified multiple times)")
	fs.StringVar(&c.RuntimeNamespace, "runtime-namespace", c.RuntimeNamespace, "Namespace for runtime resources")
	fs.StringVar(&c.KRO.ChartURL, "kro-chart-url", c.KRO.ChartURL, "Helm chart URL for KRO")
	fs.StringVar(&c.KRO.ImageRepository, "kro-image-repository", c.KRO.ImageRepository, "Override image repository for KRO")
	fs.StringVar(&c.KRO.ImageTag, "kro-image-tag", c.KRO.ImageTag, "Override image tag for KRO")
	fs.StringVar(&c.Flux.ChartURL, "flux-chart-url", c.Flux.ChartURL, "Helm chart URL for Flux")
	fs.StringVar(&c.Flux.ImageRepository, "flux-image-repository", c.Flux.ImageRepository, "Override image repository for Flux")
	fs.StringVar(&c.Flux.ImageTag, "flux-image-tag", c.Flux.ImageTag, "Override image tag for Flux")
	fs.StringVar(&c.OCM.ChartURL, "ocm-chart-url", c.OCM.ChartURL, "Helm chart URL for OCM Controller")
	fs.StringVar(&c.OCM.ImageRepository, "ocm-image-repository", c.OCM.ImageRepository, "Override image repository for OCM Controller")
	fs.StringVar(&c.OCM.ImageTag, "ocm-image-tag", c.OCM.ImageTag, "Override image tag for OCM Controller")
}
