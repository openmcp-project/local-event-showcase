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

	apisv1alpha1 "github.com/kcp-dev/sdk/apis/apis/v1alpha1"
	platformmeshconfig "github.com/platform-mesh/golang-commons/config"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/builder"
	mclifecycle "github.com/platform-mesh/golang-commons/controller/lifecycle/multicluster"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/logger"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	mccontext "sigs.k8s.io/multicluster-runtime/pkg/context"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/subroutines"
)

const apiResourceSchemaReconcilerName = "APIResourceSchemaReconciler"

// APIResourceSchemaReconciler watches APIResourceSchemas in user workspaces and manages their
// finalization via APIResourceSchemaProtectionSubroutine: a schema can only be removed when
// no APIExport that references it has active APIBindings.
type APIResourceSchemaReconciler struct {
	lifecycle *mclifecycle.LifecycleManager
	log       *logger.Logger
}

// NewAPIResourceSchemaReconciler returns a new APIResourceSchemaReconciler.
func NewAPIResourceSchemaReconciler(mgr mcmanager.Manager, log *logger.Logger) *APIResourceSchemaReconciler {
	provider := &mcManagerKCPAdapter{mgr: mgr}
	subs := []subroutine.Subroutine{
		subroutines.NewAPIResourceSchemaProtectionSubroutine(provider),
	}
	return &APIResourceSchemaReconciler{
		lifecycle: builder.NewBuilder(operatorName, apiResourceSchemaReconcilerName, subs, log).
			BuildMultiCluster(mgr),
		log: log,
	}
}

// Reconcile delegates to the lifecycle manager.
func (r *APIResourceSchemaReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	return r.lifecycle.Reconcile(mccontext.WithCluster(ctx, req.ClusterName), req, &apisv1alpha1.APIResourceSchema{})
}

// SetupWithManager registers the controller with the multicluster manager.
func (r *APIResourceSchemaReconciler) SetupWithManager(mgr mcmanager.Manager, cfg *platformmeshconfig.CommonServiceConfig, log *logger.Logger, eventPredicates ...predicate.Predicate) error {
	return r.lifecycle.SetupWithManager(mgr, cfg.MaxConcurrentReconciles, apiResourceSchemaReconcilerName, &apisv1alpha1.APIResourceSchema{}, cfg.DebugLabelValue, r, log, eventPredicates...)
}
