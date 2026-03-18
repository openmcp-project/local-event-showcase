package subroutines

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	crossplanev1alpha1 "github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/api/v1alpha1"
)

// KCPClientProvider abstracts the retrieval of a KCP workspace client from context.
// In production this is satisfied by MCManagerAdapter; in tests by a simple mock.
type KCPClientProvider interface {
	KCPClientFromContext(ctx context.Context) (client.Client, error)
}

const (
	DeployContentConfigurationsSubroutineName = "DeployContentConfigurationsSubroutine"
	DeployContentConfigurationsFinalizerName  = "contentconfigurations.opencp.io/managed-content-configurations"

	contentConfigAPIVersion = "ui.platform-mesh.io/v1alpha1"
	contentConfigKind       = "ContentConfiguration"

	entityLabel     = "ui.platform-mesh.io/entity"
	entityValue     = "core_platform-mesh_io_account"
	contentForLabel = "ui.platform-mesh.io/content-for"
	contentForValue = "crossplane.services.opencp.cloud"
)

//+kubebuilder:rbac:groups=ui.platform-mesh.io,resources=contentconfigurations,verbs=get;list;watch;create;update;patch;delete

type DeployContentConfigurationsSubroutine struct {
	kcpProvider KCPClientProvider
}

func NewDeployContentConfigurationsSubroutine(provider KCPClientProvider) *DeployContentConfigurationsSubroutine {
	return &DeployContentConfigurationsSubroutine{kcpProvider: provider}
}

var _ subroutine.Subroutine = &DeployContentConfigurationsSubroutine{}

func (d *DeployContentConfigurationsSubroutine) GetName() string {
	return DeployContentConfigurationsSubroutineName
}

func (d *DeployContentConfigurationsSubroutine) Finalizers(_ runtimeobject.RuntimeObject) []string {
	return []string{DeployContentConfigurationsFinalizerName}
}

func (d *DeployContentConfigurationsSubroutine) Process(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)
	sourceCrossplane := runtimeObj.(*crossplanev1alpha1.Crossplane)

	cluster, err := d.kcpProvider.KCPClientFromContext(ctx)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}
	kcpClient := cluster

	resources := resourcesToPublishForProviders(sourceCrossplane.Spec.Providers)
	for _, entry := range resources {
		providerMeta, ok := contentConfigMetadataByProvider[entry.prefix]
		if !ok {
			continue
		}
		for _, resource := range entry.resources {
			meta, ok := providerMeta[resource.Kind]
			if !ok {
				continue
			}

			cc, err := buildContentConfiguration(entry.prefix, resource, meta)
			if err != nil {
				return ctrl.Result{}, errors.NewOperatorError(fmt.Errorf("building ContentConfiguration for %s: %w", resource.Kind, err), false, true)
			}

			existing := &unstructured.Unstructured{}
			existing.SetAPIVersion(contentConfigAPIVersion)
			existing.SetKind(contentConfigKind)
			existing.SetName(cc.GetName())

			_, createErr := controllerutil.CreateOrUpdate(ctx, kcpClient, existing, func() error {
				existing.SetLabels(cc.GetLabels())
				existing.Object["spec"] = cc.Object["spec"]
				return nil
			})
			if createErr != nil {
				log.Error().Err(createErr).Str("name", cc.GetName()).Msg("failed to create/update ContentConfiguration")
				return ctrl.Result{}, errors.NewOperatorError(createErr, true, true)
			}
			log.Info().Str("name", cc.GetName()).Msg("ContentConfiguration created/updated")
		}
	}

	return ctrl.Result{}, nil
}

func (d *DeployContentConfigurationsSubroutine) Finalize(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)
	sourceCrossplane := runtimeObj.(*crossplanev1alpha1.Crossplane)

	cluster, err := d.kcpProvider.KCPClientFromContext(ctx)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}
	kcpClient := cluster

	resources := resourcesToPublishForProviders(sourceCrossplane.Spec.Providers)
	for _, entry := range resources {
		providerMeta, ok := contentConfigMetadataByProvider[entry.prefix]
		if !ok {
			continue
		}
		for _, resource := range entry.resources {
			if _, ok := providerMeta[resource.Kind]; !ok {
				continue
			}

			cc := &unstructured.Unstructured{}
			cc.SetAPIVersion(contentConfigAPIVersion)
			cc.SetKind(contentConfigKind)
			cc.SetName(contentConfigName(entry.prefix, resource.Kind))

			if deleteErr := kcpClient.Delete(ctx, cc); deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
				log.Error().Err(deleteErr).Str("name", cc.GetName()).Msg("failed to delete ContentConfiguration")
				return ctrl.Result{}, errors.NewOperatorError(deleteErr, true, true)
			}
			log.Info().Str("name", cc.GetName()).Msg("ContentConfiguration deleted")
		}
	}

	return ctrl.Result{}, nil
}

type contentConfigMeta struct {
	DisplayLabel string
	Icon         string
	Order        int
	PathSegment  string
}

var contentConfigMetadataByProvider = map[string]map[string]contentConfigMeta{
	"k8s": {
		"ProviderConfig": {
			DisplayLabel: "ProviderConfigs",
			Icon:         "settings",
			Order:        100,
			PathSegment:  "providerconfigs",
		},
		"Object": {
			DisplayLabel: "Objects",
			Icon:         "document",
			Order:        110,
			PathSegment:  "objects",
		},
		"ObservedObjectCollection": {
			DisplayLabel: "ObservedObjectCollections",
			Icon:         "list",
			Order:        120,
			PathSegment:  "observedobjectcollections",
		},
	},
	"gardener-auth": {
		"AdminKubeconfigRequest": {
			DisplayLabel: "Admin Kubeconfig Requests",
			Icon:         "key-user-settings",
			Order:        200,
			PathSegment:  "adminkubeconfigrequests",
		},
		"ProviderConfig": {
			DisplayLabel: "ProviderConfigs",
			Icon:         "settings",
			Order:        210,
			PathSegment:  "gardener-providerconfigs",
		},
		"ProviderConfigUsage": {
			DisplayLabel: "ProviderConfigUsages",
			Icon:         "connected",
			Order:        220,
			PathSegment:  "providerconfigusages",
		},
		"StoreConfig": {
			DisplayLabel: "StoreConfigs",
			Icon:         "database",
			Order:        230,
			PathSegment:  "storeconfigs",
		},
	},
}

func contentConfigName(prefix, kind string) string {
	return fmt.Sprintf("opencp-crossplane-%s-%s", prefix, strings.ToLower(kind))
}

func buildContentConfiguration(prefix string, resource ResourcesToPublish, meta contentConfigMeta) (*unstructured.Unstructured, error) {
	name := contentConfigName(prefix, resource.Kind)

	node := map[string]any{
		"pathSegment":             meta.PathSegment,
		"navigationContext":       meta.PathSegment,
		"label":                   meta.DisplayLabel,
		"icon":                    meta.Icon,
		"order":                   meta.Order,
		"hideSideNav":             false,
		"keepSelectedForChildren": true,
		"virtualTree":             true,
		"entityType":              "main.core_platform-mesh_io_account.core_opencp_cloud_managedcontrolplane",
		"loadingIndicator":        map[string]any{"enabled": false},
		"category": map[string]any{
			"id":      prefix + "-provider",
			"isGroup": true,
			"label":   categoryLabelForPrefix(prefix),
			"order":   categoryOrderForPrefix(prefix),
		},
		"url": "https://{context.organization}.portal.localhost:8443/ui/generic-resource/#/",
		"context": map[string]any{
			"resourceDefinition": map[string]any{
				"group":    resource.Group,
				"version":  resource.Version,
				"kind":     resource.Kind,
				"plural":   resource.Kind + "s",
				"singular": strings.ToLower(resource.Kind),
				"scope":    "Cluster",
			},
		},
	}

	fragment := map[string]any{
		"name": name,
		"luigiConfigFragment": map[string]any{
			"data": map[string]any{
				"nodes": []any{node},
			},
		},
	}

	contentBytes, err := json.Marshal(fragment)
	if err != nil {
		return nil, err
	}

	cc := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": contentConfigAPIVersion,
			"kind":       contentConfigKind,
			"metadata": map[string]any{
				"name": name,
				"labels": map[string]any{
					entityLabel:     entityValue,
					contentForLabel: contentForValue,
				},
			},
			"spec": map[string]any{
				"inlineConfiguration": map[string]any{
					"contentType": "json",
					"content":     string(contentBytes),
				},
			},
		},
	}

	return cc, nil
}

func categoryLabelForPrefix(prefix string) string {
	switch prefix {
	case "k8s":
		return "Kubernetes Provider"
	case "gardener-auth":
		return "Gardener Auth Provider"
	default:
		return prefix + " Provider"
	}
}

func categoryOrderForPrefix(prefix string) int {
	switch prefix {
	case "k8s":
		return 800
	case "gardener-auth":
		return 810
	default:
		return 900
	}
}
