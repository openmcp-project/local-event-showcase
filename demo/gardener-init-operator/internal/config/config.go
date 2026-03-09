package config

import "github.com/spf13/pflag"

type SubroutineToggle struct {
	Enabled bool
}

type SubroutinesConfig struct {
	CreateGardenerProject SubroutineToggle
	SetupGardenerAccess   SubroutineToggle
}

type KCPConfig struct {
	Kubeconfig                 string
	APIExportEndpointSliceName string
	PlatformMeshIP             string
	HostOverride               string
}

type GardenerConfig struct {
	Kubeconfig string
	IP         string
}

type OperatorConfig struct {
	Subroutines      SubroutinesConfig
	KCP              KCPConfig
	Gardener         GardenerConfig
	RuntimeNamespace string
}

func NewOperatorConfig() OperatorConfig {
	return OperatorConfig{
		Subroutines: SubroutinesConfig{
			CreateGardenerProject: SubroutineToggle{Enabled: true},
			SetupGardenerAccess:   SubroutineToggle{Enabled: true},
		},
		KCP: KCPConfig{
			APIExportEndpointSliceName: "gardener.cloud",
		},
		RuntimeNamespace: "default",
	}
}

func (c *OperatorConfig) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&c.Subroutines.CreateGardenerProject.Enabled, "subroutines-create-gardener-project-enabled", c.Subroutines.CreateGardenerProject.Enabled, "Enable CreateGardenerProject subroutine")
	fs.BoolVar(&c.Subroutines.SetupGardenerAccess.Enabled, "subroutines-setup-gardener-access-enabled", c.Subroutines.SetupGardenerAccess.Enabled, "Enable SetupGardenerAccess subroutine")
	fs.StringVar(&c.KCP.Kubeconfig, "kcp-kubeconfig", c.KCP.Kubeconfig, "Path to the KCP kubeconfig file")
	fs.StringVar(&c.KCP.APIExportEndpointSliceName, "kcp-api-export-endpoint-slice-name", c.KCP.APIExportEndpointSliceName, "APIExportEndpointSlice name to watch")
	fs.StringVar(&c.KCP.PlatformMeshIP, "kcp-platform-mesh-ip", c.KCP.PlatformMeshIP, "Docker IP of platform-mesh control plane")
	fs.StringVar(&c.KCP.HostOverride, "kcp-host-override", c.KCP.HostOverride, "Override host:port in KCP endpoint URLs")
	fs.StringVar(&c.Gardener.Kubeconfig, "gardener-kubeconfig", c.Gardener.Kubeconfig, "Path to the Gardener kubeconfig file")
	fs.StringVar(&c.Gardener.IP, "gardener-ip", c.Gardener.IP, "Docker IP of gardener-local control plane")
	fs.StringVar(&c.RuntimeNamespace, "runtime-namespace", c.RuntimeNamespace, "Namespace for runtime resources")
}
