package tool

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PreDeleteResourceCheck describes a resource type (by GVR) that must have zero
// instances in the KCP workspace before the tool's Helm release may be uninstalled.
type PreDeleteResourceCheck struct {
	Group    string // e.g. "kro.run"
	Version  string // e.g. "v1alpha1"
	Resource string // plural form, e.g. "resourcegraphdefinitions"
}

// ToolConfig parameterizes the generic subroutines for deploying a tool
// (CRDs into KCP workspace, Helm chart onto MCP cluster, content configs for UI).
type ToolConfig struct {
	Name            string
	Namespace       string // Helm install namespace on MCP cluster (e.g. "kro-system")
	FinalizerPrefix string
	HelmChartURL    string
	HelmReleaseName string
	APIExportName   string // Name for the per-tool APIExport (e.g. "kro.services.openmcp.cloud")
	SkipCRDs        bool   // Skip CRD installation via Helm (CRDs deployed separately to KCP workspace)
	HelmValuesFunc  func(version string, kcpKubeconfig string, platformMeshIP string) map[string]any
	PostInstallFunc func(ctx context.Context, mcpClient client.Client, kubeconfigSecret string, platformMeshIP string) error
	ContentConfigs  []ContentConfigEntry
	PreDeleteChecks []PreDeleteResourceCheck // Resources to check before allowing uninstall
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
