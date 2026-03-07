package controller

import (
	"context"

	"github.com/kcp-dev/sdk/apis/apis/v1alpha2"
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

	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/subroutines"
)

var (
	operatorName          = "openmcp-init-operator"
	accountReconcilerName = "OpenMCPInitReconciler"
)

type OpenMCPInitReconciler struct {
	lifecycle *mclifecycle.LifecycleManager
	log       *logger.Logger
}

func NewOpenMCPInitReconciler(cfg config.OperatorConfig, mgr mcmanager.Manager, onboardingClient client.Client, log *logger.Logger) *OpenMCPInitReconciler {
	var subs []subroutine.Subroutine

	log.Info().
		Bool("createMCP", cfg.Subroutines.CreateMCP.Enabled).
		Bool("setupSyncAgent", cfg.Subroutines.SetupSyncAgent.Enabled).
		Bool("setupFlux", cfg.Subroutines.SetupFlux.Enabled).
		Bool("initPublishedResources", cfg.Subroutines.InitializePublishedResources.Enabled).
		Bool("deployAPIExportBinding", cfg.Subroutines.DeployAPIExportBinding.Enabled).
		Bool("createCrossplane", cfg.Subroutines.CreateCrossplane.Enabled).
		Msg("OpenMCPInitReconciler: subroutine configuration")

	if cfg.Subroutines.CreateMCP.Enabled {
		subs = append(subs, subroutines.NewCreateMCPSubroutine(mgr.GetLocalManager().GetClient(), onboardingClient, &cfg))
	}
	if cfg.Subroutines.SetupSyncAgent.Enabled {
		subs = append(subs, subroutines.NewSetupSyncAgentSubroutine(mgr, onboardingClient, &cfg))
	}
	if cfg.Subroutines.SetupFlux.Enabled {
		subs = append(subs, subroutines.NewSetupFluxSubroutine(onboardingClient, &cfg))
	}
	if cfg.Subroutines.DeployAPIExportBinding.Enabled {
		subs = append(subs, subroutines.NewDeployAPIExportBindingSubroutine(&cfg, mgr.GetLocalManager().GetScheme()))
	}

	return &OpenMCPInitReconciler{
		lifecycle: builder.NewBuilder(operatorName, accountReconcilerName, subs, log).
			WithReadOnly().
			BuildMultiCluster(mgr),
		log: log,
	}
}

func (r *OpenMCPInitReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	return r.lifecycle.Reconcile(mccontext.WithCluster(ctx, req.ClusterName), req, &v1alpha2.APIBinding{})
}
func (r *OpenMCPInitReconciler) SetupWithManager(mgr mcmanager.Manager, cfg *platformmeshconfig.CommonServiceConfig, log *logger.Logger, eventPredicates ...predicate.Predicate) error { // coverage-ignore
	return r.lifecycle.SetupWithManager(mgr, cfg.MaxConcurrentReconciles, accountReconcilerName, &v1alpha2.APIBinding{}, cfg.DebugLabelValue, r, log, eventPredicates...)
}
