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

	gardenerv1alpha1 "github.com/openmcp/local-event-showcase/demo/gardener-init-operator/api/v1alpha1"
	"github.com/openmcp/local-event-showcase/demo/gardener-init-operator/internal/config"
	"github.com/openmcp/local-event-showcase/demo/gardener-init-operator/internal/subroutines"
)

var (
	operatorName                  = "gardener-init-operator"
	gardenerProjectReconcilerName = "GardenerProjectReconciler"
)

type GardenerProjectReconciler struct {
	lifecycle *mclifecycle.LifecycleManager
	log       *logger.Logger
}

func NewGardenerProjectReconciler(cfg config.OperatorConfig, mgr mcmanager.Manager, gardenerClient client.Client, log *logger.Logger) *GardenerProjectReconciler {
	var subs []subroutine.Subroutine

	log.Info().
		Bool("createGardenerProject", cfg.Subroutines.CreateGardenerProject.Enabled).
		Bool("setupGardenerAccess", cfg.Subroutines.SetupGardenerAccess.Enabled).
		Msg("GardenerProjectReconciler: subroutine configuration")

	if cfg.Subroutines.CreateGardenerProject.Enabled {
		subs = append(subs, subroutines.NewCreateGardenerProjectSubroutine(gardenerClient, &cfg))
	}
	if cfg.Subroutines.SetupGardenerAccess.Enabled {
		subs = append(subs, subroutines.NewSetupGardenerAccessSubroutine(mgr, gardenerClient, &cfg))
	}

	return &GardenerProjectReconciler{
		lifecycle: builder.NewBuilder(operatorName, gardenerProjectReconcilerName, subs, log).
			BuildMultiCluster(mgr),
		log: log,
	}
}

func (r *GardenerProjectReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	return r.lifecycle.Reconcile(mccontext.WithCluster(ctx, req.ClusterName), req, &gardenerv1alpha1.GardenerProject{})
}

func (r *GardenerProjectReconciler) SetupWithManager(mgr mcmanager.Manager, cfg *platformmeshconfig.CommonServiceConfig, log *logger.Logger, eventPredicates ...predicate.Predicate) error {
	return r.lifecycle.SetupWithManager(mgr, cfg.MaxConcurrentReconciles, gardenerProjectReconcilerName, &gardenerv1alpha1.GardenerProject{}, cfg.DebugLabelValue, r, log, eventPredicates...)
}
