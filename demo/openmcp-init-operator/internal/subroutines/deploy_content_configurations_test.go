package subroutines

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/platform-mesh/golang-commons/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	mccontext "sigs.k8s.io/multicluster-runtime/pkg/context"

	crossplanev1alpha1 "github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/api/v1alpha1"
)

// mockKCPClientProvider implements KCPClientProvider with a fixed client or error.
type mockKCPClientProvider struct {
	kcpClient client.Client
	err       error
}

func (m *mockKCPClientProvider) KCPClientFromContext(_ context.Context) (client.Client, error) {
	return m.kcpClient, m.err
}

var _ KCPClientProvider = (*mockKCPClientProvider)(nil)

// newTestContext returns a context with logger and cluster ID set.
func newTestContext(t *testing.T) context.Context {
	t.Helper()
	log, err := logger.New(logger.DefaultConfig())
	require.NoError(t, err)
	ctx := context.Background()
	ctx = mccontext.WithCluster(ctx, "test-cluster-123")
	ctx = logger.SetLoggerInContext(ctx, log)
	return ctx
}

func TestDeployContentConfigurationsSubroutine_GetName(t *testing.T) {
	sub := NewDeployContentConfigurationsSubroutine(nil)
	assert.Equal(t, DeployContentConfigurationsSubroutineName, sub.GetName())
}

func TestDeployContentConfigurationsSubroutine_Finalizers(t *testing.T) {
	sub := NewDeployContentConfigurationsSubroutine(nil)
	finalizers := sub.Finalizers(nil)
	require.Len(t, finalizers, 1)
	assert.Equal(t, DeployContentConfigurationsFinalizerName, finalizers[0])
}

func TestDeployContentConfigurationsSubroutine_Process(t *testing.T) {
	tests := []struct {
		name            string
		providers       []*crossplanev1alpha1.CrossplaneProviderConfig
		existingObjects []client.Object
		expectError     bool
		validate        func(t *testing.T, kcpClient client.Client)
	}{
		{
			name: "creates ContentConfigurations for provider-kubernetes",
			providers: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-kubernetes", Version: "0.11.0"},
			},
			validate: func(t *testing.T, kcpClient client.Client) {
				expectedNames := []string{
					"openmcp-crossplane-k8s-providerconfig",
					"openmcp-crossplane-k8s-object",
					"openmcp-crossplane-k8s-observedobjectcollection",
				}
				ctx := context.Background()
				for _, name := range expectedNames {
					cc := newCC(name)
					err := kcpClient.Get(ctx, types.NamespacedName{Name: name}, cc)
					require.NoError(t, err, "ContentConfiguration %q should exist", name)

					labels := cc.GetLabels()
					assert.Equal(t, entityValue, labels[entityLabel], "entity label for %s", name)
					assert.Equal(t, contentForValue, labels[contentForLabel], "content-for label for %s", name)

					spec, ok := cc.Object["spec"].(map[string]any)
					require.True(t, ok, "spec should be a map for %s", name)
					inline, ok := spec["inlineConfiguration"].(map[string]any)
					require.True(t, ok, "inlineConfiguration should be a map for %s", name)
					assert.Equal(t, "json", inline["contentType"], "contentType for %s", name)

					content, ok := inline["content"].(string)
					require.True(t, ok, "content should be a string for %s", name)
					var fragment map[string]any
					require.NoError(t, json.Unmarshal([]byte(content), &fragment),
						"content should be valid JSON for %s", name)
					assert.Contains(t, fragment, "luigiConfigFragment",
						"fragment should contain luigiConfigFragment for %s", name)
				}
			},
		},
		{
			name: "no ContentConfigurations created for unknown provider",
			providers: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-unknown", Version: "1.0.0"},
			},
			validate: func(t *testing.T, kcpClient client.Client) {
				list := &unstructured.UnstructuredList{}
				list.SetAPIVersion(contentConfigAPIVersion)
				list.SetKind(contentConfigKind + "List")
				err := kcpClient.List(context.Background(), list)
				if err == nil {
					assert.Empty(t, list.Items)
				}
			},
		},
		{
			name:      "no ContentConfigurations created for nil providers",
			providers: nil,
			validate: func(t *testing.T, kcpClient client.Client) {
				list := &unstructured.UnstructuredList{}
				list.SetAPIVersion(contentConfigAPIVersion)
				list.SetKind(contentConfigKind + "List")
				err := kcpClient.List(context.Background(), list)
				if err == nil {
					assert.Empty(t, list.Items)
				}
			},
		},
		{
			name: "updates labels and spec on existing ContentConfiguration",
			providers: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-kubernetes", Version: "0.11.0"},
			},
			existingObjects: []client.Object{
				func() *unstructured.Unstructured {
					obj := newCC("openmcp-crossplane-k8s-object")
					obj.SetLabels(map[string]string{"stale-label": "stale-value"})
					return obj
				}(),
			},
			validate: func(t *testing.T, kcpClient client.Client) {
				cc := newCC("openmcp-crossplane-k8s-object")
				err := kcpClient.Get(context.Background(), types.NamespacedName{Name: "openmcp-crossplane-k8s-object"}, cc)
				require.NoError(t, err)

				labels := cc.GetLabels()
				assert.Equal(t, entityValue, labels[entityLabel])
				assert.Equal(t, contentForValue, labels[contentForLabel])
				assert.NotContains(t, labels, "stale-label", "stale labels should be replaced")

				spec, ok := cc.Object["spec"].(map[string]any)
				require.True(t, ok)
				inline, ok := spec["inlineConfiguration"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "json", inline["contentType"])
			},
		},
		{
			name: "creates ContentConfigurations for provider-gardener-auth",
			providers: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-gardener-auth", Version: "0.0.6"},
			},
			validate: func(t *testing.T, kcpClient client.Client) {
				expectedNames := []string{
					"openmcp-crossplane-gardener-auth-adminkubeconfigrequest",
					"openmcp-crossplane-gardener-auth-providerconfig",
					"openmcp-crossplane-gardener-auth-providerconfigusage",
					"openmcp-crossplane-gardener-auth-storeconfig",
				}
				ctx := context.Background()
				for _, name := range expectedNames {
					cc := newCC(name)
					err := kcpClient.Get(ctx, types.NamespacedName{Name: name}, cc)
					require.NoError(t, err, "ContentConfiguration %q should exist", name)

					labels := cc.GetLabels()
					assert.Equal(t, entityValue, labels[entityLabel], "entity label for %s", name)
					assert.Equal(t, contentForValue, labels[contentForLabel], "content-for label for %s", name)
				}
			},
		},
		{
			name: "creates ContentConfigurations for both providers",
			providers: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-kubernetes", Version: "0.15.0"},
				{Name: "provider-gardener-auth", Version: "0.0.6"},
			},
			validate: func(t *testing.T, kcpClient client.Client) {
				k8sNames := []string{
					"openmcp-crossplane-k8s-providerconfig",
					"openmcp-crossplane-k8s-object",
					"openmcp-crossplane-k8s-observedobjectcollection",
				}
				gardenerNames := []string{
					"openmcp-crossplane-gardener-auth-adminkubeconfigrequest",
					"openmcp-crossplane-gardener-auth-providerconfig",
					"openmcp-crossplane-gardener-auth-providerconfigusage",
					"openmcp-crossplane-gardener-auth-storeconfig",
				}
				ctx := context.Background()
				for _, name := range append(k8sNames, gardenerNames...) {
					cc := newCC(name)
					err := kcpClient.Get(ctx, types.NamespacedName{Name: name}, cc)
					require.NoError(t, err, "ContentConfiguration %q should exist", name)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kcpClient := fake.NewClientBuilder().Build()
			for _, obj := range tt.existingObjects {
				require.NoError(t, kcpClient.Create(context.Background(), obj))
			}

			provider := &mockKCPClientProvider{kcpClient: kcpClient}
			sub := NewDeployContentConfigurationsSubroutine(provider)

			crossplane := &crossplanev1alpha1.Crossplane{
				ObjectMeta: metav1.ObjectMeta{Name: "test-crossplane"},
				Spec:       crossplanev1alpha1.CrossplaneSpec{Providers: tt.providers},
			}

			result, opErr := sub.Process(newTestContext(t), crossplane)

			if tt.expectError {
				require.NotNil(t, opErr)
			} else {
				require.Nil(t, opErr)
				assert.Equal(t, ctrl.Result{}, result)
			}

			if tt.validate != nil {
				tt.validate(t, kcpClient)
			}
		})
	}
}

func TestDeployContentConfigurationsSubroutine_Process_ProviderError(t *testing.T) {
	provider := &mockKCPClientProvider{err: errors.New("cluster not found")}
	sub := NewDeployContentConfigurationsSubroutine(provider)

	crossplane := &crossplanev1alpha1.Crossplane{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: crossplanev1alpha1.CrossplaneSpec{
			Providers: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-kubernetes", Version: "0.11.0"},
			},
		},
	}

	_, opErr := sub.Process(newTestContext(t), crossplane)

	require.NotNil(t, opErr)
	assert.True(t, opErr.Retry())
}

func TestDeployContentConfigurationsSubroutine_Finalize(t *testing.T) {
	tests := []struct {
		name            string
		providers       []*crossplanev1alpha1.CrossplaneProviderConfig
		existingObjects []client.Object
		validate        func(t *testing.T, kcpClient client.Client)
	}{
		{
			name: "deletes existing ContentConfigurations for provider-kubernetes",
			providers: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-kubernetes", Version: "0.11.0"},
			},
			existingObjects: []client.Object{
				newCC("openmcp-crossplane-k8s-providerconfig"),
				newCC("openmcp-crossplane-k8s-object"),
				newCC("openmcp-crossplane-k8s-observedobjectcollection"),
			},
			validate: func(t *testing.T, kcpClient client.Client) {
				deletedNames := []string{
					"openmcp-crossplane-k8s-providerconfig",
					"openmcp-crossplane-k8s-object",
					"openmcp-crossplane-k8s-observedobjectcollection",
				}
				ctx := context.Background()
				for _, name := range deletedNames {
					cc := newCC(name)
					err := kcpClient.Get(ctx, types.NamespacedName{Name: name}, cc)
					require.Error(t, err, "ContentConfiguration %q should have been deleted", name)
				}
			},
		},
		{
			name: "not found is not an error (idempotent finalize)",
			providers: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-kubernetes", Version: "0.11.0"},
			},
			existingObjects: nil,
			validate:        func(t *testing.T, kcpClient client.Client) {},
		},
		{
			name:            "no-op for nil providers",
			providers:       nil,
			existingObjects: nil,
			validate:        func(t *testing.T, kcpClient client.Client) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kcpClient := fake.NewClientBuilder().Build()
			for _, obj := range tt.existingObjects {
				require.NoError(t, kcpClient.Create(context.Background(), obj))
			}

			provider := &mockKCPClientProvider{kcpClient: kcpClient}
			sub := NewDeployContentConfigurationsSubroutine(provider)

			crossplane := &crossplanev1alpha1.Crossplane{
				ObjectMeta: metav1.ObjectMeta{Name: "test-crossplane"},
				Spec:       crossplanev1alpha1.CrossplaneSpec{Providers: tt.providers},
			}

			result, opErr := sub.Finalize(newTestContext(t), crossplane)

			require.Nil(t, opErr)
			assert.Equal(t, ctrl.Result{}, result)

			if tt.validate != nil {
				tt.validate(t, kcpClient)
			}
		})
	}
}

func TestDeployContentConfigurationsSubroutine_Finalize_ProviderError(t *testing.T) {
	provider := &mockKCPClientProvider{err: errors.New("cluster not found")}
	sub := NewDeployContentConfigurationsSubroutine(provider)

	crossplane := &crossplanev1alpha1.Crossplane{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: crossplanev1alpha1.CrossplaneSpec{
			Providers: []*crossplanev1alpha1.CrossplaneProviderConfig{
				{Name: "provider-kubernetes", Version: "0.11.0"},
			},
		},
	}

	_, opErr := sub.Finalize(newTestContext(t), crossplane)

	require.NotNil(t, opErr)
	assert.True(t, opErr.Retry())
}

// newCC builds a minimal ContentConfiguration unstructured object with the correct GVK.
func newCC(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(contentConfigAPIVersion)
	obj.SetKind(contentConfigKind)
	obj.SetName(name)
	return obj
}

func TestBuildContentConfiguration(t *testing.T) {
	resource := ResourcesToPublish{
		Group:   "kubernetes.crossplane.io",
		Kind:    "ProviderConfig",
		Version: "v1alpha1",
	}
	meta := contentConfigMeta{
		DisplayLabel: "ProviderConfigs",
		Icon:         "settings",
		Order:        100,
		PathSegment:  "providerconfigs",
	}

	cc, err := buildContentConfiguration("k8s", resource, meta)
	require.NoError(t, err)
	require.NotNil(t, cc)

	assert.Equal(t, "openmcp-crossplane-k8s-providerconfig", cc.GetName())
	assert.Equal(t, contentConfigAPIVersion, cc.GetAPIVersion())
	assert.Equal(t, contentConfigKind, cc.GetKind())

	labels := cc.GetLabels()
	assert.Equal(t, entityValue, labels[entityLabel])
	assert.Equal(t, contentForValue, labels[contentForLabel])

	spec := cc.Object["spec"].(map[string]any)
	inlineCfg := spec["inlineConfiguration"].(map[string]any)
	assert.Equal(t, "json", inlineCfg["contentType"])

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(inlineCfg["content"].(string)), &parsed))

	fragment := parsed["luigiConfigFragment"].(map[string]any)
	data := fragment["data"].(map[string]any)
	nodes := data["nodes"].([]any)
	require.Len(t, nodes, 1)

	node := nodes[0].(map[string]any)
	assert.Equal(t, "providerconfigs", node["pathSegment"])
	assert.Equal(t, "ProviderConfigs", node["label"])
	assert.Equal(t, "settings", node["icon"])

	resourceDef := node["context"].(map[string]any)["resourceDefinition"].(map[string]any)
	assert.Equal(t, "kubernetes.crossplane.io", resourceDef["group"])
	assert.Equal(t, "v1alpha1", resourceDef["version"])
	assert.Equal(t, "ProviderConfig", resourceDef["kind"])
	assert.Equal(t, "ProviderConfigs", resourceDef["plural"])
	assert.Equal(t, "providerconfig", resourceDef["singular"])
	assert.Equal(t, "Cluster", resourceDef["scope"])

	url := node["url"].(string)
	assert.Contains(t, url, "{context.organization}")
}

func TestContentConfigName(t *testing.T) {
	tests := []struct {
		prefix   string
		kind     string
		expected string
	}{
		{"k8s", "ProviderConfig", "openmcp-crossplane-k8s-providerconfig"},
		{"k8s", "Object", "openmcp-crossplane-k8s-object"},
		{"k8s", "ObservedObjectCollection", "openmcp-crossplane-k8s-observedobjectcollection"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, contentConfigName(tt.prefix, tt.kind))
		})
	}
}
