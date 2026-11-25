/*
Copyright 2021.

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

package main

import (
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	konfiguratorv1alpha1 "github.com/stakater/konfigurator/api/v1alpha1"
	"github.com/stakater/konfigurator/controllers"
	xContext "github.com/stakater/konfigurator/pkg/context"
	"github.com/stakater/konfigurator/pkg/webhook"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(konfiguratorv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	options := managerOptions(scheme, managerConfig{
		MetricsAddr:    metricsAddr,
		ProbeAddr:      probeAddr,
		LeaderElection: enableLeaderElection,
		WatchNamespace: getWatchNamespace(),
	})
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	var resourceContext xContext.Context

	if err = (&controllers.KonfiguratorTemplateReconciler{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("KonfiguratorTemplate"),
		Scheme:   mgr.GetScheme(),
		XContext: &resourceContext,
		Webhook:  newWebhookValidator(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KonfiguratorTemplate")
		os.Exit(1)
	}

	if err = (&controllers.PodReconciler{
		Client:  mgr.GetClient(),
		Log:     ctrl.Log.WithName("controllers").WithName("Pod"),
		Context: &resourceContext,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Pod")
		os.Exit(1)
	}

	if err = (&controllers.ServiceReconciler{
		Client:  mgr.GetClient(),
		Log:     ctrl.Log.WithName("controllers").WithName("Service"),
		Context: &resourceContext,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Service")
		os.Exit(1)
	}

	if err = (&controllers.IngressReconciler{
		Client:  mgr.GetClient(),
		Log:     ctrl.Log.WithName("controllers").WithName("Ingress"),
		Context: &resourceContext,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Ingress")
		os.Exit(1)
	}
	if err = (&controllers.PodMetadataInjectorReconciler{
		Client:  mgr.GetClient(),
		Log:     ctrl.Log.WithName("controllers").WithName("PodMetadataInjector"),
		Scheme:  mgr.GetScheme(),
		Context: &resourceContext,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PodMetadataInjector")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

const leaderElectionID = "1d6b66a1.stakater.com"

// managerConfig is the operator's startup configuration (flags + env).
type managerConfig struct {
	MetricsAddr    string
	ProbeAddr      string
	LeaderElection bool
	WatchNamespace string // empty = all namespaces
}

// managerOptions builds the controller-runtime manager options for the binary.
func managerOptions(scheme *runtime.Scheme, cfg managerConfig) ctrl.Options {
	opts := ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: cfg.MetricsAddr},
		HealthProbeBindAddress: cfg.ProbeAddr,
		LeaderElection:         cfg.LeaderElection,
		LeaderElectionID:       leaderElectionID,
		// A panicking reconcile fails that request, never the process (review B3).
		Controller: config.Controller{RecoverPanic: ptr.To(true)},
	}
	if cfg.WatchNamespace != "" {
		opts.Cache = cache.Options{DefaultNamespaces: map[string]cache.Config{cfg.WatchNamespace: {}}}
	}
	return opts
}

// newWebhookValidator builds the template validation-webhook client from operator
// config. With no allowed hosts, every validationWebhookURL is refused (H.2).
func newWebhookValidator() *webhook.Validator {
	hosts := webhook.ParseAllowedHosts(os.Getenv(webhook.AllowedHostsEnv))
	if len(hosts) == 0 {
		setupLog.Info("validation webhooks disabled: " + webhook.AllowedHostsEnv + " is empty")
	} else {
		setupLog.Info("validation webhooks enabled", "allowedHosts", hosts)
	}
	return &webhook.Validator{AllowedHosts: hosts}
}

func getWatchNamespace() string {
	namespace := os.Getenv("WATCH_NAMESPACE")
	if namespace == "" {
		setupLog.Info("WATCH_NAMESPACE is empty, so looking in all namespaces")
	}
	return namespace
}
