package controller

import (
	"context"
	"fmt"

	platformmeshconfig "github.com/platform-mesh/golang-commons/config"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/builder"
	mclifecycle "github.com/platform-mesh/golang-commons/controller/lifecycle/multicluster"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/logger"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	mccontext "sigs.k8s.io/multicluster-runtime/pkg/context"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	fluxv1alpha1 "github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/api/flux/v1alpha1"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/subroutines"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/tool"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/toolcrds"
)

var (
	fluxReconcilerName = "FluxReconciler"

	fluxToolConfig = tool.ToolConfig{
		Name:            "flux",
		Namespace:       "flux-system",
		FinalizerPrefix: "flux.openmcp.io",
		HelmReleaseName: "flux",
	}

	fluxContentConfigs = []tool.ContentConfigEntry{
		{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: "GitRepository", Plural: "gitrepositories", DisplayLabel: "Git Repositories", Icon: "source-code", Order: 100, PathSegment: "gitrepositories", CategoryID: "flux-sources", CategoryLabel: "Flux Sources", CategoryOrder: 810, Scope: "Namespaced"},
		{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: "HelmRepository", Plural: "helmrepositories", DisplayLabel: "Helm Repositories", Icon: "database", Order: 110, PathSegment: "helmrepositories", CategoryID: "flux-sources", CategoryLabel: "Flux Sources", CategoryOrder: 810, Scope: "Namespaced"},
		{Group: "source.toolkit.fluxcd.io", Version: "v1beta2", Kind: "OCIRepository", Plural: "ocirepositories", DisplayLabel: "OCI Repositories", Icon: "shipping-status", Order: 120, PathSegment: "ocirepositories", CategoryID: "flux-sources", CategoryLabel: "Flux Sources", CategoryOrder: 810, Scope: "Namespaced"},
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Kind: "Kustomization", Plural: "kustomizations", DisplayLabel: "Kustomizations", Icon: "customize", Order: 200, PathSegment: "kustomizations", CategoryID: "flux-delivery", CategoryLabel: "Flux Delivery", CategoryOrder: 820, Scope: "Namespaced"},
		{Group: "helm.toolkit.fluxcd.io", Version: "v2", Kind: "HelmRelease", Plural: "helmreleases", DisplayLabel: "Helm Releases", Icon: "deploy", Order: 210, PathSegment: "helmreleases", CategoryID: "flux-delivery", CategoryLabel: "Flux Delivery", CategoryOrder: 820, Scope: "Namespaced"},
	}
)

type FluxReconciler struct {
	lifecycle *mclifecycle.LifecycleManager
	log       *logger.Logger
}

func NewFluxReconciler(cfg config.OperatorConfig, mgr mcmanager.Manager, onboardingClient client.Client, log *logger.Logger) *FluxReconciler {
	var subs []subroutine.Subroutine

	provider := &mcManagerKCPAdapter{mgr: mgr}

	toolCfg := fluxToolConfig
	toolCfg.HelmChartURL = cfg.Flux.ChartURL
	toolCfg.HelmValuesFunc = func(version string, kubeconfigSecret string, platformMeshIP string) map[string]any {
		values := map[string]any{
			"installCRDs": false,
		}
		if platformMeshIP != "" {
			values["hostAliases"] = map[string]any{
				"enabled": true,
				"values": []any{
					map[string]any{
						"ip":        platformMeshIP,
						"hostnames": []any{"localhost"},
					},
				},
			}
		}
		if cfg.Flux.ImageRepository != "" {
			image := map[string]any{"repository": cfg.Flux.ImageRepository}
			if cfg.Flux.ImageTag != "" {
				image["tag"] = cfg.Flux.ImageTag
			}
			values["image"] = image
		}
		return values
	}
	toolCfg.PostInstallFunc = fluxPostInstall

	if cfg.Subroutines.DeployFluxCRDs.Enabled {
		subs = append(subs, subroutines.NewDeployCRDsSubroutine(provider, "flux", toolcrds.FluxCRDs, "flux.openmcp.io/managed-crds"))
	}
	if cfg.Subroutines.InstallFlux.Enabled {
		subs = append(subs, subroutines.NewInstallToolSubroutine(provider, onboardingClient, &cfg, &toolCfg))
	}
	if cfg.Subroutines.DeployContentConfigurations.Enabled {
		subs = append(subs, subroutines.NewDeployToolContentConfigurationsSubroutine(provider, "flux", "services.openmcp.cloud", fluxContentConfigs, "flux.openmcp.io/managed-content-configurations"))
	}

	return &FluxReconciler{
		lifecycle: builder.NewBuilder(operatorName, fluxReconcilerName, subs, log).
			BuildMultiCluster(mgr),
		log: log,
	}
}

func (r *FluxReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	return r.lifecycle.Reconcile(mccontext.WithCluster(ctx, req.ClusterName), req, &fluxv1alpha1.Flux{})
}

func (r *FluxReconciler) SetupWithManager(mgr mcmanager.Manager, cfg *platformmeshconfig.CommonServiceConfig, log *logger.Logger, eventPredicates ...predicate.Predicate) error {
	return r.lifecycle.SetupWithManager(mgr, cfg.MaxConcurrentReconciles, fluxReconcilerName, &fluxv1alpha1.Flux{}, cfg.DebugLabelValue, r, log, eventPredicates...)
}

// fluxPostInstall patches the Flux controller Deployments to mount the kcp-kubeconfig secret
// and set the KUBECONFIG env var so Flux controllers target the KCP workspace.
// The Flux community helm chart only supports extraSecretMounts for the kustomize-controller,
// so we patch all controller Deployments after helm install.
func fluxPostInstall(ctx context.Context, mcpClient client.Client, kubeconfigSecret string, platformMeshIP string) error {
	fluxDeployments := []string{
		"source-controller",
		"kustomize-controller",
		"helm-controller",
		"notification-controller",
		"image-reflector-controller",
		"image-automation-controller",
	}

	volumeName := "kcp-kubeconfig"
	mountPath := "/etc/flux/kcp"
	kubeconfigPath := mountPath + "/kubeconfig"

	for _, name := range fluxDeployments {
		deploy := &appsv1.Deployment{}
		if err := mcpClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, deploy); err != nil {
			return fmt.Errorf("failed to get deployment %s: %w", name, err)
		}

		if hasVolume(deploy, volumeName) {
			continue
		}

		// Add hostAliases so Flux controllers can reach KCP at localhost via the platform-mesh IP
		if platformMeshIP != "" {
			deploy.Spec.Template.Spec.HostAliases = append(deploy.Spec.Template.Spec.HostAliases, corev1.HostAlias{
				IP:        platformMeshIP,
				Hostnames: []string{"localhost"},
			})
		}

		deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: kubeconfigSecret,
				},
			},
		})

		for i := range deploy.Spec.Template.Spec.Containers {
			deploy.Spec.Template.Spec.Containers[i].VolumeMounts = append(
				deploy.Spec.Template.Spec.Containers[i].VolumeMounts,
				corev1.VolumeMount{
					Name:      volumeName,
					MountPath: mountPath,
					ReadOnly:  true,
				},
			)
			deploy.Spec.Template.Spec.Containers[i].Env = append(
				deploy.Spec.Template.Spec.Containers[i].Env,
				corev1.EnvVar{
					Name:  "KUBECONFIG",
					Value: kubeconfigPath,
				},
			)
		}

		if err := mcpClient.Update(ctx, deploy); err != nil {
			return fmt.Errorf("failed to patch deployment %s with kcp-kubeconfig volume: %w", name, err)
		}
	}

	return nil
}

func hasVolume(deploy *appsv1.Deployment, volumeName string) bool {
	for _, v := range deploy.Spec.Template.Spec.Volumes {
		if v.Name == volumeName {
			return true
		}
	}
	return false
}
