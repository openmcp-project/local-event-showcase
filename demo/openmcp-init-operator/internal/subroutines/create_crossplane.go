package subroutines

import (
	"context"
	"time"

	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	mccontext "sigs.k8s.io/multicluster-runtime/pkg/context"

	crossplanev1alpha1 "github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/api/v1alpha1"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
)

const (
	CreateCrossplaneSubroutineName = "CreateCrossplane"
	CreateCrossplaneFinalizerName  = "crossplane.openmcp.io/managed-crossplane"
)

type CreateCrossplaneSubroutine struct {
	onboardingClient client.Client
	cfg              *config.OperatorConfig
}

func NewCreateCrossplaneSubroutine(onboardingClient client.Client, cfg *config.OperatorConfig) *CreateCrossplaneSubroutine {
	return &CreateCrossplaneSubroutine{
		onboardingClient: onboardingClient,
		cfg:              cfg,
	}
}

func (r *CreateCrossplaneSubroutine) GetName() string {
	return CreateCrossplaneSubroutineName
}

func (r *CreateCrossplaneSubroutine) Finalize(ctx context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)

	clusterID, ok := mccontext.ClusterFrom(ctx)
	if !ok {
		return ctrl.Result{}, errors.NewOperatorError(errors.New("could not get cluster ID from context"), false, true)
	}

	crossplane := &crossplanev1alpha1.Crossplane{}
	err := r.onboardingClient.Get(ctx, types.NamespacedName{Name: clusterID, Namespace: r.cfg.MCP.Namespace}, crossplane)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info().Str("name", clusterID).Msg("Crossplane already deleted, finalizer can be removed")
			return ctrl.Result{}, nil
		}
		log.Error().Err(err).Str("name", clusterID).Msg("failed to get Crossplane")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	if crossplane.DeletionTimestamp.IsZero() {
		log.Info().Str("clusterID", clusterID).Msg("deleting Crossplane")
		if err := r.onboardingClient.Delete(ctx, crossplane); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error().Err(err).Str("name", clusterID).Msg("failed to delete Crossplane")
				return ctrl.Result{}, errors.NewOperatorError(err, true, true)
			}
		}
	}

	log.Info().Str("name", clusterID).Msg("waiting for Crossplane to be deleted")
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *CreateCrossplaneSubroutine) Finalizers(_ runtimeobject.RuntimeObject) []string {
	return []string{CreateCrossplaneFinalizerName}
}

var _ subroutine.Subroutine = &CreateCrossplaneSubroutine{}

func (r *CreateCrossplaneSubroutine) Process(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)
	sourceCrossplane := runtimeObj.(*crossplanev1alpha1.Crossplane)

	clusterID, ok := mccontext.ClusterFrom(ctx)
	if !ok {
		return ctrl.Result{}, errors.NewOperatorError(errors.New("could not get cluster ID from context"), false, true)
	}

	log.Info().Str("clusterID", clusterID).Msg("creating Crossplane in MCP cluster")

	targetCrossplane := &crossplanev1alpha1.Crossplane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterID,
			Namespace: r.cfg.MCP.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.onboardingClient, targetCrossplane, func() error {
		targetCrossplane.Spec.Version = sourceCrossplane.Spec.Version
		targetCrossplane.Spec.Providers = deepCopyProviders(sourceCrossplane.Spec.Providers)
		return nil
	})
	if err != nil {
		log.Error().Err(err).Str("name", clusterID).Msg("failed to create or update Crossplane")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	log.Info().Str("name", clusterID).Msg("Crossplane created or updated successfully")
	return ctrl.Result{}, nil
}

func deepCopyProviders(providers []*crossplanev1alpha1.CrossplaneProviderConfig) []*crossplanev1alpha1.CrossplaneProviderConfig {
	if providers == nil {
		return nil
	}
	copied := make([]*crossplanev1alpha1.CrossplaneProviderConfig, len(providers))
	for i, p := range providers {
		if p != nil {
			copied[i] = &crossplanev1alpha1.CrossplaneProviderConfig{
				Name:    p.Name,
				Version: p.Version,
			}
		}
	}
	return copied
}
