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

	ocmv1alpha1 "github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/api/ocm/v1alpha1"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/subroutines"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/tool"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/toolcrds"
)

var (
	ocmControllerReconcilerName = "OCMControllerReconciler"

	ocmToolConfig = tool.ToolConfig{
		Name:            "ocm",
		Namespace:       "ocm-system",
		FinalizerPrefix: "ocm.openmcp.io",
		HelmReleaseName: "ocm-controller",
	}

	ocmContentConfigs = []tool.ContentConfigEntry{
		{Group: "delivery.ocm.software", Version: "v1alpha1", Kind: "Component", Plural: "components", DisplayLabel: "Components", Icon: "product", Order: 100, PathSegment: "components", CategoryID: "ocm-delivery", CategoryLabel: "OCM Delivery", CategoryOrder: 840, Scope: "Namespaced"},
		{Group: "delivery.ocm.software", Version: "v1alpha1", Kind: "Resource", Plural: "resources", DisplayLabel: "Resources", Icon: "document", Order: 110, PathSegment: "resources", CategoryID: "ocm-delivery", CategoryLabel: "OCM Delivery", CategoryOrder: 840, Scope: "Namespaced"},
		{Group: "delivery.ocm.software", Version: "v1alpha1", Kind: "Repository", Plural: "repositories", DisplayLabel: "Repositories", Icon: "folder-full", Order: 120, PathSegment: "repositories", CategoryID: "ocm-delivery", CategoryLabel: "OCM Delivery", CategoryOrder: 840, Scope: "Namespaced"},
		{Group: "delivery.ocm.software", Version: "v1alpha1", Kind: "Deployer", Plural: "deployers", DisplayLabel: "Deployers", Icon: "deploy", Order: 130, PathSegment: "deployers", CategoryID: "ocm-delivery", CategoryLabel: "OCM Delivery", CategoryOrder: 840, Scope: "Cluster"},
	}
)

type OCMControllerReconciler struct {
	lifecycle *mclifecycle.LifecycleManager
	log       *logger.Logger
}

func NewOCMControllerReconciler(cfg config.OperatorConfig, mgr mcmanager.Manager, onboardingClient client.Client, log *logger.Logger) *OCMControllerReconciler {
	var subs []subroutine.Subroutine

	provider := &mcManagerKCPAdapter{mgr: mgr}

	toolCfg := ocmToolConfig
	toolCfg.SkipCRDs = true
	toolCfg.HelmChartURL = cfg.OCM.ChartURL
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
		if cfg.OCM.ImageRepository != "" {
			image := map[string]any{"repository": cfg.OCM.ImageRepository}
			if cfg.OCM.ImageTag != "" {
				image["tag"] = cfg.OCM.ImageTag
			}
			values["image"] = image
		}
		return values
	}
	toolCfg.PostInstallFunc = ocmPostInstall

	if cfg.Subroutines.DeployOCMCRDs.Enabled {
		subs = append(subs, subroutines.NewDeployCRDsSubroutine(provider, "ocm", toolcrds.OCMCRDs, "ocm.openmcp.io/managed-crds"))
	}
	if cfg.Subroutines.InstallOCM.Enabled {
		subs = append(subs, subroutines.NewInstallToolSubroutine(provider, onboardingClient, &cfg, &toolCfg))
	}
	if cfg.Subroutines.DeployContentConfigurations.Enabled {
		subs = append(subs, subroutines.NewDeployToolContentConfigurationsSubroutine(provider, "ocm", "services.openmcp.cloud", ocmContentConfigs, "ocm.openmcp.io/managed-content-configurations"))
	}

	return &OCMControllerReconciler{
		lifecycle: builder.NewBuilder(operatorName, ocmControllerReconcilerName, subs, log).
			BuildMultiCluster(mgr),
		log: log,
	}
}

func (r *OCMControllerReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	return r.lifecycle.Reconcile(mccontext.WithCluster(ctx, req.ClusterName), req, &ocmv1alpha1.OCMController{})
}

func (r *OCMControllerReconciler) SetupWithManager(mgr mcmanager.Manager, cfg *platformmeshconfig.CommonServiceConfig, log *logger.Logger, eventPredicates ...predicate.Predicate) error {
	return r.lifecycle.SetupWithManager(mgr, cfg.MaxConcurrentReconciles, ocmControllerReconcilerName, &ocmv1alpha1.OCMController{}, cfg.DebugLabelValue, r, log, eventPredicates...)
}

// ocmPostInstall patches the OCM controller Deployment to mount the kcp-kubeconfig secret
// and set the KUBECONFIG env var so the OCM controller targets the KCP workspace.
func ocmPostInstall(ctx context.Context, mcpClient client.Client, kubeconfigSecret string, platformMeshIP string) error {
	deploy := &appsv1.Deployment{}
	if err := mcpClient.Get(ctx, types.NamespacedName{Name: "ocm-controller-ocm-k8s-toolkit-controller-manager", Namespace: "default"}, deploy); err != nil {
		return fmt.Errorf("failed to get ocm-controller deployment: %w", err)
	}

	volumeName := "kcp-kubeconfig"
	if hasVolume(deploy, volumeName) {
		return nil
	}

	mountPath := "/etc/ocm/kcp"
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
		return fmt.Errorf("failed to patch ocm-controller deployment with kcp-kubeconfig volume: %w", err)
	}

	return nil
}
