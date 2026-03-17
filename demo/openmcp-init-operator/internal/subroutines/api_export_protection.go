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
	APIExportProtectionSubroutineName = "APIExportProtection"
	// APIExportFinalizerName ensures APIExports are only removed when no APIBindings reference them.
	APIExportFinalizerName = "apis.kcp.io/api-export-protection"
)

// APIExportProtectionSubroutine manages finalization of APIExports: the export can only be
// removed when no APIBindings reference it. APIBindings are assumed to be apis.kcp.io/v1alpha2.
type APIExportProtectionSubroutine struct {
	kcpProvider KCPClientProvider
}

// NewAPIExportProtectionSubroutine returns a new APIExportProtectionSubroutine.
func NewAPIExportProtectionSubroutine(provider KCPClientProvider) *APIExportProtectionSubroutine {
	return &APIExportProtectionSubroutine{kcpProvider: provider}
}

var _ subroutine.Subroutine = (*APIExportProtectionSubroutine)(nil)

func (s *APIExportProtectionSubroutine) GetName() string {
	return APIExportProtectionSubroutineName
}

func (s *APIExportProtectionSubroutine) Finalizers(_ runtimeobject.RuntimeObject) []string {
	return []string{APIExportFinalizerName}
}

func (s *APIExportProtectionSubroutine) Finalize(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	apiExport := runtimeObj.(*apisv1alpha1.APIExport)
	kcpClient, err := s.kcpProvider.KCPClientFromContext(ctx)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}
	hasBindings, err := apiExportHasBindings(ctx, kcpClient, apiExport.Name)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(fmt.Errorf("list APIBindings: %w", err), true, true)
	}
	if hasBindings {
		return ctrl.Result{}, errors.NewOperatorError(
			fmt.Errorf("APIExport %q cannot be removed: APIBindings still reference it", apiExport.Name),
			true, true,
		)
	}
	return ctrl.Result{}, nil
}

func (s *APIExportProtectionSubroutine) Process(ctx context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)
	log.Debug().Msg("APIExportProtection: nothing to process (finalizer is managed by lifecycle)")
	return ctrl.Result{}, nil
}

// apiExportHasBindings returns true if any APIBinding in the cluster references the given APIExport by name.
// We list APIBindings in the same cluster as the APIExport. When ref.Path is empty, the binding
// refers to an export in the APIBinding's (and thus our) logical cluster. When ref.Path is set,
// it refers to another cluster, so we ignore it (we only care about bindings to this export).
func apiExportHasBindings(ctx context.Context, kcpClient client.Client, exportName string) (bool, error) {
	list := &apisv1alpha2.APIBindingList{}
	if err := kcpClient.List(ctx, list); err != nil {
		return false, err
	}
	for i := range list.Items {
		binding := &list.Items[i]
		ref := binding.Spec.Reference.Export
		if ref == nil {
			continue
		}
		if ref.Name != exportName {
			continue
		}
		if ref.Path != "" {
			// Binding targets an APIExport in another logical cluster (path), not ours.
			continue
		}
		return true, nil
	}
	return false, nil
}
