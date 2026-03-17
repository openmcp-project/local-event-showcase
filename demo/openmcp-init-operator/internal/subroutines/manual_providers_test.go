package subroutines

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	crossplanev1alpha1 "github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/api/v1alpha1"
)

func TestIsManualProvider(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"provider-gardener-auth", true},
		{"provider-kubernetes", false},
		{"provider-github", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isManualProvider(tt.name))
		})
	}
}

func TestFilterManualProviders(t *testing.T) {
	tests := []struct {
		name     string
		input    []*crossplanev1alpha1.CrossplaneProviderConfig
		expected []*crossplanev1alpha1.CrossplaneProviderConfig
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty input",
			input:    []*crossplanev1alpha1.CrossplaneProviderConfig{},
			expected: nil,
		},
		{
			name: "only normal providers",
			input: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-kubernetes", Version: "0.15.0"},
			},
			expected: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-kubernetes", Version: "0.15.0"},
			},
		},
		{
			name: "only manual providers",
			input: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-gardener-auth", Version: "0.0.6"},
			},
			expected: nil,
		},
		{
			name: "mixed providers",
			input: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-kubernetes", Version: "0.15.0"},
				{Name: "provider-gardener-auth", Version: "0.0.6"},
				{Name: "provider-github", Version: "0.18.0"},
			},
			expected: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-kubernetes", Version: "0.15.0"},
				{Name: "provider-github", Version: "0.18.0"},
			},
		},
		{
			name: "skips nil entries",
			input: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-kubernetes", Version: "0.15.0"},
				nil,
				{Name: "provider-gardener-auth", Version: "0.0.6"},
			},
			expected: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-kubernetes", Version: "0.15.0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterManualProviders(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetManualProviders(t *testing.T) {
	tests := []struct {
		name     string
		input    []*crossplanev1alpha1.CrossplaneProviderConfig
		expected []*crossplanev1alpha1.CrossplaneProviderConfig
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "no manual providers",
			input: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-kubernetes", Version: "0.15.0"},
			},
			expected: nil,
		},
		{
			name: "returns only manual providers",
			input: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-kubernetes", Version: "0.15.0"},
				{Name: "provider-gardener-auth", Version: "0.0.6"},
				{Name: "provider-github", Version: "0.18.0"},
			},
			expected: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-gardener-auth", Version: "0.0.6"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getManualProviders(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEnsureGardenerAuthProvider(t *testing.T) {
	ctx := context.Background()
	mcpClient := fake.NewClientBuilder().Build()

	err := ensureGardenerAuthProvider(ctx, mcpClient, "0.0.6")
	require.NoError(t, err)

	// Verify ControllerConfig was created
	cc := &unstructured.Unstructured{}
	cc.SetAPIVersion("pkg.crossplane.io/v1alpha1")
	cc.SetKind("ControllerConfig")
	err = mcpClient.Get(ctx, types.NamespacedName{Name: gardenerAuthControllerConfigName}, cc)
	require.NoError(t, err)

	image, found, err := unstructured.NestedString(cc.Object, "spec", "image")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "ghcr.io/openmcp-project/local-event-showcase/crossplane/provider-gardener-auth-controller:0.0.6", image)

	// Verify Provider was created
	prov := &unstructured.Unstructured{}
	prov.SetAPIVersion("pkg.crossplane.io/v1")
	prov.SetKind("Provider")
	err = mcpClient.Get(ctx, types.NamespacedName{Name: gardenerAuthProviderResourceName}, prov)
	require.NoError(t, err)

	pkg, found, err := unstructured.NestedString(prov.Object, "spec", "package")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "ghcr.io/openmcp-project/local-event-showcase/crossplane/provider-gardener-auth:0.0.6", pkg)

	pullPolicy, found, err := unstructured.NestedString(prov.Object, "spec", "packagePullPolicy")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "IfNotPresent", pullPolicy)

	configRef, found, err := unstructured.NestedString(prov.Object, "spec", "controllerConfigRef", "name")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, gardenerAuthControllerConfigName, configRef)
}

func TestEnsureGardenerAuthProvider_UpdatesExisting(t *testing.T) {
	ctx := context.Background()
	mcpClient := fake.NewClientBuilder().Build()

	// Create with initial version
	require.NoError(t, ensureGardenerAuthProvider(ctx, mcpClient, "0.0.5"))

	// Update to new version
	require.NoError(t, ensureGardenerAuthProvider(ctx, mcpClient, "0.0.6"))

	// Verify ControllerConfig has new version
	cc := &unstructured.Unstructured{}
	cc.SetAPIVersion("pkg.crossplane.io/v1alpha1")
	cc.SetKind("ControllerConfig")
	require.NoError(t, mcpClient.Get(ctx, types.NamespacedName{Name: gardenerAuthControllerConfigName}, cc))

	image, _, _ := unstructured.NestedString(cc.Object, "spec", "image")
	assert.Contains(t, image, "0.0.6")

	// Verify Provider has new version
	prov := &unstructured.Unstructured{}
	prov.SetAPIVersion("pkg.crossplane.io/v1")
	prov.SetKind("Provider")
	require.NoError(t, mcpClient.Get(ctx, types.NamespacedName{Name: gardenerAuthProviderResourceName}, prov))

	pkg, _, _ := unstructured.NestedString(prov.Object, "spec", "package")
	assert.Contains(t, pkg, "0.0.6")
}

func TestDeleteGardenerAuthProvider(t *testing.T) {
	ctx := context.Background()
	mcpClient := fake.NewClientBuilder().Build()

	// Create the resources first
	require.NoError(t, ensureGardenerAuthProvider(ctx, mcpClient, "0.0.6"))

	// Delete them
	require.NoError(t, deleteGardenerAuthProvider(ctx, mcpClient))

	// Verify Provider is gone
	prov := &unstructured.Unstructured{}
	prov.SetAPIVersion("pkg.crossplane.io/v1")
	prov.SetKind("Provider")
	err := mcpClient.Get(ctx, types.NamespacedName{Name: gardenerAuthProviderResourceName}, prov)
	assert.Error(t, err)

	// Verify ControllerConfig is gone
	cc := &unstructured.Unstructured{}
	cc.SetAPIVersion("pkg.crossplane.io/v1alpha1")
	cc.SetKind("ControllerConfig")
	err = mcpClient.Get(ctx, types.NamespacedName{Name: gardenerAuthControllerConfigName}, cc)
	assert.Error(t, err)
}

func TestDeleteGardenerAuthProvider_IdempotentWhenNotFound(t *testing.T) {
	ctx := context.Background()
	mcpClient := fake.NewClientBuilder().Build()

	// Should not error when resources don't exist
	require.NoError(t, deleteGardenerAuthProvider(ctx, mcpClient))
}
