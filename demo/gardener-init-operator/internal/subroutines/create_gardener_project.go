package subroutines

import (
	"context"
	"fmt"
	"time"

	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	mccontext "sigs.k8s.io/multicluster-runtime/pkg/context"

	gardenerv1alpha1 "github.com/openmcp/local-event-showcase/demo/gardener-init-operator/api/v1alpha1"
	"github.com/openmcp/local-event-showcase/demo/gardener-init-operator/internal/config"
)

const (
	CreateGardenerProjectSubroutineName = "CreateGardenerProject"
	CreateGardenerProjectFinalizerName  = "gardener.cloud/gardener-project"
	gardenerServiceAccountName          = "openmcp"
	// Gardener Project names are limited to 10 characters.
	maxProjectNameLength = 10
)

// shortenClusterID truncates a KCP cluster ID to fit within Gardener's
// 10-character Project name limit.
func shortenClusterID(clusterID string) string {
	if len(clusterID) <= maxProjectNameLength {
		return clusterID
	}
	return clusterID[:maxProjectNameLength]
}

var gardenerProjectGVR = schema.GroupVersionResource{
	Group:    "core.gardener.cloud",
	Version:  "v1beta1",
	Resource: "projects",
}

type CreateGardenerProjectSubroutine struct {
	gardenerClient client.Client
	cfg            *config.OperatorConfig
}

func NewCreateGardenerProjectSubroutine(gardenerClient client.Client, cfg *config.OperatorConfig) *CreateGardenerProjectSubroutine {
	return &CreateGardenerProjectSubroutine{
		gardenerClient: gardenerClient,
		cfg:            cfg,
	}
}

func (r *CreateGardenerProjectSubroutine) GetName() string {
	return CreateGardenerProjectSubroutineName
}

var _ subroutine.Subroutine = &CreateGardenerProjectSubroutine{}

func (r *CreateGardenerProjectSubroutine) Finalizers(_ runtimeobject.RuntimeObject) []string {
	return []string{CreateGardenerProjectFinalizerName}
}

func (r *CreateGardenerProjectSubroutine) Finalize(ctx context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)

	clusterID, ok := mccontext.ClusterFrom(ctx)
	if !ok {
		return ctrl.Result{}, errors.NewOperatorError(errors.New("could not get cluster ID from context"), false, true)
	}

	projectName := shortenClusterID(clusterID)

	// Check if Gardener Project still exists
	project := &unstructured.Unstructured{}
	project.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gardenerProjectGVR.Group,
		Version: gardenerProjectGVR.Version,
		Kind:    "Project",
	})
	err := r.gardenerClient.Get(ctx, types.NamespacedName{Name: projectName}, project)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info().Str("name", projectName).Msg("Gardener Project already deleted, finalizer can be removed")
			return ctrl.Result{}, nil
		}
		log.Error().Err(err).Str("name", projectName).Msg("failed to get Gardener Project")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	// Project exists, trigger deletion if not already deleting
	deletionTimestamp := project.GetDeletionTimestamp()
	if deletionTimestamp == nil || deletionTimestamp.IsZero() {
		log.Info().Str("clusterID", clusterID).Msg("deleting Gardener Project")
		if err := r.gardenerClient.Delete(ctx, project); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error().Err(err).Str("name", projectName).Msg("failed to delete Gardener Project")
				return ctrl.Result{}, errors.NewOperatorError(err, true, true)
			}
		}
	}

	log.Info().Str("name", projectName).Msg("waiting for Gardener Project to be deleted")
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *CreateGardenerProjectSubroutine) Process(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)
	gardenerProject := runtimeObj.(*gardenerv1alpha1.GardenerProject)

	gardenerProject.Status.Phase = gardenerv1alpha1.GardenerProjectPhaseProvisioning

	clusterID, ok := mccontext.ClusterFrom(ctx)
	if !ok {
		return ctrl.Result{}, errors.NewOperatorError(errors.New("could not get cluster ID from context"), false, true)
	}

	projectName := shortenClusterID(clusterID)
	log.Info().Str("clusterID", clusterID).Str("projectName", projectName).Msg("creating Gardener Project")

	// Step 1: Create the Gardener Project (unstructured to avoid massive Gardener Go dependency)
	project := &unstructured.Unstructured{}
	project.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gardenerProjectGVR.Group,
		Version: gardenerProjectGVR.Version,
		Kind:    "Project",
	})

	err := r.gardenerClient.Get(ctx, types.NamespacedName{Name: projectName}, project)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error().Err(err).Str("name", projectName).Msg("failed to get Gardener Project")
			return ctrl.Result{}, errors.NewOperatorError(err, true, true)
		}

		// Create the project
		project.SetName(projectName)
		project.Object["spec"] = map[string]interface{}{
			"namespace": fmt.Sprintf("garden-%s", projectName),
		}
		if err := r.gardenerClient.Create(ctx, project); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				log.Error().Err(err).Str("name", projectName).Msg("failed to create Gardener Project")
				return ctrl.Result{}, errors.NewOperatorError(err, true, true)
			}
		}
		log.Info().Str("name", projectName).Msg("Gardener Project created")
	}

	// Step 2: Wait for project to be ready
	phase, _, _ := unstructured.NestedString(project.Object, "status", "phase")
	if phase != "Ready" {
		log.Info().Str("name", projectName).Str("phase", phase).Msg("Gardener Project not ready yet, requeuing")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Step 3: Ensure the project namespace exists (Gardener creates it)
	projectNamespace := fmt.Sprintf("garden-%s", projectName)
	ns := &corev1.Namespace{}
	if err := r.gardenerClient.Get(ctx, types.NamespacedName{Name: projectNamespace}, ns); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info().Str("namespace", projectNamespace).Msg("project namespace not ready yet, requeuing")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		log.Error().Err(err).Str("namespace", projectNamespace).Msg("failed to get project namespace")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	// Step 4: Create ServiceAccount in the project namespace
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gardenerServiceAccountName,
			Namespace: projectNamespace,
		},
	}
	if err := r.gardenerClient.Get(ctx, types.NamespacedName{Name: sa.Name, Namespace: sa.Namespace}, sa); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, errors.NewOperatorError(err, true, true)
		}
		if err := r.gardenerClient.Create(ctx, sa); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				log.Error().Err(err).Str("name", sa.Name).Str("namespace", sa.Namespace).Msg("failed to create ServiceAccount")
				return ctrl.Result{}, errors.NewOperatorError(err, true, true)
			}
		}
		log.Info().Str("name", sa.Name).Str("namespace", sa.Namespace).Msg("ServiceAccount created")
	}

	// Step 5: Create RoleBinding granting admin in the project namespace
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-admin", gardenerServiceAccountName),
			Namespace: projectNamespace,
		},
	}
	if err := r.gardenerClient.Get(ctx, types.NamespacedName{Name: rb.Name, Namespace: rb.Namespace}, rb); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, errors.NewOperatorError(err, true, true)
		}
		rb.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "admin",
		}
		rb.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      gardenerServiceAccountName,
				Namespace: projectNamespace,
			},
		}
		if err := r.gardenerClient.Create(ctx, rb); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				log.Error().Err(err).Str("name", rb.Name).Str("namespace", rb.Namespace).Msg("failed to create RoleBinding")
				return ctrl.Result{}, errors.NewOperatorError(err, true, true)
			}
		}
		log.Info().Str("name", rb.Name).Str("namespace", rb.Namespace).Msg("RoleBinding created")
	}

	gardenerProject.Status.Phase = gardenerv1alpha1.GardenerProjectPhaseProjectReady
	log.Info().Str("name", projectName).Msg("Gardener Project provisioning complete")
	return ctrl.Result{}, nil
}
