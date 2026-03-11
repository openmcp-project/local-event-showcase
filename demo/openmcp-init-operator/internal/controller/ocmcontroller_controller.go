package controller

import (
	"context"

	platformmeshconfig "github.com/platform-mesh/golang-commons/config"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/builder"
	mclifecycle "github.com/platform-mesh/golang-commons/controller/lifecycle/multicluster"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/logger"
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

	ocmContentConfigs []tool.ContentConfigEntry
)

type OCMControllerReconciler struct {
	lifecycle *mclifecycle.LifecycleManager
	log       *logger.Logger
}

func NewOCMControllerReconciler(cfg config.OperatorConfig, mgr mcmanager.Manager, onboardingClient client.Client, log *logger.Logger) *OCMControllerReconciler {
	var subs []subroutine.Subroutine

	provider := &mcManagerKCPAdapter{mgr: mgr}

	toolCfg := ocmToolConfig
	toolCfg.HelmChartURL = cfg.OCM.ChartURL
	toolCfg.HelmValuesFunc = func(version string, kubeconfigSecret string, platformMeshIP string) map[string]any {
		values := map[string]any{
			"kcpKubeconfig": kubeconfigSecret,
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
		if cfg.OCM.ImageRepository != "" {
			image := map[string]any{"repository": cfg.OCM.ImageRepository}
			if cfg.OCM.ImageTag != "" {
				image["tag"] = cfg.OCM.ImageTag
			}
			values["image"] = image
		}
		return values
	}

	if cfg.Subroutines.DeployOCMCRDs.Enabled {
		subs = append(subs, subroutines.NewDeployCRDsSubroutine(provider, "ocm", toolcrds.OCMCRDs, "ocm.openmcp.io/managed-crds"))
	}
	if cfg.Subroutines.InstallOCM.Enabled {
		subs = append(subs, subroutines.NewInstallToolSubroutine(provider, onboardingClient, &cfg, &toolCfg))
	}
	if cfg.Subroutines.DeployContentConfigurations.Enabled {
		subs = append(subs, subroutines.NewDeployToolContentConfigurationsSubroutine(provider, "ocm", "ocm.services.openmcp.cloud", ocmContentConfigs, "ocm.openmcp.io/managed-content-configurations"))
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
