package subroutines

import (
	"context"
	"fmt"
	"time"

	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	kcpProvider      KCPClientProvider
	onboardingClient client.Client
	cfg              *config.OperatorConfig
}

func NewCreateCrossplaneSubroutine(kcpProvider KCPClientProvider, onboardingClient client.Client, cfg *config.OperatorConfig) *CreateCrossplaneSubroutine {
	return &CreateCrossplaneSubroutine{
		kcpProvider:      kcpProvider,
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

	// Check for remaining Crossplane-managed resources in the KCP workspace before deleting.
	kcpClient, err := r.kcpProvider.KCPClientFromContext(ctx)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	remaining, checkErr := checkCrossplaneRemainingResources(ctx, kcpClient)
	if checkErr != nil {
		log.Error().Err(checkErr).Msg("failed to check remaining Crossplane resources")
		return ctrl.Result{}, errors.NewOperatorError(checkErr, true, true)
	}
	if remaining > 0 {
		log.Info().Int("remaining", remaining).Msg("Crossplane-managed resources still exist in workspace, waiting before deletion")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	crossplane := &crossplanev1alpha1.Crossplane{}
	err = r.onboardingClient.Get(ctx, types.NamespacedName{Name: clusterID, Namespace: r.cfg.MCP.Namespace}, crossplane)
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
		targetCrossplane.Spec.Providers = deepCopyProviders(filterManualProviders(sourceCrossplane.Spec.Providers))
		return nil
	})
	if err != nil {
		log.Error().Err(err).Str("name", clusterID).Msg("failed to create or update Crossplane")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	log.Info().Str("name", clusterID).Msg("Crossplane created or updated successfully")

	sourceCrossplane.Status.Phase = crossplanev1alpha1.CrossplanePhaseProvisioning
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

// checkCrossplaneRemainingResources checks the KCP workspace for any remaining
// Crossplane-managed resources (derived from providerResourceMap).
func checkCrossplaneRemainingResources(ctx context.Context, kcpClient client.Client) (int, error) {
	log := logger.LoadLoggerFromContext(ctx)
	total := 0

	for _, entry := range providerResourceMap {
		for _, r := range entry.resources {
			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   r.Group,
				Version: r.Version,
				Kind:    r.Kind,
			})

			if err := kcpClient.List(ctx, list); err != nil {
				if apierrors.IsNotFound(err) || apierrors.IsMethodNotSupported(err) {
					continue
				}
				return 0, fmt.Errorf("listing %s/%s/%s: %w", r.Group, r.Version, r.Kind, err)
			}

			count := len(list.Items)
			if count > 0 {
				log.Warn().Str("group", r.Group).Str("version", r.Version).Str("kind", r.Kind).Int("count", count).Msg("remaining Crossplane resources found")
				total += count
			}
		}
	}

	return total, nil
}
