package subroutines

import (
	"context"
	"fmt"
	"strings"

	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
)

const (
	ConfigureCoreDNSSubroutineName = "ConfigureCoreDNS"

	coreDNSConfigMapName      = "coredns"
	coreDNSConfigMapNamespace = "kube-system"
	coreDNSConfigMapKey       = "Corefile"
	coreDNSPodLabel           = "k8s-app"
	coreDNSPodLabelValue      = "kube-dns"

	gardenerProxyName      = "gardener-api-proxy"
	gardenerProxyNamespace = "kube-system"
	gardenerNodePort       = 30443

	coreDNSBlockMarker = "local.gardener.cloud:53"
)

var _ subroutine.Subroutine = &ConfigureCoreDNSSubroutine{}

type ConfigureCoreDNSSubroutine struct {
	onboardingClient client.Client
	cfg              *config.OperatorConfig
}

func NewConfigureCoreDNSSubroutine(onboardingClient client.Client, cfg *config.OperatorConfig) *ConfigureCoreDNSSubroutine {
	return &ConfigureCoreDNSSubroutine{
		onboardingClient: onboardingClient,
		cfg:              cfg,
	}
}

func (r *ConfigureCoreDNSSubroutine) GetName() string {
	return ConfigureCoreDNSSubroutineName
}

func (r *ConfigureCoreDNSSubroutine) Finalizers(_ runtimeobject.RuntimeObject) []string {
	return []string{}
}

func (r *ConfigureCoreDNSSubroutine) Process(ctx context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)

	mcpKubeconfig, result, operatorError := getMcpKubeconfig(ctx, r.onboardingClient, defaultMCPNamespace, r.cfg.MCP.HostOverride)
	if mcpKubeconfig == nil {
		return result, operatorError
	}

	mcpClient, err := client.New(mcpKubeconfig, client.Options{})
	if err != nil {
		log.Error().Err(err).Msg("ConfigureCoreDNS: failed to create MCP client")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	// Step 1: Create/update the gardener-api-proxy Service (ClusterIP, no selector)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gardenerProxyName,
			Namespace: gardenerProxyNamespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, mcpClient, svc, func() error {
		svc.Spec.Type = corev1.ServiceTypeClusterIP
		svc.Spec.Selector = nil
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "https",
				Protocol:   corev1.ProtocolTCP,
				Port:       443,
				TargetPort: intstr.FromInt32(gardenerNodePort),
			},
		}
		return nil
	})
	if err != nil {
		log.Error().Err(err).Msg("ConfigureCoreDNS: failed to create/update gardener-api-proxy Service")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	// Read back to get the assigned ClusterIP
	if err := mcpClient.Get(ctx, types.NamespacedName{Name: gardenerProxyName, Namespace: gardenerProxyNamespace}, svc); err != nil {
		log.Error().Err(err).Msg("ConfigureCoreDNS: failed to read back Service")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}
	clusterIP := svc.Spec.ClusterIP
	log.Info().Str("clusterIP", clusterIP).Msg("ConfigureCoreDNS: gardener-api-proxy Service ready")

	// Step 2: Create/update the matching Endpoints (v1 Endpoints required for selector-less Services;
	// Kubernetes auto-mirrors them to EndpointSlice)
	ep := &corev1.Endpoints{ //nolint:staticcheck // v1 Endpoints required for selector-less Service routing
		ObjectMeta: metav1.ObjectMeta{
			Name:      gardenerProxyName,
			Namespace: gardenerProxyNamespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, mcpClient, ep, func() error {
		ep.Subsets = []corev1.EndpointSubset{ //nolint:staticcheck // see above
			{
				Addresses: []corev1.EndpointAddress{
					{IP: r.cfg.Gardener.IP},
				},
				Ports: []corev1.EndpointPort{
					{
						Name:     "https",
						Port:     gardenerNodePort,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		}
		return nil
	})
	if err != nil {
		log.Error().Err(err).Msg("ConfigureCoreDNS: failed to create/update gardener-api-proxy Endpoints")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}
	log.Info().Msg("ConfigureCoreDNS: gardener-api-proxy Endpoints ready")

	// Step 3: Patch CoreDNS ConfigMap with template block
	cm := &corev1.ConfigMap{}
	if err := mcpClient.Get(ctx, types.NamespacedName{Name: coreDNSConfigMapName, Namespace: coreDNSConfigMapNamespace}, cm); err != nil {
		log.Error().Err(err).Msg("ConfigureCoreDNS: failed to get coredns ConfigMap")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	corefile, exists := cm.Data[coreDNSConfigMapKey]
	if !exists {
		log.Error().Msg("ConfigureCoreDNS: Corefile key not found in ConfigMap")
		return ctrl.Result{}, errors.NewOperatorError(errors.New("Corefile key not found in coredns ConfigMap"), false, true)
	}

	templateBlock := buildCoreDNSTemplateBlock(clusterIP)

	if strings.Contains(corefile, coreDNSBlockMarker) {
		if strings.Contains(corefile, clusterIP) {
			log.Info().Msg("ConfigureCoreDNS: template block with correct ClusterIP already present")
			return ctrl.Result{}, nil
		}
		// Remove stale block before adding updated one
		corefile = removeCoreDNSBlock(corefile)
	}

	cm.Data[coreDNSConfigMapKey] = templateBlock + corefile
	if err := mcpClient.Update(ctx, cm); err != nil {
		log.Error().Err(err).Msg("ConfigureCoreDNS: failed to update coredns ConfigMap")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}
	log.Info().Str("clusterIP", clusterIP).Msg("ConfigureCoreDNS: template block added to Corefile")

	if opErr := restartCoreDNSPods(ctx, mcpClient); opErr != nil {
		return ctrl.Result{}, opErr
	}

	log.Info().Msg("ConfigureCoreDNS: Process completed successfully")
	return ctrl.Result{}, nil
}

func (r *ConfigureCoreDNSSubroutine) Finalize(ctx context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)

	mcpKubeconfig, result, operatorError := getMcpKubeconfig(ctx, r.onboardingClient, defaultMCPNamespace, r.cfg.MCP.HostOverride)
	if mcpKubeconfig == nil {
		return result, operatorError
	}

	mcpClient, err := client.New(mcpKubeconfig, client.Options{})
	if err != nil {
		log.Error().Err(err).Msg("ConfigureCoreDNS: finalize failed to create MCP client")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	// Remove Service and Endpoints
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: gardenerProxyName, Namespace: gardenerProxyNamespace}}
	if deleteErr := mcpClient.Delete(ctx, svc); deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
		log.Error().Err(deleteErr).Msg("ConfigureCoreDNS: finalize failed to delete Service")
	}
	ep := &corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: gardenerProxyName, Namespace: gardenerProxyNamespace}} //nolint:staticcheck // v1 Endpoints required for selector-less Service
	if deleteErr := mcpClient.Delete(ctx, ep); deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
		log.Error().Err(deleteErr).Msg("ConfigureCoreDNS: finalize failed to delete Endpoints")
	}

	// Remove CoreDNS template block
	cm := &corev1.ConfigMap{}
	if err := mcpClient.Get(ctx, types.NamespacedName{Name: coreDNSConfigMapName, Namespace: coreDNSConfigMapNamespace}, cm); err != nil {
		log.Info().Err(err).Msg("ConfigureCoreDNS: finalize could not get coredns ConfigMap, skipping")
		return ctrl.Result{}, nil
	}

	corefile, exists := cm.Data[coreDNSConfigMapKey]
	if !exists || !strings.Contains(corefile, coreDNSBlockMarker) {
		log.Info().Msg("ConfigureCoreDNS: finalize no template block found, nothing to remove")
		return ctrl.Result{}, nil
	}

	cm.Data[coreDNSConfigMapKey] = removeCoreDNSBlock(corefile)
	if err := mcpClient.Update(ctx, cm); err != nil {
		log.Error().Err(err).Msg("ConfigureCoreDNS: finalize failed to update coredns ConfigMap")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	_ = restartCoreDNSPods(ctx, mcpClient)

	log.Info().Msg("ConfigureCoreDNS: finalize completed")
	return ctrl.Result{}, nil
}

func buildCoreDNSTemplateBlock(clusterIP string) string {
	return fmt.Sprintf(`local.gardener.cloud:53 {
    errors
    cache 30
    template IN A local.gardener.cloud {
        answer "{{ .Name }} 60 IN A %s"
    }
    template IN AAAA local.gardener.cloud {
        rcode NOERROR
    }
}
`, clusterIP)
}

// removeCoreDNSBlock removes the local.gardener.cloud:53 server block from a Corefile string.
// It finds the block start marker and its matching closing brace at depth 0.
func removeCoreDNSBlock(corefile string) string {
	start := strings.Index(corefile, coreDNSBlockMarker)
	if start == -1 {
		return corefile
	}

	// Find the outermost closing brace that ends this server block
	depth := 0
	end := -1
	for i := start; i < len(corefile); i++ {
		switch corefile[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
				// Consume trailing newline
				if end < len(corefile) && corefile[end] == '\n' {
					end++
				}
				break
			}
		}
		if end != -1 {
			break
		}
	}

	if end == -1 {
		return corefile
	}

	return corefile[:start] + corefile[end:]
}

func restartCoreDNSPods(ctx context.Context, mcpClient client.Client) errors.OperatorError {
	log := logger.LoadLoggerFromContext(ctx)

	podList := &corev1.PodList{}
	if err := mcpClient.List(ctx, podList,
		client.InNamespace(coreDNSConfigMapNamespace),
		client.MatchingLabels{coreDNSPodLabel: coreDNSPodLabelValue},
	); err != nil {
		log.Error().Err(err).Msg("ConfigureCoreDNS: failed to list CoreDNS pods")
		return errors.NewOperatorError(err, true, true)
	}

	for i := range podList.Items {
		pod := &podList.Items[i]
		if err := mcpClient.Delete(ctx, pod); err != nil {
			log.Error().Err(err).Str("pod", pod.Name).Msg("ConfigureCoreDNS: failed to delete CoreDNS pod")
		} else {
			log.Info().Str("pod", pod.Name).Msg("ConfigureCoreDNS: deleted CoreDNS pod")
		}
	}

	return nil
}
