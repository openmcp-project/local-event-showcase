package predicates

import (
	"github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func APIBindingExportReferencePredicate(exportName, exportPath string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		binding, ok := obj.(*v1alpha2.APIBinding)
		if !ok {
			return false
		}

		if binding.Spec.Reference.Export == nil {
			return false
		}

		return binding.Spec.Reference.Export.Name == exportName &&
			binding.Spec.Reference.Export.Path == exportPath
	})
}
