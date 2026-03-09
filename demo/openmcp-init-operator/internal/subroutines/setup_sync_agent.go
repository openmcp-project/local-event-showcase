package subroutines

import (
	"context"
	"fmt"
	"net/url"

	apisv1alpha1 "github.com/kcp-dev/sdk/apis/apis/v1alpha1"
	goHelm "github.com/mittwald/go-helm-client"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	mccontext "sigs.k8s.io/multicluster-runtime/pkg/context"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"

	corev1alpha1 "github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/api/core/v1alpha1"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
)

const (
	DeploySyncSubroutineName = "DeploySyncAgent"
	apiExportName            = "crossplane.services.openmcp.cloud"
	apiExportBindRoleName    = "crossplane-apiexport-bind"
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
	managedCP := runtimeObj.(*corev1alpha1.ManagedControlPlane)

	clusterID, ok := mccontext.ClusterFrom(ctx)
	if !ok {
		return ctrl.Result{}, errors.NewOperatorError(errors.New("could not get cluster ID from context"), false, true)
	}
	log.Info().
		Str("managedControlPlane", managedCP.Name).
		Str("clusterID", clusterID).
		Msg("SetupSyncAgent: starting Process")

	// Ensure the workspace APIExport exists
	log.Info().Msg("SetupSyncAgent: ensuring workspace APIExport")
	if err := r.ensureWorkspaceAPIExport(ctx); err != nil {
		log.Error().Err(err).Msg("SetupSyncAgent: failed to ensure workspace APIExport")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}
	log.Info().Msg("SetupSyncAgent: workspace APIExport ensured successfully")

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
	kcpURL.Path = fmt.Sprintf("/clusters/%s", clusterID)
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

	if r.cfg.SyncAgent.ImageRepository != "" {
		valuesYaml += fmt.Sprintf(`
image:
  repository: "%s"`, r.cfg.SyncAgent.ImageRepository)
		if r.cfg.SyncAgent.ImageTag != "" {
			valuesYaml += fmt.Sprintf(`
  tag: "%s"`, r.cfg.SyncAgent.ImageTag)
		}
	}

	log.Info().Msg("SetupSyncAgent: installing/upgrading api-syncagent helm chart")
	_, err = helmClient.InstallOrUpgradeChart(ctx, &goHelm.ChartSpec{
		ReleaseName: "api-syncagent",
		ChartName:   r.cfg.SyncAgent.ChartURL,
		Namespace:   "default",
		UpgradeCRDs: true,
		ValuesYaml:  valuesYaml,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msg("SetupSyncAgent: helm install/upgrade failed")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	managedCP.Status.Phase = corev1alpha1.ManagedControlPlanePhaseReady
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
		return err
	}
	log.Info().Str("apiExportName", apiExportName).Str("result", string(result)).Msg("ensureWorkspaceAPIExport: CreateOrUpdate completed")

	if err := r.ensureAPIExportBindRBAC(ctx, kcpClient); err != nil {
		return err
	}

	return nil
}

func (r *SetupSyncAgentSubroutine) ensureAPIExportBindRBAC(ctx context.Context, kcpClient client.Client) error {
	log := logger.LoadLoggerFromContext(ctx)

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: apiExportBindRoleName,
		},
	}

	log.Info().Msg("ensureAPIExportBindRBAC: creating or updating ClusterRole")
	_, err := controllerutil.CreateOrUpdate(ctx, kcpClient, clusterRole, func() error {
		clusterRole.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apis.kcp.io"},
				Resources: []string{"apiexports"},
				Verbs:     []string{"bind"},
			},
		}
		return nil
	})
	if err != nil {
		log.Error().Err(err).Msg("ensureAPIExportBindRBAC: failed to create/update ClusterRole")
		return err
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: apiExportBindRoleName,
		},
	}

	log.Info().Msg("ensureAPIExportBindRBAC: creating or updating ClusterRoleBinding")
	_, err = controllerutil.CreateOrUpdate(ctx, kcpClient, clusterRoleBinding, func() error {
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     apiExportBindRoleName,
		}
		clusterRoleBinding.Subjects = []rbacv1.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "system:authenticated",
			},
		}
		return nil
	})
	if err != nil {
		log.Error().Err(err).Msg("ensureAPIExportBindRBAC: failed to create/update ClusterRoleBinding")
		return err
	}

	log.Info().Msg("ensureAPIExportBindRBAC: RBAC ensured successfully")
	return nil
}
