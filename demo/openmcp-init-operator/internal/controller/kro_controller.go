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
	}

	kroContentConfigs []tool.ContentConfigEntry
)

type KROReconciler struct {
	lifecycle *mclifecycle.LifecycleManager
	log       *logger.Logger
}

func NewKROReconciler(cfg config.OperatorConfig, mgr mcmanager.Manager, onboardingClient client.Client, log *logger.Logger) *KROReconciler {
	var subs []subroutine.Subroutine

	provider := &mcManagerKCPAdapter{mgr: mgr}

	toolCfg := kroToolConfig
	toolCfg.HelmChartURL = cfg.KRO.ChartURL
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
		if cfg.KRO.ImageRepository != "" {
			image := map[string]any{"repository": cfg.KRO.ImageRepository}
			if cfg.KRO.ImageTag != "" {
				image["tag"] = cfg.KRO.ImageTag
			}
			values["image"] = image
		}
		return values
	}

	if cfg.Subroutines.DeployKROCRDs.Enabled {
		subs = append(subs, subroutines.NewDeployCRDsSubroutine(provider, "kro", toolcrds.KROCRDs, "kro.openmcp.io/managed-crds"))
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
