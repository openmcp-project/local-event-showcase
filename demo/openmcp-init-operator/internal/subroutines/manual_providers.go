package subroutines

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	crossplanev1alpha1 "github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/api/v1alpha1"
)

const (
	gardenerAuthProviderName = "provider-gardener-auth"
	gardenerAuthImageBase    = "ghcr.io/openmcp-project/local-event-showcase/crossplane"

	gardenerAuthControllerConfigName = "provider-gardener-auth-config"
	gardenerAuthProviderResourceName = "provider-gardener-auth"
)

var manualProviders = map[string]struct{}{
	gardenerAuthProviderName: {},
}

func isManualProvider(name string) bool {
	_, ok := manualProviders[name]
	return ok
}

// filterManualProviders returns a new slice with manual providers removed.
func filterManualProviders(providers []*crossplanev1alpha1.CrossplaneProviderConfig) []*crossplanev1alpha1.CrossplaneProviderConfig {
	if providers == nil {
		return nil
	}
	var result []*crossplanev1alpha1.CrossplaneProviderConfig
	for _, p := range providers {
		if p != nil && !isManualProvider(p.Name) {
			result = append(result, p)
		}
	}
	return result
}

// getManualProviders returns only the manual providers from the slice.
func getManualProviders(providers []*crossplanev1alpha1.CrossplaneProviderConfig) []*crossplanev1alpha1.CrossplaneProviderConfig {
	var result []*crossplanev1alpha1.CrossplaneProviderConfig
	for _, p := range providers {
		if p != nil && isManualProvider(p.Name) {
			result = append(result, p)
		}
	}
	return result
}

// ensureGardenerAuthProvider creates or updates the ControllerConfig and Provider
// resources for gardener-auth on the MCP cluster.
func ensureGardenerAuthProvider(ctx context.Context, mcpClient client.Client, version string) error {
	controllerConfig := &unstructured.Unstructured{}
	controllerConfig.SetAPIVersion("pkg.crossplane.io/v1alpha1")
	controllerConfig.SetKind("ControllerConfig")
	controllerConfig.SetName(gardenerAuthControllerConfigName)

	if _, err := controllerutil.CreateOrUpdate(ctx, mcpClient, controllerConfig, func() error {
		if err := unstructured.SetNestedField(controllerConfig.Object, gardenerAuthImageBase+"/provider-gardener-auth-controller:"+version, "spec", "image"); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	provider := &unstructured.Unstructured{}
	provider.SetAPIVersion("pkg.crossplane.io/v1")
	provider.SetKind("Provider")
	provider.SetName(gardenerAuthProviderResourceName)

	if _, err := controllerutil.CreateOrUpdate(ctx, mcpClient, provider, func() error {
		if err := unstructured.SetNestedField(provider.Object, gardenerAuthImageBase+"/provider-gardener-auth:"+version, "spec", "package"); err != nil {
			return err
		}
		if err := unstructured.SetNestedField(provider.Object, "IfNotPresent", "spec", "packagePullPolicy"); err != nil {
			return err
		}
		if err := unstructured.SetNestedField(provider.Object, gardenerAuthControllerConfigName, "spec", "controllerConfigRef", "name"); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

// deleteGardenerAuthProvider removes the ControllerConfig and Provider resources
// for gardener-auth from the MCP cluster. Ignores NotFound errors.
func deleteGardenerAuthProvider(ctx context.Context, mcpClient client.Client) error {
	provider := &unstructured.Unstructured{}
	provider.SetAPIVersion("pkg.crossplane.io/v1")
	provider.SetKind("Provider")
	provider.SetName(gardenerAuthProviderResourceName)

	if err := mcpClient.Delete(ctx, provider); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	controllerConfig := &unstructured.Unstructured{}
	controllerConfig.SetAPIVersion("pkg.crossplane.io/v1alpha1")
	controllerConfig.SetKind("ControllerConfig")
	controllerConfig.SetName(gardenerAuthControllerConfigName)

	if err := mcpClient.Delete(ctx, controllerConfig); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}
