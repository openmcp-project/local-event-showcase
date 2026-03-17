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

	krov1alpha1 "github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/api/kro/v1alpha1"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/subroutines"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/tool"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/toolcrds"
)

var (
	kroReconcilerName = "KROReconciler"

	kroToolConfig = tool.ToolConfig{
		Name:            "kro",
		Namespace:       "kro-system",
		FinalizerPrefix: "kro.openmcp.io",
		HelmReleaseName: "kro",
		PreDeleteChecks: []tool.PreDeleteResourceCheck{
			{Group: "kro.run", Version: "v1alpha1", Resource: "resourcegraphdefinitions"},
		},
	}

	kroContentConfigs = []tool.ContentConfigEntry{
		{Group: "kro.run", Version: "v1alpha1", Kind: "ResourceGraphDefinition", Plural: "ResourceGraphDefinitions", DisplayLabel: "Resource Graph Definitions", Icon: "org-chart", Order: 100, PathSegment: "resourcegraphdefinitions", CategoryID: "kro-resources", CategoryLabel: "KRO Resources", CategoryOrder: 830, Scope: "Cluster"},
	}
)

type KROReconciler struct {
	lifecycle *mclifecycle.LifecycleManager
	log       *logger.Logger
}

func NewKROReconciler(cfg config.OperatorConfig, mgr mcmanager.Manager, onboardingClient client.Client, log *logger.Logger) *KROReconciler {
	var subs []subroutine.Subroutine

	provider := &mcManagerKCPAdapter{mgr: mgr}

	toolCfg := kroToolConfig
	toolCfg.SkipCRDs = true
	toolCfg.HelmChartURL = cfg.KRO.ChartURL
	toolCfg.HelmValuesFunc = func(version string, kubeconfigSecret string, platformMeshIP string) map[string]any {
		values := map[string]any{}
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
		if cfg.KRO.ImageRepository != "" {
			image := map[string]any{"repository": cfg.KRO.ImageRepository}
			if cfg.KRO.ImageTag != "" {
				image["tag"] = cfg.KRO.ImageTag
			}
			values["image"] = image
		}
		return values
	}
	toolCfg.PostInstallFunc = kroPostInstall

	if cfg.Subroutines.DeployKROCRDs.Enabled {
		subs = append(subs, subroutines.NewDeployAPIResourceSchemasSubroutine(provider, "kro", "kro.services.openmcp.cloud", toolcrds.KROCRDs, "kro.openmcp.io/managed-crds"))
	}
	if cfg.Subroutines.InstallKRO.Enabled {
		subs = append(subs, subroutines.NewInstallToolSubroutine(provider, onboardingClient, &cfg, &toolCfg))
	}
	if cfg.Subroutines.DeployContentConfigurations.Enabled {
		subs = append(subs, subroutines.NewDeployToolContentConfigurationsSubroutine(provider, "kro", "kro.services.openmcp.cloud", kroContentConfigs, "kro.openmcp.io/managed-content-configurations"))
	}

	return &KROReconciler{
		lifecycle: builder.NewBuilder(operatorName, kroReconcilerName, subs, log).
			BuildMultiCluster(mgr),
		log: log,
	}
}

func (r *KROReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	return r.lifecycle.Reconcile(mccontext.WithCluster(ctx, req.ClusterName), req, &krov1alpha1.KRO{})
}

func (r *KROReconciler) SetupWithManager(mgr mcmanager.Manager, cfg *platformmeshconfig.CommonServiceConfig, log *logger.Logger, eventPredicates ...predicate.Predicate) error {
	return r.lifecycle.SetupWithManager(mgr, cfg.MaxConcurrentReconciles, kroReconcilerName, &krov1alpha1.KRO{}, cfg.DebugLabelValue, r, log, eventPredicates...)
}

// kroPostInstall patches the KRO controller Deployment to mount the kcp-kubeconfig secret
// and set the KUBECONFIG env var so KRO targets the KCP workspace.
func kroPostInstall(ctx context.Context, mcpClient client.Client, kubeconfigSecret string, platformMeshIP string) error {
	deploy := &appsv1.Deployment{}
	if err := mcpClient.Get(ctx, types.NamespacedName{Name: "kro", Namespace: "default"}, deploy); err != nil {
		return fmt.Errorf("failed to get kro deployment: %w", err)
	}

	volumeName := "kcp-kubeconfig"
	if hasVolume(deploy, volumeName) {
		return nil
	}

	mountPath := "/etc/kro/kcp"
	kubeconfigPath := mountPath + "/kubeconfig"

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
		return fmt.Errorf("failed to patch kro deployment with kcp-kubeconfig volume: %w", err)
	}

	return nil
}
