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

	crossplanev1alpha1 "github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/api/v1alpha1"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/subroutines"
)

var crossplaneReconcilerName = "CrossplaneReconciler"

type CrossplaneReconciler struct {
	lifecycle *mclifecycle.LifecycleManager
	log       *logger.Logger
}

func NewCrossplaneReconciler(cfg config.OperatorConfig, mgr mcmanager.Manager, onboardingClient client.Client, log *logger.Logger) *CrossplaneReconciler {
	var subs []subroutine.Subroutine

	if cfg.Subroutines.CreateCrossplane.Enabled {
		subs = append(subs, subroutines.NewCreateCrossplaneSubroutine(onboardingClient, &cfg))
	}
	if cfg.Subroutines.InitializePublishedResources.Enabled {
		subs = append(subs, subroutines.NewInitializePublishedResourcesSubroutine(onboardingClient, &cfg))
	}

	return &CrossplaneReconciler{
		lifecycle: builder.NewBuilder(operatorName, crossplaneReconcilerName, subs, log).
			WithReadOnly().
			BuildMultiCluster(mgr),
		log: log,
	}
}

func (r *CrossplaneReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	return r.lifecycle.Reconcile(mccontext.WithCluster(ctx, req.ClusterName), req, &crossplanev1alpha1.Crossplane{})
}

func (r *CrossplaneReconciler) SetupWithManager(mgr mcmanager.Manager, cfg *platformmeshconfig.CommonServiceConfig, log *logger.Logger, eventPredicates ...predicate.Predicate) error {
	return r.lifecycle.SetupWithManager(mgr, cfg.MaxConcurrentReconciles, crossplaneReconcilerName, &crossplanev1alpha1.Crossplane{}, cfg.DebugLabelValue, r, log, eventPredicates...)
}
