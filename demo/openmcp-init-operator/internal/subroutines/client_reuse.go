package subroutines

import (
	"context"
	"net/url"
	"time"

	goHelm "github.com/mittwald/go-helm-client"
	mcpv2alpha1 "github.com/openmcp-project/openmcp-operator/api/core/v2alpha1"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/ratelimiter"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	mccontext "sigs.k8s.io/multicluster-runtime/pkg/context"
)

const (
	tokenAccessKey      = "token_operator-token"
	defaultMCPNamespace = "default"
)

var mcpKubeconfigRateLimiter = mustCreateRateLimiter()

func mustCreateRateLimiter() *ratelimiter.StaticThenExponentialRateLimiter[string] {
	rl, err := ratelimiter.NewStaticThenExponentialRateLimiter[string](ratelimiter.NewConfig(
		ratelimiter.WithRequeueDelay(2*time.Second),
		ratelimiter.WithStaticWindow(30*time.Second),
		ratelimiter.WithExponentialInitialBackoff(5*time.Second),
		ratelimiter.WithExponentialMaxBackoff(60*time.Second),
	))
	if err != nil {
		panic(err)
	}
	return rl
}

// getMcpKubeconfig retrieves the kubeconfig for an MCP cluster from the ManagedControlPlaneV2 status.
// It reads the access secret referenced in status.Access[tokenAccessKey] and parses the kubeconfig.
// If hostOverride is provided, the host in the kubeconfig is replaced (for local testing outside cluster).
func getMcpKubeconfig(ctx context.Context, onboardingClient client.Client, namespace string, hostOverride string) (*rest.Config, ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)

	clusterID, ok := mccontext.ClusterFrom(ctx)
	if !ok {
		return nil, ctrl.Result{}, errors.NewOperatorError(errors.New("could not get cluster ID from context"), false, true)
	}

	mcp := &mcpv2alpha1.ManagedControlPlaneV2{}
	if err := onboardingClient.Get(ctx, types.NamespacedName{Name: clusterID, Namespace: namespace}, mcp); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info().Str("clusterID", clusterID).Msg("ManagedControlPlaneV2 not found, requeuing")
			return nil, ctrl.Result{RequeueAfter: mcpKubeconfigRateLimiter.When(clusterID)}, nil
		}
		log.Error().Err(err).Str("clusterID", clusterID).Msg("failed to get ManagedControlPlaneV2")
		return nil, ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	accessRef, exists := mcp.Status.Access[tokenAccessKey]
	if !exists {
		log.Info().Str("clusterID", clusterID).Str("tokenKey", tokenAccessKey).Msg("MCP access not ready yet, requeuing")
		return nil, ctrl.Result{RequeueAfter: mcpKubeconfigRateLimiter.When(clusterID)}, nil
	}

	secret := &corev1.Secret{}
	if err := onboardingClient.Get(ctx, types.NamespacedName{Name: accessRef.Name, Namespace: namespace}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info().Str("secretName", accessRef.Name).Msg("access secret not found yet, requeuing")
			return nil, ctrl.Result{RequeueAfter: mcpKubeconfigRateLimiter.When(clusterID)}, nil
		}
		log.Error().Err(err).Str("secretName", accessRef.Name).Msg("failed to get access secret")
		return nil, ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	kubeconfigData, exists := secret.Data["kubeconfig"]
	if !exists {
		log.Error().Str("secretName", accessRef.Name).Msg("secret does not contain kubeconfig key")
		return nil, ctrl.Result{}, errors.NewOperatorError(errors.New("access secret missing kubeconfig key"), false, true)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigData)
	if err != nil {
		log.Error().Err(err).Msg("failed to parse kubeconfig from secret")
		return nil, ctrl.Result{}, errors.NewOperatorError(err, false, true)
	}

	// Apply host override if configured (for local testing outside cluster)
	if hostOverride != "" {
		originalURL, parseErr := url.Parse(restConfig.Host)
		if parseErr != nil {
			log.Error().Err(parseErr).Msg("failed to parse host URL from kubeconfig")
			return nil, ctrl.Result{}, errors.NewOperatorError(parseErr, false, true)
		}
		originalURL.Host = hostOverride
		restConfig.Host = originalURL.String()
		log.Debug().Str("newHost", restConfig.Host).Msg("applied MCP host override")
	}

	mcpKubeconfigRateLimiter.Forget(clusterID)
	return restConfig, ctrl.Result{}, nil
}

func createHelmClient(mcpKubeconfig *rest.Config) (goHelm.Client, error) {
	helmClient, err := goHelm.NewClientFromRestConf(&goHelm.RestConfClientOptions{
		Options: &goHelm.Options{
			Namespace: "default",
			Debug:     true,
			Linting:   false,
		},
		RestConfig: mcpKubeconfig,
	})
	return helmClient, err
}
