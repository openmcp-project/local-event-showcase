/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	"github.com/google/uuid"
	"github.com/kcp-dev/logicalcluster/v3"
	corev1alpha1 "github.com/kcp-dev/sdk/apis/core/v1alpha1"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/builder"
	mclifecycle "github.com/platform-mesh/golang-commons/controller/lifecycle/multicluster"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/logger"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mccontext "sigs.k8s.io/multicluster-runtime/pkg/context"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/subroutines"
)

// LogicalClusterReconciler reconciles a LogicalCluster object
type LogicalClusterReconciler struct {
	lifecycle *mclifecycle.LifecycleManager
	log       *logger.Logger
}

func NewLogicalClusterReconciler(cfg config.InitializerConfig, mgr mcmanager.Manager, vsConfig *rest.Config, log *logger.Logger) *LogicalClusterReconciler {
	var subs []subroutine.Subroutine

	if cfg.Subroutines.InitializeWorkspace.Enabled {
		subs = append(subs, subroutines.NewInitializeWorkspaceSubroutine(mgr.GetLocalManager().GetClient(), vsConfig, cfg.KCP.InitializerName))
	}

	return &LogicalClusterReconciler{
		lifecycle: builder.NewBuilder("openmcp-init-operator", "LogicalClusterReconciler", subs, log).
			BuildMultiCluster(mgr),
		log: log,
	}
}

func (r *LogicalClusterReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	reconcileId := uuid.New().String()

	log := r.log.MustChildLoggerWithAttributes("name", req.Name, "namespace", req.Namespace, "reconcile_id", reconcileId)
	if req.ClusterName != "" {
		log = log.MustChildLoggerWithAttributes("cluster", req.ClusterName)
	}
	ctx = logger.SetLoggerInContext(ctx, log)

	instance := &corev1alpha1.LogicalCluster{}
	// Use the lifecycle manager's client through reconcile
	ctxWithCluster := mccontext.WithCluster(ctx, req.ClusterName)
	return r.lifecycle.Reconcile(ctxWithCluster, req, instance)
}

// SetupWithManager sets up the controller with the Manager.
func (r *LogicalClusterReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	wrappedReconciler := mcreconcile.Func(func(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
		if req.ClusterName != "" {
			clusterName := logicalcluster.Name(req.ClusterName)
			r.log.Info().Str("cluster", clusterName.String()).Str("name", req.Name).Str("namespace", req.Namespace).Msg("reconciling logical cluster")
		}
		return r.Reconcile(ctx, req)
	})

	return mcbuilder.ControllerManagedBy(mgr).
		For(&corev1alpha1.LogicalCluster{}).
		Complete(wrappedReconciler)
}
