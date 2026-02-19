package subroutines

import (
	"context"

	apisv1alpha1 "github.com/kcp-dev/sdk/apis/apis/v1alpha1"
	"github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
)

type DeployAPIExportBindingSubroutine struct {
	cfg    *config.OperatorConfig
	scheme *runtime.Scheme
}

func NewDeployAPIExportBindingSubroutine(cfg *config.OperatorConfig, scheme *runtime.Scheme) *DeployAPIExportBindingSubroutine {
	return &DeployAPIExportBindingSubroutine{
		cfg:    cfg,
		scheme: scheme,
	}
}

func (r *DeployAPIExportBindingSubroutine) GetName() string {
	return "DeployAPIExportBinding"
}

func (r *DeployAPIExportBindingSubroutine) Finalize(_ context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	return ctrl.Result{}, nil
}

func (r *DeployAPIExportBindingSubroutine) Finalizers(_ runtimeobject.RuntimeObject) []string {
	return []string{}
}

func (r *DeployAPIExportBindingSubroutine) Process(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	_ = runtimeObj.(*v1alpha2.APIBinding)

	kcpConfig, err := clientcmd.BuildConfigFromFlags("", r.cfg.KCP.Kubeconfig)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	kcpClient, err := client.New(kcpConfig, client.Options{
		Scheme: r.scheme,
	})
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	// Create APIExport if it does not exist
	apiExport := &apisv1alpha1.APIExport{
		ObjectMeta: metav1.ObjectMeta{
			Name: apiExportName,
		},
		Spec: apisv1alpha1.APIExportSpec{},
	}

	mutateApiExport := func() error {
		return nil
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, kcpClient, apiExport, mutateApiExport); err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	// Create APIBinding if it does not exist
	apiBinding := &apisv1alpha1.APIBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: apiExportName,
		},
		Spec: apisv1alpha1.APIBindingSpec{
			Reference: apisv1alpha1.BindingReference{
				Export: &apisv1alpha1.ExportBindingReference{
					Name: apiExport.Name,
				},
			},
			PermissionClaims: []apisv1alpha1.AcceptablePermissionClaim{
				{
					State: apisv1alpha1.ClaimAccepted,
					PermissionClaim: apisv1alpha1.PermissionClaim{
						All: true,
						GroupResource: apisv1alpha1.GroupResource{
							Group:    "",
							Resource: "secrets",
						},
					},
				},
				{
					State: apisv1alpha1.ClaimAccepted,
					PermissionClaim: apisv1alpha1.PermissionClaim{
						All: true,
						GroupResource: apisv1alpha1.GroupResource{
							Group:    "",
							Resource: "configmaps",
						},
					},
				},
				{
					State: apisv1alpha1.ClaimAccepted,
					PermissionClaim: apisv1alpha1.PermissionClaim{
						All: true,
						GroupResource: apisv1alpha1.GroupResource{
							Group:    "",
							Resource: "namespaces",
						},
					},
				},
			},
		},
	}

	mutateApiBinding := func() error {
		return nil
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, kcpClient, apiBinding, mutateApiBinding); err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	return ctrl.Result{}, nil
}
