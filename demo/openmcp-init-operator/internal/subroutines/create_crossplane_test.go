package subroutines

import (
	"context"
	"testing"

	kcpapisv1alpha2 "github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	"github.com/platform-mesh/golang-commons/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	mccontext "sigs.k8s.io/multicluster-runtime/pkg/context"

	crossplanev1alpha1 "github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/api/v1alpha1"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
)

func TestCreateCrossplaneSubroutine_GetName(t *testing.T) {
	sub := NewCreateCrossplaneSubroutine(nil, nil, nil)
	assert.Equal(t, "CreateCrossplane", sub.GetName())
}

func TestCreateCrossplaneSubroutine_Finalizers(t *testing.T) {
	sub := NewCreateCrossplaneSubroutine(nil, nil, nil)
	finalizers := sub.Finalizers(nil)
	require.Len(t, finalizers, 1)
	assert.Equal(t, "crossplane.opencp.io/managed-crossplane", finalizers[0])
}

func TestCreateCrossplaneSubroutine_Process(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, crossplanev1alpha1.AddToScheme(scheme))

	const (
		clusterID    = "test-cluster-123"
		mcpNamespace = "mcp-namespace"
	)

	tests := []struct {
		name             string
		sourceCrossplane *crossplanev1alpha1.Crossplane
		existingObjects  []runtime.Object
		expectError      bool
		validateResult   func(t *testing.T, client *fake.ClientBuilder, result ctrl.Result)
	}{
		{
			name: "creates new Crossplane in MCP cluster",
			sourceCrossplane: &crossplanev1alpha1.Crossplane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "source-crossplane",
					Namespace: "source-namespace",
				},
				Spec: crossplanev1alpha1.CrossplaneSpec{
					Version: "1.15.0",
					Providers: []*crossplanev1alpha1.CrossplaneProviderConfig{
						{Name: "provider-kubernetes", Version: "0.11.0"},
						{Name: "provider-github", Version: "0.18.0"},
					},
				},
			},
			existingObjects: nil,
			expectError:     false,
			validateResult: func(t *testing.T, builder *fake.ClientBuilder, result ctrl.Result) {
				assert.Equal(t, ctrl.Result{}, result)
			},
		},
		{
			name: "handles nil providers slice",
			sourceCrossplane: &crossplanev1alpha1.Crossplane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "source-crossplane",
					Namespace: "source-namespace",
				},
				Spec: crossplanev1alpha1.CrossplaneSpec{
					Version:   "1.15.0",
					Providers: nil,
				},
			},
			existingObjects: nil,
			expectError:     false,
		},
		{
			name: "handles empty providers slice",
			sourceCrossplane: &crossplanev1alpha1.Crossplane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "source-crossplane",
					Namespace: "source-namespace",
				},
				Spec: crossplanev1alpha1.CrossplaneSpec{
					Version:   "1.15.0",
					Providers: []*crossplanev1alpha1.CrossplaneProviderConfig{},
				},
			},
			existingObjects: nil,
			expectError:     false,
		},
		{
			name: "filters out manual provider gardener-auth from MCP",
			sourceCrossplane: &crossplanev1alpha1.Crossplane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "source-crossplane",
					Namespace: "source-namespace",
				},
				Spec: crossplanev1alpha1.CrossplaneSpec{
					Version: "1.15.0",
					Providers: []*crossplanev1alpha1.CrossplaneProviderConfig{
						{Name: "provider-kubernetes", Version: "0.15.0"},
						{Name: "provider-gardener-auth", Version: "0.0.6"},
					},
				},
			},
			existingObjects: nil,
			expectError:     false,
			validateResult: func(t *testing.T, builder *fake.ClientBuilder, result ctrl.Result) {
				assert.Equal(t, ctrl.Result{}, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.existingObjects != nil {
				clientBuilder = clientBuilder.WithRuntimeObjects(tt.existingObjects...)
			}
			mcpClient := clientBuilder.Build()

			cfg := &config.OperatorConfig{}
			cfg.MCP.Namespace = mcpNamespace

			sub := NewCreateCrossplaneSubroutine(nil, mcpClient, cfg)

			log, err := logger.New(logger.DefaultConfig())
			require.NoError(t, err)

			ctx := context.Background()
			ctx = mccontext.WithCluster(ctx, clusterID)
			ctx = logger.SetLoggerInContext(ctx, log)

			result, opErr := sub.Process(ctx, tt.sourceCrossplane)

			if tt.expectError {
				require.NotNil(t, opErr)
			} else {
				require.Nil(t, opErr)

				// Verify the unstructured Crossplane was created with the openmcp GVK
				target := &unstructured.Unstructured{}
				target.SetGroupVersionKind(onboardingCrossplaneGVK)
				err := mcpClient.Get(ctx, types.NamespacedName{Name: clusterID, Namespace: mcpNamespace}, target)
				require.NoError(t, err)
				spec, _, _ := unstructured.NestedMap(target.Object, "spec")
				assert.Equal(t, tt.sourceCrossplane.Spec.Version, spec["version"])
			}

			if tt.validateResult != nil {
				tt.validateResult(t, clientBuilder, result)
			}
		})
	}
}

func TestCreateCrossplaneSubroutine_Process_MissingClusterID(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, crossplanev1alpha1.AddToScheme(scheme))

	mcpClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	cfg := &config.OperatorConfig{}
	cfg.MCP.Namespace = "mcp-namespace"

	sub := NewCreateCrossplaneSubroutine(nil, mcpClient, cfg)

	log, err := logger.New(logger.DefaultConfig())
	require.NoError(t, err)

	ctx := logger.SetLoggerInContext(context.Background(), log)

	sourceCrossplane := &crossplanev1alpha1.Crossplane{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec:       crossplanev1alpha1.CrossplaneSpec{Version: "1.15.0"},
	}

	_, opErr := sub.Process(ctx, sourceCrossplane)

	require.NotNil(t, opErr)
	assert.False(t, opErr.Retry())
}

func TestCreateCrossplaneSubroutine_Finalize(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, crossplanev1alpha1.AddToScheme(scheme))
	require.NoError(t, kcpapisv1alpha2.AddToScheme(scheme))

	const (
		clusterID    = "test-cluster-123"
		mcpNamespace = "mcp-namespace"
	)

	// Helper to create an unstructured Crossplane with the openmcp GVK
	makeExistingCrossplane := func() *unstructured.Unstructured {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(onboardingCrossplaneGVK)
		obj.SetName(clusterID)
		obj.SetNamespace(mcpNamespace)
		return obj
	}

	tests := []struct {
		name            string
		existingObjects []runtime.Object
		expectRequeue   bool
		expectError     bool
	}{
		{
			name:            "Crossplane already deleted",
			existingObjects: nil,
			expectRequeue:   false,
			expectError:     false,
		},
		{
			name:            "Crossplane exists and needs deletion",
			existingObjects: []runtime.Object{makeExistingCrossplane()},
			expectRequeue:   true,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.existingObjects != nil {
				clientBuilder = clientBuilder.WithRuntimeObjects(tt.existingObjects...)
			}
			mcpClient := clientBuilder.Build()

			// KCP client for pre-delete resource checks (empty workspace = no remaining resources)
			kcpClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			kcpProvider := &mockKCPClientProvider{kcpClient: kcpClient}

			cfg := &config.OperatorConfig{}
			cfg.MCP.Namespace = mcpNamespace

			sub := NewCreateCrossplaneSubroutine(kcpProvider, mcpClient, cfg)

			log, err := logger.New(logger.DefaultConfig())
			require.NoError(t, err)

			ctx := context.Background()
			ctx = mccontext.WithCluster(ctx, clusterID)
			ctx = logger.SetLoggerInContext(ctx, log)

			result, opErr := sub.Finalize(ctx, nil)

			if tt.expectError {
				require.NotNil(t, opErr)
			} else {
				require.Nil(t, opErr)
			}

			if tt.expectRequeue {
				assert.NotZero(t, result.RequeueAfter)
			} else {
				assert.Equal(t, ctrl.Result{}, result)
			}
		})
	}
}

func TestCreateCrossplaneSubroutine_Finalize_MissingClusterID(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, crossplanev1alpha1.AddToScheme(scheme))

	mcpClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	cfg := &config.OperatorConfig{}
	cfg.MCP.Namespace = "mcp-namespace"

	sub := NewCreateCrossplaneSubroutine(nil, mcpClient, cfg)

	log, err := logger.New(logger.DefaultConfig())
	require.NoError(t, err)

	ctx := logger.SetLoggerInContext(context.Background(), log)

	_, opErr := sub.Finalize(ctx, nil)

	require.NotNil(t, opErr)
	assert.False(t, opErr.Retry())
}

func TestNewOnboardingCrossplane(t *testing.T) {
	obj := newOnboardingCrossplane("test-name", "test-ns")
	assert.Equal(t, "test-name", obj.GetName())
	assert.Equal(t, "test-ns", obj.GetNamespace())
	assert.Equal(t, schema.GroupVersionKind{
		Group:   "crossplane.services.openmcp.cloud",
		Version: "v1alpha1",
		Kind:    "Crossplane",
	}, obj.GroupVersionKind())

	// Cluster-scoped (no namespace)
	obj2 := newOnboardingCrossplane("test-name", "")
	assert.Equal(t, "", obj2.GetNamespace())
}
