package subroutines

import (
	"context"
	"fmt"

	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	mccontext "sigs.k8s.io/multicluster-runtime/pkg/context"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"

	gardenerv1alpha1 "github.com/openmcp/local-event-showcase/demo/gardener-init-operator/api/v1alpha1"
	"github.com/openmcp/local-event-showcase/demo/gardener-init-operator/internal/config"
)

const (
	SetupGardenerAccessSubroutineName = "SetupGardenerAccess"
	gardenerKubeconfigSecretName      = "gardener-project-kubeconfig"
	saTokenSecretPrefix               = "openmcp-token"
)

type SetupGardenerAccessSubroutine struct {
	cfg            *config.OperatorConfig
	gardenerClient client.Client
	mgr            mcmanager.Manager
}

func NewSetupGardenerAccessSubroutine(mgr mcmanager.Manager, gardenerClient client.Client, cfg *config.OperatorConfig) *SetupGardenerAccessSubroutine {
	return &SetupGardenerAccessSubroutine{mgr: mgr, gardenerClient: gardenerClient, cfg: cfg}
}

func (r *SetupGardenerAccessSubroutine) GetName() string {
	return SetupGardenerAccessSubroutineName
}

func (r *SetupGardenerAccessSubroutine) Finalize(_ context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	return ctrl.Result{}, nil
}

func (r *SetupGardenerAccessSubroutine) Finalizers(_ runtimeobject.RuntimeObject) []string {
	return []string{}
}

func (r *SetupGardenerAccessSubroutine) Process(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)
	gardenerProject := runtimeObj.(*gardenerv1alpha1.GardenerProject)

	clusterID, ok := mccontext.ClusterFrom(ctx)
	if !ok {
		return ctrl.Result{}, errors.NewOperatorError(errors.New("could not get cluster ID from context"), false, true)
	}

	projectName := shortenClusterID(clusterID)
	projectNamespace := fmt.Sprintf("garden-%s", projectName)

	log.Info().
		Str("clusterID", clusterID).
		Str("projectName", projectName).
		Str("projectNamespace", projectNamespace).
		Msg("SetupGardenerAccess: starting Process")

	// Step 1: Create explicit Secret of type kubernetes.io/service-account-token for the SA
	tokenSecretName := fmt.Sprintf("%s-%s", saTokenSecretPrefix, projectName)
	tokenSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tokenSecretName,
			Namespace: projectNamespace,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": gardenerServiceAccountName,
			},
		},
	}

	if err := r.gardenerClient.Get(ctx, types.NamespacedName{Name: tokenSecretName, Namespace: projectNamespace}, tokenSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, errors.NewOperatorError(err, true, true)
		}
		tokenSecret.Type = corev1.SecretTypeServiceAccountToken
		if err := r.gardenerClient.Create(ctx, tokenSecret); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				log.Error().Err(err).Msg("SetupGardenerAccess: failed to create SA token secret")
				return ctrl.Result{}, errors.NewOperatorError(err, true, true)
			}
		}
		log.Info().Str("name", tokenSecretName).Msg("SetupGardenerAccess: created SA token secret")
	}

	// Re-read to get the populated token
	if err := r.gardenerClient.Get(ctx, types.NamespacedName{Name: tokenSecretName, Namespace: projectNamespace}, tokenSecret); err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	token, exists := tokenSecret.Data["token"]
	if !exists || len(token) == 0 {
		log.Info().Msg("SetupGardenerAccess: SA token not populated yet, requeuing")
		return ctrl.Result{RequeueAfter: 5 * 1e9}, nil // 5 seconds
	}

	// Step 2: Build kubeconfig for Gardener API with bearer token
	gardenerServer := fmt.Sprintf("https://%s:6443", r.cfg.Gardener.IP)
	gardenerKubeconfig := api.Config{
		Clusters: map[string]*api.Cluster{
			"gardener": {
				Server:                gardenerServer,
				InsecureSkipTLSVerify: true,
			},
		},
		Contexts: map[string]*api.Context{
			"gardener": {
				Cluster:  "gardener",
				AuthInfo: "gardener",
			},
		},
		CurrentContext: "gardener",
		AuthInfos: map[string]*api.AuthInfo{
			"gardener": {
				Token: string(token),
			},
		},
	}

	kubeconfigBytes, err := clientcmd.Write(gardenerKubeconfig)
	if err != nil {
		log.Error().Err(err).Msg("SetupGardenerAccess: failed to serialize kubeconfig")
		return ctrl.Result{}, errors.NewOperatorError(err, false, true)
	}

	// Step 3: Get KCP workspace client and store kubeconfig as Secret
	kcpCluster, err := r.mgr.ClusterFromContext(ctx)
	if err != nil {
		log.Error().Err(err).Msg("SetupGardenerAccess: failed to get KCP cluster from context")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}
	kcpClient := kcpCluster.GetClient()

	kubeconfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gardenerKubeconfigSecretName,
			Namespace: r.cfg.RuntimeNamespace,
		},
	}

	log.Info().Msg("SetupGardenerAccess: creating/updating gardener kubeconfig secret in KCP workspace")
	_, err = controllerutil.CreateOrUpdate(ctx, kcpClient, kubeconfigSecret, func() error {
		kubeconfigSecret.Data = map[string][]byte{
			"kubeconfig": kubeconfigBytes,
		}
		return nil
	})
	if err != nil {
		log.Error().Err(err).Msg("SetupGardenerAccess: failed to create/update kubeconfig secret")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	gardenerProject.Status.Phase = gardenerv1alpha1.GardenerProjectPhaseReady
	log.Info().Msg("SetupGardenerAccess: Process completed successfully")
	return ctrl.Result{}, nil
}
