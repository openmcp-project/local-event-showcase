package cmd

import (
	"strings"

	kcpapisv1alpha1 "github.com/kcp-dev/sdk/apis/apis/v1alpha1"
	kcpapisv1alpha2 "github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	kcpcorev1alpha1 "github.com/kcp-dev/sdk/apis/core/v1alpha1"
	mcpv2alpha1 "github.com/openmcp-project/openmcp-operator/api/core/v2alpha1"
	platformmeshconfig "github.com/platform-mesh/golang-commons/config"
	"github.com/platform-mesh/golang-commons/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	crossplanev1alpha1 "github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/api/v1alpha1"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/cmd/initializer"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/cmd/operator"
)

var (
	scheme     = runtime.NewScheme()
	defaultCfg *platformmeshconfig.CommonServiceConfig
	log        *logger.Logger
)

var rootCmd = &cobra.Command{
	Use:   "openmcp-init-operator",
	Short: "operator for OpenMCP workspace initialization",
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kcpapisv1alpha1.AddToScheme(scheme))
	utilruntime.Must(kcpapisv1alpha2.AddToScheme(scheme))
	utilruntime.Must(kcpcorev1alpha1.AddToScheme(scheme))
	utilruntime.Must(mcpv2alpha1.AddToScheme(scheme))
	utilruntime.Must(crossplanev1alpha1.AddToScheme(scheme))

	var err error
	_, defaultCfg, err = platformmeshconfig.NewDefaultConfig(rootCmd)
	if err != nil {
		panic(err)
	}

	operatorV := newViper()
	rootCmd.AddCommand(operator.NewOperatorCmd(operatorV, defaultCfg, scheme))

	initializerV := newViper()
	rootCmd.AddCommand(initializer.NewInitializerCmd(initializerV, defaultCfg, scheme))

	cobra.OnInitialize(initLog)
}

func newViper() *viper.Viper {
	v := viper.NewWithOptions(
		viper.EnvKeyReplacer(strings.NewReplacer("-", "_")),
	)
	v.AutomaticEnv()
	return v
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
