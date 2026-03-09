package cmd

import (
	kcpapisv1alpha1 "github.com/kcp-dev/sdk/apis/apis/v1alpha1"
	kcpapisv1alpha2 "github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	kcpcorev1alpha1 "github.com/kcp-dev/sdk/apis/core/v1alpha1"
	platformmeshconfig "github.com/platform-mesh/golang-commons/config"
	"github.com/platform-mesh/golang-commons/logger"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	gardenerv1alpha1 "github.com/openmcp/local-event-showcase/demo/gardener-init-operator/api/v1alpha1"
	"github.com/openmcp/local-event-showcase/demo/gardener-init-operator/cmd/operator"
	"github.com/openmcp/local-event-showcase/demo/gardener-init-operator/internal/config"
)

var (
	scheme      = runtime.NewScheme()
	defaultCfg  *platformmeshconfig.CommonServiceConfig
	operatorCfg config.OperatorConfig
	log         *logger.Logger
)

var rootCmd = &cobra.Command{
	Use:   "gardener-init-operator",
	Short: "operator for Gardener project initialization per KCP workspace",
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kcpapisv1alpha1.AddToScheme(scheme))
	utilruntime.Must(kcpapisv1alpha2.AddToScheme(scheme))
	utilruntime.Must(kcpcorev1alpha1.AddToScheme(scheme))
	utilruntime.Must(gardenerv1alpha1.AddToScheme(scheme))

	defaultCfg = platformmeshconfig.NewDefaultConfig()
	operatorCfg = config.NewOperatorConfig()

	defaultCfg.AddFlags(rootCmd.PersistentFlags())

	operatorCmd := operator.NewOperatorCmd(&operatorCfg, defaultCfg, scheme)
	operatorCfg.AddFlags(operatorCmd.Flags())
	rootCmd.AddCommand(operatorCmd)

	cobra.OnInitialize(initLog)
}

func initLog() {
	logcfg := logger.DefaultConfig()
	logcfg.Level = defaultCfg.Log.Level
	logcfg.NoJSON = defaultCfg.Log.NoJson

	var err error
	log, err = logger.New(logcfg)
	if err != nil {
		panic(err)
	}
	ctrl.SetLogger(log.Logr())
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
