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

	corev1alpha1 "github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/api/core/v1alpha1"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/subroutines"
)

var (
	operatorName                      = "openmcp-init-operator"
	managedControlPlaneReconcilerName = "ManagedControlPlaneReconciler"
)

type ManagedControlPlaneReconciler struct {
	lifecycle *mclifecycle.LifecycleManager
	log       *logger.Logger
}

func NewManagedControlPlaneReconciler(cfg config.OperatorConfig, mgr mcmanager.Manager, onboardingClient client.Client, log *logger.Logger) *ManagedControlPlaneReconciler {
	var subs []subroutine.Subroutine

	log.Info().
		Bool("createMCP", cfg.Subroutines.CreateMCP.Enabled).
		Bool("setupSyncAgent", cfg.Subroutines.SetupSyncAgent.Enabled).
		Msg("ManagedControlPlaneReconciler: subroutine configuration")

	if cfg.Subroutines.CreateMCP.Enabled {
		subs = append(subs, subroutines.NewCreateMCPSubroutine(mgr.GetLocalManager().GetClient(), onboardingClient, &cfg))
	}
	if cfg.Subroutines.SetupSyncAgent.Enabled {
		subs = append(subs, subroutines.NewSetupSyncAgentSubroutine(mgr, onboardingClient, &cfg))
	}

	return &ManagedControlPlaneReconciler{
		lifecycle: builder.NewBuilder(operatorName, managedControlPlaneReconcilerName, subs, log).
			BuildMultiCluster(mgr),
		log: log,
	}
}

func (r *ManagedControlPlaneReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	return r.lifecycle.Reconcile(mccontext.WithCluster(ctx, req.ClusterName), req, &corev1alpha1.ManagedControlPlane{})
}

func (r *ManagedControlPlaneReconciler) SetupWithManager(mgr mcmanager.Manager, cfg *platformmeshconfig.CommonServiceConfig, log *logger.Logger, eventPredicates ...predicate.Predicate) error {
	return r.lifecycle.SetupWithManager(mgr, cfg.MaxConcurrentReconciles, managedControlPlaneReconcilerName, &corev1alpha1.ManagedControlPlane{}, cfg.DebugLabelValue, r, log, eventPredicates...)
}
