package subroutines

import (
	"context"
	"fmt"
	"net/url"

	apisv1alpha1 "github.com/kcp-dev/sdk/apis/apis/v1alpha1"
	"github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	goHelm "github.com/mittwald/go-helm-client"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	mccontext "sigs.k8s.io/multicluster-runtime/pkg/context"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"

	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
)

const (
	DeploySyncSubroutineName = "DeploySyncAgent"
	apiExportName            = "crossplane.openmcp.cloud"
	kcpPathAnnotation        = "kcp.io/path"
)

type SetupSyncAgentSubroutine struct {
	cfg              *config.OperatorConfig
	onboardingClient client.Client
	mgr              mcmanager.Manager
}

func NewSetupSyncAgentSubroutine(mgr mcmanager.Manager, onboardingClient client.Client, cfg *config.OperatorConfig) *SetupSyncAgentSubroutine {
	return &SetupSyncAgentSubroutine{mgr: mgr, onboardingClient: onboardingClient, cfg: cfg}
}

func (r *SetupSyncAgentSubroutine) GetName() string {
	return DeploySyncSubroutineName
}

func (r *SetupSyncAgentSubroutine) Finalize(_ context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	return ctrl.Result{}, nil
}

func (r *SetupSyncAgentSubroutine) Finalizers(_ runtimeobject.RuntimeObject) []string { // coverage-ignore
	return []string{}
}

func (r *SetupSyncAgentSubroutine) Process(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)
	apiBinding := runtimeObj.(*v1alpha2.APIBinding)

	clusterID, _ := mccontext.ClusterFrom(ctx)
	log.Info().
		Str("apiBinding", apiBinding.Name).
		Str("clusterID", clusterID).
		Msg("SetupSyncAgent: starting Process")

	kcpPath := apiBinding.GetAnnotations()[kcpPathAnnotation]
	if kcpPath == "" {
		log.Error().
			Str("apiBinding", apiBinding.Name).
			Msg("SetupSyncAgent: missing kcp.io/path annotation")
		return ctrl.Result{}, errors.NewOperatorError(
			fmt.Errorf("APIBinding %s is missing required annotation %s", apiBinding.Name, kcpPathAnnotation),
			false, true)
	}
	log.Info().Str("kcpPath", kcpPath).Msg("SetupSyncAgent: resolved KCP path")

	// Ensure the workspace APIExport exists
	log.Info().Msg("SetupSyncAgent: ensuring workspace APIExport")
	if err := r.ensureWorkspaceAPIExport(ctx); err != nil {
		log.Error().Err(err).Msg("SetupSyncAgent: failed to ensure workspace APIExport")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}
	log.Info().Msg("SetupSyncAgent: workspace APIExport ensured successfully")

	// Ensure ContentConfiguration for the Crossplane UI
	log.Info().Msg("SetupSyncAgent: ensuring ContentConfiguration")
	if err := r.ensureContentConfiguration(ctx); err != nil {
		log.Error().Err(err).Msg("SetupSyncAgent: failed to ensure ContentConfiguration")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}
	log.Info().Msg("SetupSyncAgent: ContentConfiguration ensured successfully")

	// Ensure ProviderMetadata for the marketplace
	log.Info().Msg("SetupSyncAgent: ensuring ProviderMetadata")
	if err := r.ensureProviderMetadata(ctx); err != nil {
		log.Error().Err(err).Msg("SetupSyncAgent: failed to ensure ProviderMetadata")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}
	log.Info().Msg("SetupSyncAgent: ProviderMetadata ensured successfully")

	log.Info().Msg("SetupSyncAgent: retrieving MCP kubeconfig")
	mcpKubeconfig, result, operatorError := getMcpKubeconfig(ctx, r.onboardingClient, defaultMCPNamespace, r.cfg.MCP.HostOverride)
	if mcpKubeconfig == nil {
		log.Info().Msg("SetupSyncAgent: MCP kubeconfig not ready, requeuing")
		return result, operatorError
	}
	log.Info().Msg("SetupSyncAgent: MCP kubeconfig retrieved")

	kcpConfig, err := clientcmd.BuildConfigFromFlags("", r.cfg.KCP.Kubeconfig)
	if err != nil {
		log.Error().Err(err).Str("kubeconfigPath", r.cfg.KCP.Kubeconfig).Msg("SetupSyncAgent: failed to build KCP config")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	// Replace the port with 31000 (NodePort inside kind container) but keep localhost
	// hostAliases will map localhost to the platform-mesh Docker IP for routing
	kcpURL, err := url.Parse(kcpConfig.Host)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, false, true)
	}
	kcpURL.Host = "localhost:31000"
	kcpURL.Path = fmt.Sprintf("/clusters/%s", kcpPath)
	host := kcpURL.String()
	log.Info().Str("kcpHost", host).Msg("SetupSyncAgent: constructed KCP host URL")

	kcpKubeconfig := api.Config{
		Clusters: map[string]*api.Cluster{
			"kcp": {
				Server:                   host,
				CertificateAuthorityData: kcpConfig.CAData,
				InsecureSkipTLSVerify:    kcpConfig.Insecure,
			},
		},
		Contexts: map[string]*api.Context{
			"kcp": {
				Cluster:  "kcp",
				AuthInfo: "kcp",
			},
		},
		CurrentContext: "kcp",
		AuthInfos: map[string]*api.AuthInfo{
			"kcp": {
				ClientCertificateData: kcpConfig.CertData,
				ClientKeyData:         kcpConfig.KeyData,
			},
		},
	}

	onboardingClient, err := client.New(mcpKubeconfig, client.Options{})
	if err != nil {
		log.Error().Err(err).Msg("SetupSyncAgent: failed to create onboarding client")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	kcpKubeconfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kcp-kubeconfig",
			Namespace: "default",
		},
	}

	log.Info().Msg("SetupSyncAgent: creating/updating kcp-kubeconfig secret on MCP")
	_, err = controllerutil.CreateOrUpdate(ctx, onboardingClient, kcpKubeconfigSecret, func() error {
		out, err := clientcmd.Write(kcpKubeconfig)
		if err != nil {
			return errors.Wrap(err, "failed to marshal kubeconfig")
		}

		kcpKubeconfigSecret.Data = map[string][]byte{
			"kubeconfig": out,
		}

		return nil
	})
	if err != nil {
		log.Error().Err(err).Msg("SetupSyncAgent: failed to create/update kcp-kubeconfig secret")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}
	log.Info().Msg("SetupSyncAgent: kcp-kubeconfig secret created/updated")

	helmClient, err := createHelmClient(mcpKubeconfig)
	if err != nil {
		log.Error().Err(err).Msg("SetupSyncAgent: failed to create helm client")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	valuesYaml := fmt.Sprintf(`apiExportEndpointSliceName: "%s"
kcpKubeconfig: "kcp-kubeconfig"
replicas: 1
hostAliases:
  enabled: true
  values:
    - ip: "%s"
      hostnames:
        - "localhost"`, apiExportName, r.cfg.KCP.PlatformMeshIP)

	log.Info().Msg("SetupSyncAgent: installing/upgrading api-syncagent helm chart")
	_, err = helmClient.InstallOrUpgradeChart(ctx, &goHelm.ChartSpec{
		ReleaseName: "api-syncagent",
		ChartName:   "https://github.com/kcp-dev/helm-charts/releases/download/api-syncagent-0.5.0/api-syncagent-0.5.0.tgz",
		Namespace:   "default",
		UpgradeCRDs: true,
		ValuesYaml:  valuesYaml,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msg("SetupSyncAgent: helm install/upgrade failed")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	log.Info().Msg("SetupSyncAgent: Process completed successfully")
	return ctrl.Result{}, nil
}

func (r *SetupSyncAgentSubroutine) ensureWorkspaceAPIExport(ctx context.Context) error {
	log := logger.LoadLoggerFromContext(ctx)

	cluster, err := r.mgr.ClusterFromContext(ctx)
	if err != nil {
		log.Error().Err(err).Msg("ensureWorkspaceAPIExport: failed to get cluster from context")
		return err
	}
	kcpClient := cluster.GetClient()
	apiExport := &apisv1alpha1.APIExport{
		ObjectMeta: metav1.ObjectMeta{
			Name: apiExportName,
		},
	}

	log.Info().Str("apiExportName", apiExportName).Msg("ensureWorkspaceAPIExport: creating or updating APIExport")
	result, err := controllerutil.CreateOrUpdate(ctx, kcpClient, apiExport, func() error {
		return nil
	})
	if err != nil {
		log.Error().Err(err).Str("apiExportName", apiExportName).Msg("ensureWorkspaceAPIExport: CreateOrUpdate failed")
	} else {
		log.Info().Str("apiExportName", apiExportName).Str("result", string(result)).Msg("ensureWorkspaceAPIExport: CreateOrUpdate completed")
	}
	return err
}

func (r *SetupSyncAgentSubroutine) ensureContentConfiguration(ctx context.Context) error {
	log := logger.LoadLoggerFromContext(ctx)

	cluster, err := r.mgr.ClusterFromContext(ctx)
	if err != nil {
		log.Error().Err(err).Msg("ensureContentConfiguration: failed to get cluster from context")
		return err
	}
	kcpClient := cluster.GetClient()
	log.Info().Msg("ensureContentConfiguration: got KCP client, preparing ContentConfiguration resource")

	contentConfig := &unstructured.Unstructured{}
	contentConfig.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "ui.platform-mesh.io",
		Version: "v1alpha1",
		Kind:    "ContentConfiguration",
	})
	contentConfig.SetName("openmcp-crossplane")
	contentConfig.SetLabels(map[string]string{
		"ui.platform-mesh.io/entity":      "core_platform-mesh_io_account",
		"ui.platform-mesh.io/content-for": "crossplane.openmcp.cloud",
	})

	inlineContent := `{
  "name": "crossplane.crossplane.openmcp.cloud",
  "luigiConfigFragment": {
    "data": {
      "nodes": [
        {
          "pathSegment": "crossplane",
          "navigationContext": "crossplane",
          "label": "Crossplane",
          "icon": "customer",
          "order": 100,
          "hideSideNav": false,
          "keepSelectedForChildren": true,
          "virtualTree": true,
          "entityType": "main.core_platform-mesh_io_account",
          "loadingIndicator": { "enabled": false },
          "category": {
            "id": "openmcp",
            "isGroup": true,
            "label": "OpenMCP",
            "order": 90
          },
          "url": "https://{context.organization}.portal.localhost:8443/ui/generic-resource/#/",
          "context": {
            "resourceDefinition": {
              "group": "crossplane.services.openmcp.cloud",
              "version": "v1alpha1",
              "kind": "Crossplane",
              "plural": "Crossplanes",
              "singular": "crossplane",
              "scope": "Cluster"
            }
          }
        }
      ]
    }
  }
}`

	log.Info().Msg("ensureContentConfiguration: calling CreateOrUpdate for ContentConfiguration 'openmcp-crossplane'")
	result, err := controllerutil.CreateOrUpdate(ctx, kcpClient, contentConfig, func() error {
		if err := unstructured.SetNestedMap(contentConfig.Object, map[string]interface{}{
			"inlineConfiguration": map[string]interface{}{
				"content":     inlineContent,
				"contentType": "json",
			},
		}, "spec"); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Error().Err(err).Msg("ensureContentConfiguration: CreateOrUpdate failed")
	} else {
		log.Info().Str("result", string(result)).Msg("ensureContentConfiguration: CreateOrUpdate completed")
	}

	return err
}

func (r *SetupSyncAgentSubroutine) ensureProviderMetadata(ctx context.Context) error {
	log := logger.LoadLoggerFromContext(ctx)

	cluster, err := r.mgr.ClusterFromContext(ctx)
	if err != nil {
		log.Error().Err(err).Msg("ensureProviderMetadata: failed to get cluster from context")
		return err
	}
	kcpClient := cluster.GetClient()
	log.Info().Msg("ensureProviderMetadata: got KCP client, preparing ProviderMetadata resource")

	providerMeta := &unstructured.Unstructured{}
	providerMeta.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "ui.platform-mesh.io",
		Version: "v1alpha1",
		Kind:    "ProviderMetadata",
	})
	providerMeta.SetName(apiExportName)

	log.Info().Str("name", apiExportName).Msg("ensureProviderMetadata: calling CreateOrUpdate")
	result, err := controllerutil.CreateOrUpdate(ctx, kcpClient, providerMeta, func() error {
		return unstructured.SetNestedMap(providerMeta.Object, map[string]interface{}{
			"displayName": "OpenMCP Crossplane",
			"description": "Crossplane-as-a-Service by OpenMCP. Provides declarative infrastructure provisioning and composition across multiple cloud providers using the Kubernetes Resource Model.",
			"tags": []interface{}{
				"crossplane",
				"infrastructure",
				"multi-cloud",
			},
			"contacts": []interface{}{
				map[string]interface{}{
					"displayName": "OpenMCP Team",
					"email":       "ManagedControlPlane@sap.com",
					"role":        []interface{}{"Technical Support"},
				},
			},
			"documentation": []interface{}{
				map[string]interface{}{
					"displayName": "OpenMCP Documentation",
					"url":         "https://github.com/openmcp-project",
				},
			},
			"links": []interface{}{
				map[string]interface{}{
					"displayName": "GitHub Organization",
					"url":         "https://github.com/openmcp-project",
				},
			},
			"preferredSupportChannels": []interface{}{
				map[string]interface{}{
					"displayName": "GitHub Issues",
					"url":         "https://github.com/openmcp-project/mcp-operator/issues",
				},
			},
			"icon": map[string]interface{}{
				"light": map[string]interface{}{
					"data": openmcpIconData,
				},
				"dark": map[string]interface{}{
					"data": openmcpIconData,
				},
			},
		}, "spec")
	})
	if err != nil {
		log.Error().Err(err).Msg("ensureProviderMetadata: CreateOrUpdate failed")
	} else {
		log.Info().Str("result", string(result)).Msg("ensureProviderMetadata: CreateOrUpdate completed")
	}

	return err
}
