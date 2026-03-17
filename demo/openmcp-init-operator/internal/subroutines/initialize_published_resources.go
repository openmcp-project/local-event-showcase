package subroutines

import (
	"context"
	"fmt"
	"strings"
	"time"

	syncAgentv1alpha1 "github.com/kcp-dev/api-syncagent/sdk/apis/syncagent/v1alpha1"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	mccontext "sigs.k8s.io/multicluster-runtime/pkg/context"

	crossplanev1alpha1 "github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/api/v1alpha1"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
)

const (
	InitializePublishedResourcesSubroutineName = "InitializePublishedResourcesSubroutine"
	InitializePublishedResourcesFinalizerName  = "publishedresources.openmcp.io/managed-published-resources"
)

var mcpScheme = func() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(syncAgentv1alpha1.AddToScheme(s))
	return s
}()

type InitializePublishedResourcesSubroutine struct {
	onboardingClient client.Client
	cfg              *config.OperatorConfig
}

func NewInitializePublishedResourcesSubroutine(onboardingClient client.Client, cfg *config.OperatorConfig) *InitializePublishedResourcesSubroutine {
	return &InitializePublishedResourcesSubroutine{
		onboardingClient: onboardingClient,
		cfg:              cfg,
	}
}

// Finalize implements subroutine.Subroutine.
func (i *InitializePublishedResourcesSubroutine) Finalize(ctx context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	mcpClient, result, opErr := i.getMCPClient(ctx)
	if mcpClient == nil {
		return result, opErr
	}
	return i.finalizeResources(ctx, mcpClient)
}

// finalizeResources deletes all PublishedResource objects created by this subroutine.
func (i *InitializePublishedResourcesSubroutine) finalizeResources(ctx context.Context, mcpClient client.Client) (ctrl.Result, errors.OperatorError) {
	for _, entry := range providerResourceMap {
		for _, resource := range entry.resources {
			pr := &syncAgentv1alpha1.PublishedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name: resource.generateObjectMetaName(entry.prefix),
				},
			}
			if err := mcpClient.Delete(ctx, pr); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, errors.NewOperatorError(err, true, true)
			}
		}
	}

	// Clean up manual provider resources from the MCP cluster.
	if err := deleteGardenerAuthProvider(ctx, mcpClient); err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	return ctrl.Result{}, nil
}

// Finalizers implements subroutine.Subroutine.
func (i *InitializePublishedResourcesSubroutine) Finalizers(_ runtimeobject.RuntimeObject) []string {
	return []string{InitializePublishedResourcesFinalizerName}
}

// GetName implements subroutine.Subroutine.
func (i *InitializePublishedResourcesSubroutine) GetName() string {
	return InitializePublishedResourcesSubroutineName
}

var _ subroutine.Subroutine = &InitializePublishedResourcesSubroutine{}

// Process implements subroutine.Subroutine.
func (i *InitializePublishedResourcesSubroutine) Process(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)
	sourceCrossplane := runtimeObj.(*crossplanev1alpha1.Crossplane)

	clusterID, ok := mccontext.ClusterFrom(ctx)
	if !ok {
		return ctrl.Result{}, errors.NewOperatorError(errors.New("could not get cluster ID from context"), false, true)
	}

	// Read the target Crossplane from the onboarding cluster to check readiness.
	// The source Crossplane (from KCP) has the spec but no conditions;
	// the target (created by CreateCrossplaneSubroutine) has the conditions.
	targetCrossplane := &crossplanev1alpha1.Crossplane{}
	if err := i.onboardingClient.Get(ctx, types.NamespacedName{Name: clusterID, Namespace: i.cfg.MCP.Namespace}, targetCrossplane); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info().Str("clusterID", clusterID).Msg("target Crossplane not found yet, waiting")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	if !allReadyConditionsMet(targetCrossplane.Status.Conditions) {
		log.Info().Str("clusterID", clusterID).Msg("Crossplane is not ready yet, waiting for all Ready conditions to be met")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	mcpClient, result, opErr := i.getMCPClient(ctx)
	if mcpClient == nil {
		return result, opErr
	}

	result, opErr = i.initializeResources(ctx, mcpClient, sourceCrossplane)
	if opErr != nil {
		return result, opErr
	}

	sourceCrossplane.Status.Phase = crossplanev1alpha1.CrossplanePhaseReady
	return result, nil
}

// getMCPClient retrieves the MCP kubeconfig and creates a client targeting the MCP cluster.
func (i *InitializePublishedResourcesSubroutine) getMCPClient(ctx context.Context) (client.Client, ctrl.Result, errors.OperatorError) {
	mcpRestConfig, result, opErr := getMcpKubeconfig(ctx, i.onboardingClient, defaultMCPNamespace, i.cfg.MCP.HostOverride)
	if mcpRestConfig == nil {
		return nil, result, opErr
	}

	mcpClient, err := client.New(mcpRestConfig, client.Options{Scheme: mcpScheme})
	if err != nil {
		return nil, ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	return mcpClient, ctrl.Result{}, nil
}

// allReadyConditionsMet returns true if all conditions with a type ending in "Ready" have status True.
// Returns false if there are no conditions at all.
func allReadyConditionsMet(conditions []metav1.Condition) bool {
	if len(conditions) == 0 {
		return false
	}
	for _, c := range conditions {
		if strings.HasSuffix(c.Type, "Ready") && c.Status != metav1.ConditionTrue {
			return false
		}
	}
	return true
}

func (i *InitializePublishedResourcesSubroutine) initializeResources(ctx context.Context, mcpClient client.Client, crossplane *crossplanev1alpha1.Crossplane) (ctrl.Result, errors.OperatorError) {
	resources := resourcesToPublishForProviders(crossplane.Spec.Providers)

	for _, entry := range resources {
		for _, resource := range entry.resources {
			pr := resource.ToPublishResource(entry.prefix)

			_, err := controllerutil.CreateOrUpdate(ctx, mcpClient, &pr, func() error {
				pr.Spec = resource.ToPublishResource(entry.prefix).Spec
				return nil
			})

			if err != nil {
				return ctrl.Result{}, errors.NewOperatorError(err, true, true)
			}
		}
	}

	// Install manual providers directly on the MCP cluster.
	for _, mp := range getManualProviders(crossplane.Spec.Providers) {
		if mp.Name == gardenerAuthProviderName {
			if err := ensureGardenerAuthProvider(ctx, mcpClient, mp.Version); err != nil {
				return ctrl.Result{}, errors.NewOperatorError(err, true, true)
			}
		}
	}

	return ctrl.Result{}, nil
}

type ResourcesToPublish struct {
	Group            string
	Kind             string
	Version          string
	RelatedResources []RelatableResources // optional
}

type RelatableResources struct {
	Identifier    string
	Origin        string
	Kind          string
	NamePath      string
	NamespacePath string // optional
}

func (r *ResourcesToPublish) ToPublishResource(namePrefix string) syncAgentv1alpha1.PublishedResource {
	pr := syncAgentv1alpha1.PublishedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.generateObjectMetaName(namePrefix),
		},
		Spec: syncAgentv1alpha1.PublishedResourceSpec{
			Naming: &syncAgentv1alpha1.ResourceNaming{
				Name:      "{{ .Object.metadata.name }}",
				Namespace: "{{ .Object.metadata.namespace }}",
			},
			Resource: syncAgentv1alpha1.SourceResourceDescriptor{
				APIGroup: r.Group,
				Kind:     r.Kind,
				Version:  r.Version,
			},
		},
	}

	if len(r.RelatedResources) > 0 {
		for _, related := range r.RelatedResources {
			pr.Spec.Related = append(pr.Spec.Related, related.ToRelatedResourceSpec())
		}
	}

	return pr
}

func (r *ResourcesToPublish) generateObjectMetaName(prefix string) string {
	return strings.ToLower(fmt.Sprintf("%s-%s", prefix, r.Kind))
}

func (r *RelatableResources) ToRelatedResourceSpec() syncAgentv1alpha1.RelatedResourceSpec {
	rr := syncAgentv1alpha1.RelatedResourceSpec{
		Identifier: r.Identifier,
		Origin:     r.Origin,
		Kind:       r.Kind,
		Object: syncAgentv1alpha1.RelatedResourceObject{
			RelatedResourceObjectSpec: syncAgentv1alpha1.RelatedResourceObjectSpec{
				Reference: &syncAgentv1alpha1.RelatedResourceObjectReference{
					Path: r.NamePath,
				},
			},
		},
	}

	if r.NamespacePath != "" {
		rr.Object.Namespace = &syncAgentv1alpha1.RelatedResourceObjectSpec{
			Reference: &syncAgentv1alpha1.RelatedResourceObjectReference{
				Path: r.NamespacePath,
			},
		}
	}

	return rr
}

type providerResources struct {
	prefix    string
	resources []ResourcesToPublish
}

var providerResourceMap = map[string]providerResources{
	"provider-kubernetes": {
		prefix:    "k8s",
		resources: k8sProviderResourcesToPublish,
	},
	gardenerAuthProviderName: {
		prefix:    "gardener-auth",
		resources: gardenerAuthProviderResourcesToPublish,
	},
}

func resourcesToPublishForProviders(providers []*crossplanev1alpha1.CrossplaneProviderConfig) []providerResources {
	var result []providerResources
	for _, p := range providers {
		if pr, ok := providerResourceMap[p.Name]; ok {
			result = append(result, pr)
		}
	}
	return result
}

var k8sProviderResourcesToPublish = []ResourcesToPublish{
	{
		Group:   "kubernetes.crossplane.io",
		Kind:    "ProviderConfig",
		Version: "v1alpha1",
		RelatedResources: []RelatableResources{
			{
				Identifier:    "k8s-credentials",
				Origin:        "kcp",
				Kind:          "Secret",
				NamePath:      "spec.credentials.secretRef.name",
				NamespacePath: "spec.credentials.secretRef.namespace",
			},
		},
	},
	{
		Group:   "kubernetes.crossplane.io",
		Kind:    "Object",
		Version: "v1alpha2",
	},
	{
		Group:   "kubernetes.crossplane.io",
		Kind:    "ObservedObjectCollection",
		Version: "v1alpha1",
	},
}

var gardenerAuthProviderResourcesToPublish = []ResourcesToPublish{
	{
		Group:   "gardener.orchestrate.cloud.sap",
		Kind:    "AdminKubeconfigRequest",
		Version: "v1alpha1",
		RelatedResources: []RelatableResources{
			{
				Identifier:    "connection-secret",
				Origin:        "service",
				Kind:          "Secret",
				NamePath:      "spec.writeConnectionSecretToRef.name",
				NamespacePath: "spec.writeConnectionSecretToRef.namespace",
			},
		},
	},
	{
		Group:   "gardener.orchestrate.cloud.sap",
		Kind:    "ProviderConfig",
		Version: "v1alpha1",
	},
	{
		Group:   "gardener.orchestrate.cloud.sap",
		Kind:    "ProviderConfigUsage",
		Version: "v1alpha1",
	},
	{
		Group:   "gardener.orchestrate.cloud.sap",
		Kind:    "StoreConfig",
		Version: "v1alpha1",
	},
}
