package tool

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ToolConfig parameterizes the generic subroutines for deploying a tool
// (CRDs into KCP workspace, Helm chart onto MCP cluster, content configs for UI).
type ToolConfig struct {
	Name            string
	Namespace       string // Helm install namespace on MCP cluster (e.g. "kro-system")
	FinalizerPrefix string
	HelmChartURL    string
	HelmReleaseName string
	SkipCRDs        bool // Skip CRD installation via Helm (CRDs deployed separately to KCP workspace)
	HelmValuesFunc  func(version string, kcpKubeconfig string, platformMeshIP string) map[string]any
	PostInstallFunc func(ctx context.Context, mcpClient client.Client, kubeconfigSecret string, platformMeshIP string) error
	ContentConfigs  []ContentConfigEntry
}

// ContentConfigEntry describes a single ContentConfiguration to deploy for UI navigation.
type ContentConfigEntry struct {
	Group         string
	Version       string
	Kind          string
	Plural        string
	DisplayLabel  string
	Icon          string
	Order         int
	PathSegment   string
	CategoryID    string
	CategoryLabel string
	CategoryOrder int
	Scope         string // "Cluster" or "Namespaced"
}
