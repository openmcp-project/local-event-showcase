package subroutines

import (
	"context"
	"time"

	clustersv1alpha1 "github.com/openmcp-project/openmcp-operator/api/clusters/v1alpha1"
	commonapi "github.com/openmcp-project/openmcp-operator/api/common"
	mcpv2alpha1 "github.com/openmcp-project/openmcp-operator/api/core/v2alpha1"
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

	corev1alpha1 "github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/api/core/v1alpha1"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
)

const (
	CreateMCPSubroutineName = "CreateMCP"
	CreateMCPFinalizerName  = "mcp.openmcp.io/managed-control-plane"
)

type CreateMCPSubroutine struct {
	client           client.Client
	onboardingClient client.Client
	cfg              *config.OperatorConfig
}

func NewCreateMCPSubroutine(client client.Client, onboardingClient client.Client, cfg *config.OperatorConfig) *CreateMCPSubroutine {
	return &CreateMCPSubroutine{
		client:           client,
		onboardingClient: onboardingClient,
		cfg:              cfg,
	}
}

func (r *CreateMCPSubroutine) GetName() string {
	return CreateMCPSubroutineName
}

func (r *CreateMCPSubroutine) Finalize(ctx context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)

	// Get cluster ID from multicluster context
	clusterID, ok := mccontext.ClusterFrom(ctx)
	if !ok {
		return ctrl.Result{}, errors.NewOperatorError(errors.New("could not get cluster ID from context"), false, true)
	}

	// Check if MCP still exists
	mcp := &mcpv2alpha1.ManagedControlPlaneV2{}
	err := r.onboardingClient.Get(ctx, types.NamespacedName{Name: clusterID, Namespace: r.cfg.MCP.Namespace}, mcp)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info().Str("name", clusterID).Msg("ManagedControlPlaneV2 already deleted, finalizer can be removed")
			return ctrl.Result{}, nil
		}
		log.Error().Err(err).Str("name", clusterID).Msg("failed to get ManagedControlPlaneV2")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	// MCP exists, trigger deletion if not already deleting
	if mcp.DeletionTimestamp.IsZero() {
		log.Info().Str("clusterID", clusterID).Msg("deleting ManagedControlPlaneV2")
		if err := r.onboardingClient.Delete(ctx, mcp); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error().Err(err).Str("name", clusterID).Msg("failed to delete ManagedControlPlaneV2")
				return ctrl.Result{}, errors.NewOperatorError(err, true, true)
			}
		}
	}

	// MCP still exists, requeue to wait for deletion
	log.Info().Str("name", clusterID).Msg("waiting for ManagedControlPlaneV2 to be deleted")
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *CreateMCPSubroutine) Finalizers(_ runtimeobject.RuntimeObject) []string {
	return []string{CreateMCPFinalizerName}
}

var _ subroutine.Subroutine = &CreateMCPSubroutine{}

// Process creates or updates a ManagedControlPlaneV2 resource using the MCP client.
// The ManagedControlPlaneV2 name is derived from the KCP workspace cluster ID.
func (r *CreateMCPSubroutine) Process(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)
	managedCP := runtimeObj.(*corev1alpha1.ManagedControlPlane)

	managedCP.Status.Phase = corev1alpha1.ManagedControlPlanePhaseProvisioning

	// Get cluster ID from multicluster context
	clusterID, ok := mccontext.ClusterFrom(ctx)
	if !ok {
		return ctrl.Result{}, errors.NewOperatorError(errors.New("could not get cluster ID from context"), false, true)
	}

	log.Info().Str("clusterID", clusterID).Msg("creating ManagedControlPlaneV2")

	mcp := &mcpv2alpha1.ManagedControlPlaneV2{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterID,
			Namespace: r.cfg.MCP.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.onboardingClient, mcp, func() error {
		mcp.Spec = mcpv2alpha1.ManagedControlPlaneV2Spec{
			IAM: mcpv2alpha1.IAMConfig{
				Tokens: []mcpv2alpha1.TokenConfig{
					{
						Name: "operator-token",
						TokenConfig: clustersv1alpha1.TokenConfig{
							RoleRefs: []commonapi.RoleRef{
								{Kind: "ClusterRole", Name: "cluster-admin"},
							},
						},
					},
				},
			},
		}
		return nil
	})
	if err != nil {
		log.Error().Err(err).Str("name", clusterID).Msg("failed to create or update ManagedControlPlaneV2")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	log.Info().Str("name", clusterID).Msg("ManagedControlPlaneV2 created or updated successfully")

	managedCP.Status.Phase = corev1alpha1.ManagedControlPlanePhaseMCPReady
	return ctrl.Result{}, nil
}
