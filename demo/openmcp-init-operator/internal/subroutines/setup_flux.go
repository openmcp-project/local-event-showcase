package subroutines

import (
	"context"
	_ "embed"
	"time"

	"github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	goHelm "github.com/mittwald/go-helm-client"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	"helm.sh/helm/v3/pkg/release"
	helmRepo "helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
)

//go:embed manifests/flux-values.yaml
var fluxValuesYaml string

type SetupFluxSubroutine struct {
	onboardingClient client.Client
	cfg              *config.OperatorConfig
}

const (
	SetupFluxSubroutineName = "SetupFluxAgent"
)

var _ subroutine.Subroutine = &SetupFluxSubroutine{}

func NewSetupFluxSubroutine(onboardingClient client.Client, cfg *config.OperatorConfig) *SetupFluxSubroutine {
	return &SetupFluxSubroutine{
		onboardingClient: onboardingClient,
		cfg:              cfg,
	}
}

// Finalize implements subroutine.Subroutine.
func (s *SetupFluxSubroutine) Finalize(_ context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	return ctrl.Result{}, nil
}

// Finalizers implements subroutine.Subroutine.
func (s *SetupFluxSubroutine) Finalizers(_ runtimeobject.RuntimeObject) []string {
	return []string{}
}

// GetName implements subroutine.Subroutine.
func (s *SetupFluxSubroutine) GetName() string {
	return SetupFluxSubroutineName
}

// Process implements subroutine.Subroutine.
func (s *SetupFluxSubroutine) Process(ctx context.Context, instance runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	_ = instance.(*v1alpha2.APIBinding)
	log := logger.LoadLoggerFromContext(ctx)

	mcpKubeconfig, result, operatorError := getMcpKubeconfig(ctx, s.onboardingClient, defaultMCPNamespace, s.cfg.MCP.HostOverride)
	if mcpKubeconfig == nil {
		return result, operatorError
	}

	helmClient, err := createHelmClient(mcpKubeconfig)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	chartRepo := helmRepo.Entry{
		Name: "flux",
		URL:  "https://fluxcd-community.github.io/helm-charts",
	}

	// Add a chart-repository to the client.
	if err := helmClient.AddOrUpdateChartRepo(chartRepo); err != nil {
		panic(err)
	}

	rel, err := helmClient.GetRelease("flux")
	if err != nil || rel.Info.Status != release.StatusDeployed {
		if err != nil {
			log.Info().Err(err).Msg("upgrading flux chart, error getting release")
		}
		log.Info().Msg("upgrading flux chart")
		_, err = helmClient.InstallOrUpgradeChart(ctx, &goHelm.ChartSpec{
			ReleaseName:     "flux",
			ChartName:       "flux/flux2",
			Version:         "2.16.0",
			Namespace:       "flux-system",
			CreateNamespace: true,
			SkipCRDs:        true,
			ValuesYaml:      fluxValuesYaml,
		}, nil)
		if err != nil {
			return ctrl.Result{}, errors.NewOperatorError(errors.WithStack(err), true, true)
		}
	}

	err = wait.ExponentialBackoffWithContext(ctx, wait.Backoff{Duration: 1 * time.Second, Steps: 10}, func(ctx context.Context) (bool, error) {
		rel, err := helmClient.GetRelease("flux")
		if err != nil {
			return false, err
		}

		return rel.Info.Status == release.StatusDeployed, nil
	})
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	return ctrl.Result{}, nil
}
