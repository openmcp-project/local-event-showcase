/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package initializer

import (
	"context"
	"crypto/tls"
	"os"
	"path/filepath"

	"github.com/kcp-dev/multicluster-provider/apiexport"
	platformmeshconfig "github.com/platform-mesh/golang-commons/config"
	openmfpcontext "github.com/platform-mesh/golang-commons/context"
	"github.com/platform-mesh/golang-commons/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"

	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/config"
	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/controller"
)

var (
	setupLog                                         = ctrl.Log.WithName("setup")
	initializerCfg                                   config.InitializerConfig
	defaultCfg                                       *platformmeshconfig.CommonServiceConfig
	scheme                                           *runtime.Scheme
	tlsOpts                                          []func(*tls.Config)
	metricsCertPath, metricsCertName, metricsCertKey string
)

func NewInitializerCmd(v *viper.Viper, cfg *platformmeshconfig.CommonServiceConfig, s *runtime.Scheme) *cobra.Command {
	defaultCfg = cfg
	scheme = s

	initializerCmd := &cobra.Command{
		Use:   "initializer",
		Short: "initializer to reconcile initializing Workspaces",
		Run:   Run,
	}

	err := platformmeshconfig.BindConfigToFlags(v, initializerCmd, &initializerCfg)
	if err != nil {
		panic(err)
	}

	initializerCmd.Flags().StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	initializerCmd.Flags().StringVar(&metricsCertName, "metrics-cert-name", "tls.crt",
		"The name of the metrics server certificate file.")
	initializerCmd.Flags().StringVar(&metricsCertKey, "metrics-cert-key", "tls.key",
		"The name of the metrics server key file.")

	return initializerCmd
}

func Run(_ *cobra.Command, _ []string) {
	log := initLog()
	opts := zap.Options{
		Development: true,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	ctrl.SetLogger(log.ComponentLogger("controller-runtime").Logr())

	ctx, _, shutdown := openmfpcontext.StartContext(log, defaultCfg, defaultCfg.ShutdownTimeout)
	defer shutdown()

	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !defaultCfg.EnableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Create watchers for metrics and webhooks certificates
	var metricsCertWatcher *certwatcher.CertWatcher

	metricsServerOptions := metricsserver.Options{
		BindAddress:   defaultCfg.Metrics.BindAddress,
		SecureServing: defaultCfg.Metrics.Secure,
		TLSOpts:       tlsOpts,
	}

	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		var err error
		metricsCertWatcher, err = certwatcher.New(
			filepath.Join(metricsCertPath, metricsCertName),
			filepath.Join(metricsCertPath, metricsCertKey),
		)
		if err != nil {
			setupLog.Error(err, "to initialize metrics certificate watcher", "error", err)
			os.Exit(1)
		}

		metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, func(config *tls.Config) {
			config.GetCertificate = metricsCertWatcher.GetCertificate
		})
	}

	setupLog.Info("setting up manager")
	managerConfig, err := clientcmd.BuildConfigFromFlags("", initializerCfg.KCP.Kubeconfig)
	if err != nil {
		setupLog.Error(err, "unable to get kubeconfig")
		os.Exit(1)
	}

	// For the initializer, use the virtual workspace URL as base config
	initConfig := rest.CopyConfig(managerConfig)
	initConfig.Host = initializerCfg.KCP.InitializingVirtualWorkspaceURL

	// Initialize APIExport provider for multicluster support
	apiExportName := "openmcp-init.openmcp.io"

	provider, err := apiexport.New(initConfig, apiExportName, apiexport.Options{
		Log:    &ctrl.Log,
		Scheme: scheme,
	})
	if err != nil {
		setupLog.Error(err, "unable to create apiexport provider")
		os.Exit(1)
	}

	mgr, err := mcmanager.New(initConfig, provider, mcmanager.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		HealthProbeBindAddress: defaultCfg.HealthProbeBindAddress,
		LeaderElection:         defaultCfg.LeaderElection.Enabled,
		LeaderElectionID:       "1da1c418.openmcp.io",
		BaseContext:            func() context.Context { return ctx },
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	vsConfig := rest.CopyConfig(initConfig)
	reconciler := controller.NewLogicalClusterReconciler(initializerCfg, mgr, vsConfig, log)
	if err = reconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LogicalCluster")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder
	if metricsCertWatcher != nil {
		setupLog.Info("Adding metrics certificate watcher to local manager")
		if err := mgr.GetLocalManager().Add(metricsCertWatcher); err != nil {
			setupLog.Error(err, "unable to add metrics certificate watcher to manager")
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func initLog() *logger.Logger {
	cfg := logger.DefaultConfig()
	cfg.Level = defaultCfg.Log.Level
	cfg.NoJSON = defaultCfg.Log.NoJson
	log, err := logger.New(cfg)
	if err != nil {
		setupLog.Error(err, "unable to create logger")
		os.Exit(1)
	}
	return log
}
