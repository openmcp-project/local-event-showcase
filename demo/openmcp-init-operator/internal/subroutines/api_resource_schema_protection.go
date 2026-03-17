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

package subroutines

import (
	"context"
	"fmt"
	"time"

	apisv1alpha1 "github.com/kcp-dev/sdk/apis/apis/v1alpha1"
	apisv1alpha2 "github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	APIResourceSchemaProtectionSubroutineName = "APIResourceSchemaProtection"
	// APIResourceSchemaFinalizerName ensures APIResourceSchemas are only removed when no APIBindings
	// reference an APIExport that includes them.
	APIResourceSchemaFinalizerName = "apis.kcp.io/api-resource-schema-protection"
)

// APIResourceSchemaProtectionSubroutine manages finalization of APIResourceSchemas: a schema can
// only be removed when no APIExport that references it has active APIBindings.
type APIResourceSchemaProtectionSubroutine struct {
	kcpProvider KCPClientProvider
}

// NewAPIResourceSchemaProtectionSubroutine returns a new APIResourceSchemaProtectionSubroutine.
func NewAPIResourceSchemaProtectionSubroutine(provider KCPClientProvider) *APIResourceSchemaProtectionSubroutine {
	return &APIResourceSchemaProtectionSubroutine{kcpProvider: provider}
}

var _ subroutine.Subroutine = (*APIResourceSchemaProtectionSubroutine)(nil)

func (s *APIResourceSchemaProtectionSubroutine) GetName() string {
	return APIResourceSchemaProtectionSubroutineName
}

func (s *APIResourceSchemaProtectionSubroutine) Finalizers(_ runtimeobject.RuntimeObject) []string {
	return []string{APIResourceSchemaFinalizerName}
}

func (s *APIResourceSchemaProtectionSubroutine) Finalize(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)
	schema := runtimeObj.(*apisv1alpha1.APIResourceSchema)

	kcpClient, err := s.kcpProvider.KCPClientFromContext(ctx)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	// Find all APIExports that reference this schema.
	exportNames, err := apiExportsReferencingSchema(ctx, kcpClient, schema.Name)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(fmt.Errorf("list APIExports: %w", err), true, true)
	}

	// For each referencing APIExport, check if it still has APIBindings.
	for _, exportName := range exportNames {
		bound, checkErr := apiExportHasBindings(ctx, kcpClient, exportName)
		if checkErr != nil {
			return ctrl.Result{}, errors.NewOperatorError(fmt.Errorf("check APIBindings for %q: %w", exportName, checkErr), true, true)
		}
		if bound {
			log.Info().Str("schema", schema.Name).Str("apiExport", exportName).
				Msg("APIResourceSchemaProtection: APIBindings still reference an APIExport using this schema, waiting")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (s *APIResourceSchemaProtectionSubroutine) Process(ctx context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)
	log.Debug().Msg("APIResourceSchemaProtection: nothing to process (finalizer is managed by lifecycle)")
	return ctrl.Result{}, nil
}

// apiExportsReferencingSchema returns the names of APIExports whose Spec.Resources reference the given schema name.
func apiExportsReferencingSchema(ctx context.Context, kcpClient client.Client, schemaName string) ([]string, error) {
	list := &apisv1alpha2.APIExportList{}
	if err := kcpClient.List(ctx, list); err != nil {
		return nil, err
	}
	var names []string
	for i := range list.Items {
		for _, res := range list.Items[i].Spec.Resources {
			if res.Schema == schemaName {
				names = append(names, list.Items[i].Name)
				break
			}
		}
	}
	return names, nil
}
