package subroutines

import (
	"context"
	"time"

	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// onboardingCrossplaneGVK is the GVK for the Crossplane resource on the onboarding cluster.
// The onboarding cluster runs the openmcp-operator which uses the openmcp.cloud API group.
var onboardingCrossplaneGVK = schema.GroupVersionKind{
	Group:   "crossplane.services.openmcp.cloud",
	Version: "v1alpha1",
	Kind:    "Crossplane",
}

const (
	CreateCrossplaneSubroutineName = "CreateCrossplane"
	CreateCrossplaneFinalizerName  = "crossplane.opencp.io/managed-crossplane"
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

	// Check for remaining APIBindings to the Crossplane APIExport before deleting.
	kcpClient, err := r.kcpProvider.KCPClientFromContext(ctx)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	bound, checkErr := apiExportHasBindings(ctx, kcpClient, "crossplane.services.opencp.cloud")
	if checkErr != nil {
		log.Error().Err(checkErr).Msg("failed to check APIBindings for Crossplane APIExport")
		return ctrl.Result{}, errors.NewOperatorError(checkErr, true, true)
	}
	if bound {
		log.Info().Msg("APIBindings still reference crossplane.services.opencp.cloud, waiting before Crossplane deletion")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	crossplane := newOnboardingCrossplane(clusterID, r.cfg.MCP.Namespace)
	err = r.onboardingClient.Get(ctx, types.NamespacedName{Name: clusterID, Namespace: r.cfg.MCP.Namespace}, crossplane)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info().Str("name", clusterID).Msg("Crossplane already deleted, finalizer can be removed")
			return ctrl.Result{}, nil
		}
		log.Error().Err(err).Str("name", clusterID).Msg("failed to get Crossplane")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	deletionTimestamp := crossplane.GetDeletionTimestamp()
	if deletionTimestamp == nil || deletionTimestamp.IsZero() {
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

	targetCrossplane := newOnboardingCrossplane(clusterID, r.cfg.MCP.Namespace)

	_, err := controllerutil.CreateOrUpdate(ctx, r.onboardingClient, targetCrossplane, func() error {
		spec := map[string]interface{}{
			"version": sourceCrossplane.Spec.Version,
		}
		filteredProviders := filterManualProviders(sourceCrossplane.Spec.Providers)
		if len(filteredProviders) > 0 {
			providers := make([]interface{}, 0, len(filteredProviders))
			for _, p := range filteredProviders {
				if p != nil {
					providers = append(providers, map[string]interface{}{
						"name":    p.Name,
						"version": p.Version,
					})
				}
			}
			spec["providers"] = providers
		}
		return unstructured.SetNestedField(targetCrossplane.Object, spec, "spec")
	})
	if err != nil {
		log.Error().Err(err).Str("name", clusterID).Msg("failed to create or update Crossplane")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	log.Info().Str("name", clusterID).Msg("Crossplane created or updated successfully")

	sourceCrossplane.Status.Phase = crossplanev1alpha1.CrossplanePhaseProvisioning
	return ctrl.Result{}, nil
}

// newOnboardingCrossplane creates an unstructured Crossplane resource for the onboarding cluster
// using the openmcp API group (crossplane.services.openmcp.cloud).
func newOnboardingCrossplane(name, namespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(onboardingCrossplaneGVK)
	obj.SetName(name)
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	return obj
}
