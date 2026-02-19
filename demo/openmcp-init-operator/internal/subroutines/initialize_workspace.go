package subroutines

import (
	"context"
	"fmt"
	"slices"
	"time"

	apisv1alpha1 "github.com/kcp-dev/sdk/apis/apis/v1alpha1"
	corev1alpha1 "github.com/kcp-dev/sdk/apis/core/v1alpha1"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	mccontext "sigs.k8s.io/multicluster-runtime/pkg/context"
)

const (
	InitializeWorkspaceSubroutineName = "InitializeWorkspace"
)

type InitializeWorkspaceSubroutine struct {
	client          client.Client
	vsConfig        *rest.Config
	initializerName string
}

func NewInitializeWorkspaceSubroutine(client client.Client, vsConfig *rest.Config, initializerName string) *InitializeWorkspaceSubroutine {
	return &InitializeWorkspaceSubroutine{client: client, vsConfig: vsConfig, initializerName: initializerName}
}

func (r *InitializeWorkspaceSubroutine) GetName() string {
	return InitializeWorkspaceSubroutineName
}

func (r *InitializeWorkspaceSubroutine) Finalize(_ context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	return ctrl.Result{}, nil
}

func (r *InitializeWorkspaceSubroutine) Finalizers(_ runtimeobject.RuntimeObject) []string { // coverage-ignore
	return []string{}
}

var _ subroutine.Subroutine = &InitializeWorkspaceSubroutine{}

func (r *InitializeWorkspaceSubroutine) Process(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)
	lc := runtimeObj.(*corev1alpha1.LogicalCluster)

	// Get cluster name from context using multicluster-runtime
	cluster, ok := mccontext.ClusterFrom(ctx)
	if !ok {
		return ctrl.Result{}, errors.NewOperatorError(fmt.Errorf("could not get cluster from context"), false, true)
	}

	restCfg := rest.CopyConfig(r.vsConfig)
	restCfg.Host = fmt.Sprintf("%s/clusters/%s/", restCfg.Host, cluster)
	wsClient, err := client.New(restCfg, client.Options{Scheme: r.client.Scheme()})
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	var export apisv1alpha1.APIExport
	export.SetName("mcp-api")

	_, err = controllerutil.CreateOrUpdate(ctx, wsClient, &export, func() error { return nil })
	if err != nil {
		log.Error().Err(err).Msg("failed to create or update APIExport")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	if lc.Status.Phase == "Initializing" && len(lc.Status.Initializers) == 0 {
		log.Info().Msg("waiting for initializers")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	idx := slices.Index(lc.Status.Initializers, corev1alpha1.LogicalClusterInitializer(r.initializerName))
	if idx == -1 {
		log.Info().Msg("Nothing to do")
		return ctrl.Result{}, nil
	}

	log.Info().Msg("removing initializer")
	updatedLc := lc.DeepCopy()
	updatedLc.Status.Initializers = slices.Delete(updatedLc.Status.Initializers, idx, idx+1)

	err = wsClient.Status().Patch(ctx, updatedLc, client.MergeFrom(lc))
	if err != nil {
		log.Error().Err(err).Msg("failed to patch logical cluster status")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	return ctrl.Result{}, nil
}
