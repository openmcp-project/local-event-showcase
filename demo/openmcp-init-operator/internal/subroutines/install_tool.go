package subroutines

import (
	"context"
	"fmt"
	"net/url"

	goHelm "github.com/mittwald/go-helm-client"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	mccontext "sigs.k8s.io/multicluster-runtime/pkg/context"

	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/tool"
)

type InstallToolSubroutine struct {
	kcpProvider      KCPClientProvider
	onboardingClient client.Client
	cfg              *config.OperatorConfig
	toolCfg          *tool.ToolConfig
}

func NewInstallToolSubroutine(kcpProvider KCPClientProvider, onboardingClient client.Client, cfg *config.OperatorConfig, toolCfg *tool.ToolConfig) *InstallToolSubroutine {
	return &InstallToolSubroutine{
		kcpProvider:      kcpProvider,
		onboardingClient: onboardingClient,
		cfg:              cfg,
		toolCfg:          toolCfg,
	}
}

var _ subroutine.Subroutine = &InstallToolSubroutine{}

func (s *InstallToolSubroutine) GetName() string {
	return "InstallTool-" + s.toolCfg.Name
}

func (s *InstallToolSubroutine) Finalizers(_ runtimeobject.RuntimeObject) []string {
	return []string{s.toolCfg.FinalizerPrefix + "/helm-install"}
}

func (s *InstallToolSubroutine) Process(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)

	clusterID, ok := mccontext.ClusterFrom(ctx)
	if !ok {
		return ctrl.Result{}, errors.NewOperatorError(errors.New("could not get cluster ID from context"), false, true)
	}
	log.Info().Str("tool", s.toolCfg.Name).Str("clusterID", clusterID).Msg("InstallTool: starting")

	mcpKubeconfig, result, operatorError := getMcpKubeconfig(ctx, s.onboardingClient, defaultMCPNamespace, s.cfg.MCP.HostOverride)
	if mcpKubeconfig == nil {
		return result, operatorError
	}

	// Build a KCP workspace kubeconfig for this user workspace
	kcpConfig, err := clientcmd.BuildConfigFromFlags("", s.cfg.KCP.Kubeconfig)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	kcpURL, err := url.Parse(kcpConfig.Host)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, false, true)
	}
	kcpURL.Host = "localhost:31000"
	kcpURL.Path = fmt.Sprintf("/clusters/%s", clusterID)
	host := kcpURL.String()

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
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	// Create the KCP kubeconfig secret for this tool on the MCP cluster
	secretName := fmt.Sprintf("kcp-kubeconfig-%s", s.toolCfg.Name)
	kcpKubeconfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
		},
	}

	log.Info().Str("secret", secretName).Msg("InstallTool: creating/updating kcp-kubeconfig secret")
	_, err = controllerutil.CreateOrUpdate(ctx, onboardingClient, kcpKubeconfigSecret, func() error {
		out, marshalErr := clientcmd.Write(kcpKubeconfig)
		if marshalErr != nil {
			return errors.Wrap(marshalErr, "failed to marshal kubeconfig")
		}
		kcpKubeconfigSecret.Data = map[string][]byte{
			"kubeconfig": out,
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	// Bind the tool service account to cluster-admin (demo only)
	saName := s.toolCfg.HelmReleaseName
	crbName := fmt.Sprintf("%s-cluster-admin", s.toolCfg.HelmReleaseName)
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: crbName,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, onboardingClient, crb, func() error {
		crb.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		}
		crb.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: "default",
			},
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	// Extract version from the runtime object's spec
	version := extractVersion(runtimeObj)
	chartVersion := extractChartVersion(runtimeObj)

	helmClient, err := createHelmClient(mcpKubeconfig)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	valuesMap := s.toolCfg.HelmValuesFunc(version, secretName, s.cfg.KCP.PlatformMeshIP)
	valuesYaml := mapToYAML(valuesMap)

	log.Info().Str("release", s.toolCfg.HelmReleaseName).Msg("InstallTool: installing/upgrading helm chart")
	_, err = helmClient.InstallOrUpgradeChart(ctx, &goHelm.ChartSpec{
		ReleaseName: s.toolCfg.HelmReleaseName,
		ChartName:   s.toolCfg.HelmChartURL,
		Version:     chartVersion,
		Namespace:   "default",
		UpgradeCRDs: true,
		ValuesYaml:  valuesYaml,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msg("InstallTool: helm install/upgrade failed")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	if s.toolCfg.PostInstallFunc != nil {
		log.Info().Str("tool", s.toolCfg.Name).Msg("InstallTool: running post-install")
		if postErr := s.toolCfg.PostInstallFunc(ctx, onboardingClient, secretName, s.cfg.KCP.PlatformMeshIP); postErr != nil {
			log.Error().Err(postErr).Msg("InstallTool: post-install failed")
			return ctrl.Result{}, errors.NewOperatorError(postErr, true, true)
		}
	}

	log.Info().Str("tool", s.toolCfg.Name).Msg("InstallTool: completed")
	setPhase(runtimeObj, "Ready")
	return ctrl.Result{}, nil
}

func (s *InstallToolSubroutine) Finalize(ctx context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)

	mcpKubeconfig, result, operatorError := getMcpKubeconfig(ctx, s.onboardingClient, defaultMCPNamespace, s.cfg.MCP.HostOverride)
	if mcpKubeconfig == nil {
		return result, operatorError
	}

	helmClient, err := createHelmClient(mcpKubeconfig)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	log.Info().Str("release", s.toolCfg.HelmReleaseName).Msg("InstallTool: uninstalling helm chart")
	if err := helmClient.UninstallRelease(&goHelm.ChartSpec{
		ReleaseName: s.toolCfg.HelmReleaseName,
		Namespace:   "default",
	}); err != nil {
		log.Error().Err(err).Msg("InstallTool: helm uninstall failed")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	// Clean up secret
	onboardingClient, err := client.New(mcpKubeconfig, client.Options{})
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	secretName := fmt.Sprintf("kcp-kubeconfig-%s", s.toolCfg.Name)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
		},
	}
	if deleteErr := onboardingClient.Delete(ctx, secret); deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
		return ctrl.Result{}, errors.NewOperatorError(deleteErr, true, true)
	}

	// Clean up CRB
	crbName := fmt.Sprintf("%s-cluster-admin", s.toolCfg.HelmReleaseName)
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: crbName,
		},
	}
	if deleteErr := onboardingClient.Delete(ctx, crb); deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
		return ctrl.Result{}, errors.NewOperatorError(deleteErr, true, true)
	}

	log.Info().Str("tool", s.toolCfg.Name).Msg("InstallTool: finalize completed")
	return ctrl.Result{}, nil
}

// setPhase sets the .Status.Phase field on runtime objects that support it.
func setPhase(obj runtimeobject.RuntimeObject, phase string) {
	type phaseSetter interface {
		SetPhase(string)
	}
	if ps, ok := obj.(phaseSetter); ok {
		ps.SetPhase(phase)
	}
}

// extractVersion extracts the .Spec.Version field from a runtime object via its unstructured content.
func extractVersion(obj runtimeobject.RuntimeObject) string {
	type versioned interface {
		GetVersion() string
	}
	if v, ok := obj.(versioned); ok {
		return v.GetVersion()
	}
	return ""
}

// extractChartVersion extracts the .Spec.ChartVersion field from a runtime object.
// Falls back to extractVersion if ChartVersion is not set.
func extractChartVersion(obj runtimeobject.RuntimeObject) string {
	type chartVersioned interface {
		GetChartVersion() string
	}
	if v, ok := obj.(chartVersioned); ok {
		if cv := v.GetChartVersion(); cv != "" {
			return cv
		}
	}
	return extractVersion(obj)
}

// mapToYAML converts a map[string]any to a YAML string for Helm values.
func mapToYAML(values map[string]any) string {
	if len(values) == 0 {
		return ""
	}
	return renderYAMLMap(values, 0)
}

func renderYAMLMap(m map[string]any, indent int) string {
	var result string
	prefix := ""
	for i := 0; i < indent; i++ {
		prefix += "  "
	}
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			result += fmt.Sprintf("%s%s:\n%s", prefix, k, renderYAMLMap(val, indent+1))
		case []any:
			result += fmt.Sprintf("%s%s:\n", prefix, k)
			for _, item := range val {
				switch itemVal := item.(type) {
				case map[string]any:
					result += fmt.Sprintf("%s  -\n%s", prefix, renderYAMLMap(itemVal, indent+2))
				default:
					result += fmt.Sprintf("%s  - %v\n", prefix, itemVal)
				}
			}
		default:
			result += fmt.Sprintf("%s%s: %v\n", prefix, k, val)
		}
	}
	return result
}
