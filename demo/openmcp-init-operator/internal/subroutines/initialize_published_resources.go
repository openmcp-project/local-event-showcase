package subroutines

import (
	"context"
	"fmt"
	"strings"

	syncAgentv1alpha1 "github.com/kcp-dev/api-syncagent/sdk/apis/syncagent/v1alpha1"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
)

const (
	InitializePublishedResourcesSubroutineName = "InitializePublishedResourcesSubroutine"
	InitializePublishedResourcesFinalizerName  = "publishedresources.openmcp.io/managed-published-resources"
)

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
	return i.finalizeResources(ctx, i.onboardingClient)
}

// finalizeResources deletes all PublishedResource objects created by this subroutine.
func (i *InitializePublishedResourcesSubroutine) finalizeResources(ctx context.Context, onboardingClient client.Client) (ctrl.Result, errors.OperatorError) {
	allResources := [][]ResourcesToPublish{
		esoResourcesToPublish,
		k8sProviderResourcesToPublish,
		githubProviderResourcesToPublish,
	}
	prefixes := []string{"eso", "k8s", "github"}

	for idx, resources := range allResources {
		for _, resource := range resources {
			pr := &syncAgentv1alpha1.PublishedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name: resource.generateObjectMetaName(prefixes[idx]),
				},
			}
			if err := onboardingClient.Delete(ctx, pr); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, errors.NewOperatorError(err, true, true)
			}
		}
	}

	// Delete github provider
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("pkg.crossplane.io/v1")
	u.SetKind("Provider")
	u.SetName("provider-upjet-github")
	if err := onboardingClient.Delete(ctx, u); err != nil && !apierrors.IsNotFound(err) {
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
func (i *InitializePublishedResourcesSubroutine) Process(ctx context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	return i.initializeResources(ctx, i.onboardingClient)
}

func (i *InitializePublishedResourcesSubroutine) initializeResources(ctx context.Context, client client.Client) (ctrl.Result, errors.OperatorError) {
	for _, resource := range esoResourcesToPublish {
		pr := resource.ToPublishResource("eso")

		_, err := controllerutil.CreateOrUpdate(ctx, client, &pr, func() error {
			pr.Spec = resource.ToPublishResource("eso").Spec

			return nil
		})

		if err != nil {
			return ctrl.Result{}, errors.NewOperatorError(err, true, true)
		}
	}

	for _, resource := range k8sProviderResourcesToPublish {
		pr := resource.ToPublishResource("k8s")

		_, err := controllerutil.CreateOrUpdate(ctx, client, &pr, func() error {
			pr.Spec = resource.ToPublishResource("k8s").Spec

			return nil
		})

		if err != nil {
			return ctrl.Result{}, errors.NewOperatorError(err, true, true)
		}
	}

	// Prepare github provider
	u := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "pkg.crossplane.io/v1",
			"kind":       "Provider",
			"metadata": map[string]interface{}{
				"name": "provider-upjet-github",
			},
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, client, &u, func() error {
		u.Object["spec"] = map[string]interface{}{
			"package": "xpkg.upbound.io/crossplane-contrib/provider-upjet-github:v0.18.0",
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	for _, resource := range githubProviderResourcesToPublish {
		pr := resource.ToPublishResource("github")

		_, err := controllerutil.CreateOrUpdate(ctx, client, &pr, func() error {
			pr.Spec = resource.ToPublishResource("github").Spec

			return nil
		})

		if err != nil {
			return ctrl.Result{}, errors.NewOperatorError(err, true, true)
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

var githubProviderResourcesToPublish = []ResourcesToPublish{
	{
		Group:   "github.upbound.io",
		Kind:    "ProviderConfig",
		Version: "v1beta1",
		RelatedResources: []RelatableResources{
			{
				Identifier:    "github-credentials",
				Origin:        "kcp",
				Kind:          "Secret",
				NamePath:      "spec.credentials.secretRef.name",
				NamespacePath: "spec.credentials.secretRef.namespace",
			},
		},
	},
	{
		Group:   "repo.github.upbound.io",
		Kind:    "Repository",
		Version: "v1alpha1",
	},
	{
		Group:   "repo.github.upbound.io",
		Kind:    "DefaultBranch",
		Version: "v1alpha1",
	},
	{
		Group:   "repo.github.upbound.io",
		Kind:    "Branch",
		Version: "v1alpha1",
	},
}

var esoResourcesToPublish = []ResourcesToPublish{
	{
		Group:   "external-secrets.io",
		Kind:    "SecretStore",
		Version: "v1",
		RelatedResources: []RelatableResources{
			{
				Identifier:    "vault-ca-secret",
				Origin:        "kcp",
				Kind:          "Secret",
				NamePath:      "spec.provider.vault.caProvider.name",
				NamespacePath: "spec.provider.vault.caProvider.namespace",
			},
			{
				Identifier:    "vault-token-secret",
				Origin:        "kcp",
				Kind:          "Secret",
				NamePath:      "spec.provider.vault.auth.tokenSecretRef.name",
				NamespacePath: "spec.provider.vault.auth.tokenSecretRef.namespace",
			},
			{
				Identifier:    "vault-approle-role-secret",
				Origin:        "kcp",
				Kind:          "Secret",
				NamePath:      "spec.provider.vault.auth.appRole.roleRef.name",
				NamespacePath: "spec.provider.vault.auth.appRole.roleRef.namespace",
			},
			{
				Identifier:    "vault-approle-secret",
				Origin:        "kcp",
				Kind:          "Secret",
				NamePath:      "spec.provider.vault.auth.appRole.secretRef.name",
				NamespacePath: "spec.provider.vault.auth.appRole.secretRef.namespace",
			},
			{
				Identifier:    "vault-kubernetes-secret",
				Origin:        "kcp",
				Kind:          "Secret",
				NamePath:      "spec.provider.vault.auth.kubernetes.secretRef.name",
				NamespacePath: "spec.provider.vault.auth.kubernetes.secretRef.namespace",
			},
		},
	},
	{
		Group:   "external-secrets.io",
		Kind:    "ClusterSecretStore",
		Version: "v1",
		RelatedResources: []RelatableResources{
			{
				Identifier:    "vault-ca-secret",
				Origin:        "kcp",
				Kind:          "Secret",
				NamePath:      "spec.provider.vault.caProvider.name",
				NamespacePath: "spec.provider.vault.caProvider.namespace",
			},
			{
				Identifier:    "vault-token-secret",
				Origin:        "kcp",
				Kind:          "Secret",
				NamePath:      "spec.provider.vault.auth.tokenSecretRef.name",
				NamespacePath: "spec.provider.vault.auth.tokenSecretRef.namespace",
			},
			{
				Identifier:    "vault-approle-role-secret",
				Origin:        "kcp",
				Kind:          "Secret",
				NamePath:      "spec.provider.vault.auth.appRole.roleRef.name",
				NamespacePath: "spec.provider.vault.auth.appRole.roleRef.namespace",
			},
			{
				Identifier:    "vault-approle-secret",
				Origin:        "kcp",
				Kind:          "Secret",
				NamePath:      "spec.provider.vault.auth.appRole.secretRef.name",
				NamespacePath: "spec.provider.vault.auth.appRole.secretRef.namespace",
			},
			{
				Identifier:    "vault-kubernetes-secret",
				Origin:        "kcp",
				Kind:          "Secret",
				NamePath:      "spec.provider.vault.auth.kubernetes.secretRef.name",
				NamespacePath: "spec.provider.vault.auth.kubernetes.secretRef.namespace",
			},
		},
	},
	{
		Group:   "external-secrets.io",
		Kind:    "ExternalSecret",
		Version: "v1",
		RelatedResources: []RelatableResources{
			{
				Identifier: "target-secret",
				Origin:     "service",
				Kind:       "Secret",
				NamePath:   "spec.target.name",
			},
		},
	},
	{
		Group:   "external-secrets.io",
		Kind:    "ClusterExternalSecret",
		Version: "v1",
	},
}
