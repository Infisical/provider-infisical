/*
Copyright 2021 Upbound Inc.
*/

package main

import (
	"os"
	"path/filepath"
	"time"

	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	tjcontroller "github.com/crossplane/upjet/v2/pkg/controller"
	ujconversion "github.com/crossplane/upjet/v2/pkg/controller/conversion"
	"github.com/crossplane/upjet/v2/pkg/terraform"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/infisical/provider-infisical/apis"
	"github.com/infisical/provider-infisical/config"
	"github.com/infisical/provider-infisical/internal/clients"
	"github.com/infisical/provider-infisical/internal/controller"
	"github.com/infisical/provider-infisical/internal/features"
	"github.com/infisical/provider-infisical/internal/version"
)

func main() {
	var (
		app                     = kingpin.New(filepath.Base(os.Args[0]), "Terraform based Crossplane provider for Infisical").DefaultEnvars()
		debug                   = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
		syncPeriod              = app.Flag("sync", "Controller manager sync period such as 300ms, 1.5h, or 2h45m").Short('s').Default("1h").Duration()
		pollInterval            = app.Flag("poll", "Poll interval controls how often an individual resource should be checked for drift.").Default("10m").Duration()
		pollStateMetricInterval = app.Flag("poll-state-metric", "State metric recording interval").Default("5s").Duration()
		leaderElection          = app.Flag("leader-election", "Use leader election for the controller manager.").Short('l').Default("false").OverrideDefaultFromEnvar("LEADER_ELECTION").Bool()
		maxReconcileRate        = app.Flag("max-reconcile-rate", "The global maximum rate per second at which resources may be checked for drift from the desired state.").Default("10").Int()
		metricsAddr             = app.Flag("metrics-bind-address", "The address the metric endpoint binds to.").Default("").Envar("METRICS_BIND_ADDRESS").String()

		enableManagementPolicies = app.Flag("enable-management-policies", "Enable support for Management Policies.").Default("true").Envar("ENABLE_MANAGEMENT_POLICIES").Bool()

		healthProbeBindAddr = app.Flag("health-probe-bind-addr", "The address the health probe endpoints (/healthz, /readyz) listen on.").Default(":8081").Envar("HEALTH_PROBE_BIND_ADDRESS").String()

		webhookPort = app.Flag("webhook-port", "The port the API conversion webhook listens on.").Default("9443").Envar("WEBHOOK_PORT").Int()
		certsDir    = app.Flag("certs-dir", "The directory with the TLS certificate (tls.crt) and key (tls.key) of the API conversion webhook. Default: the directory that Crossplane mounts.").Envar("CERTS_DIR").String()

		// External Secret Stores were removed in Crossplane v2. These flags
		// are kept as hidden no-ops, so that existing deployments that still
		// pass them do not fail to start.
		_                          = app.Flag("namespace", "Deprecated: has no effect.").Hidden().Default("crossplane-system").Envar("POD_NAMESPACE").String()
		enableExternalSecretStores = app.Flag("enable-external-secret-stores", "Deprecated: External Secret Stores are no longer supported, has no effect.").Hidden().Default("false").Envar("ENABLE_EXTERNAL_SECRET_STORES").Bool()
		_                          = app.Flag("ess-tls-cert-dir", "Deprecated: has no effect.").Hidden().Envar("ESS_TLS_CERTS_DIR").String()
	)

	kingpin.MustParse(app.Parse(os.Args[1:]))

	// The build sets the Terraform settings from the Makefile, so that they
	// always match the Terraform CLI and provider binaries in the image.
	if version.TerraformVersion == "" || version.TerraformProviderSource == "" || version.TerraformProviderVersion == "" {
		kingpin.Fatalf("the Terraform settings are not set: build the provider with make")
	}

	zl := zap.New(zap.UseDevMode(*debug))
	log := logging.NewLogrLogger(zl.WithName("provider-infisical"))
	if *debug {
		// The controller-runtime runs with a no-op logger by default. It is
		// *very* verbose even at info level, so we only provide it a real
		// logger when we're running in debug mode.
		ctrl.SetLogger(zl)
	}

	log.Debug("Starting", "sync-period", syncPeriod.String(), "poll-interval", pollInterval.String(), "max-reconcile-rate", *maxReconcileRate)

	cfg, err := ctrl.GetConfig()
	kingpin.FatalIfError(err, "Cannot get API server rest config")

	controllerOpts := ctrl.Options{
		LeaderElection:   *leaderElection,
		LeaderElectionID: "crossplane-leader-election-provider-infisical",
		Cache: cache.Options{
			SyncPeriod: syncPeriod,
		},
		HealthProbeBindAddress:     *healthProbeBindAddr,
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		LeaseDuration:              func() *time.Duration { d := 60 * time.Second; return &d }(),
		RenewDeadline:              func() *time.Duration { d := 50 * time.Second; return &d }(),
	}

	// Crossplane mounts the TLS certificate of the provider and sets one of
	// these variables. Older Crossplane versions use WEBHOOK_TLS_CERT_DIR.
	webhookCertsDir := firstNonEmpty(*certsDir, os.Getenv("TLS_SERVER_CERTS_DIR"), os.Getenv("WEBHOOK_TLS_CERT_DIR"), "/tls/server")
	controllerOpts.WebhookServer = webhook.NewServer(webhook.Options{
		CertDir: webhookCertsDir,
		Port:    *webhookPort,
	})

	if metricsAddr != nil && *metricsAddr != "" {
		controllerOpts.Metrics = metricsserver.Options{
			BindAddress: *metricsAddr,
		}
	}

	mgr, err := ctrl.NewManager(cfg, controllerOpts)
	kingpin.FatalIfError(err, "Cannot create controller manager")
	kingpin.FatalIfError(apis.AddToScheme(mgr.GetScheme()), "Cannot add Infisical APIs to scheme")

	metricRecorder := managed.NewMRMetricRecorder()
	stateMetrics := statemetrics.NewMRStateMetrics()

	metrics.Registry.MustRegister(metricRecorder)
	metrics.Registry.MustRegister(stateMetrics)

	o := tjcontroller.Options{
		Options: xpcontroller.Options{
			Logger:                  log,
			GlobalRateLimiter:       ratelimiter.NewGlobal(*maxReconcileRate),
			PollInterval:            *pollInterval,
			MaxConcurrentReconciles: *maxReconcileRate,
			Features:                &feature.Flags{},
			MetricOptions: &xpcontroller.MetricOptions{
				PollStateMetricInterval: *pollStateMetricInterval,
				MRMetrics:               metricRecorder,
				MRStateMetrics:          stateMetrics,
			},
		},
		Provider: config.GetProvider(),
		// use the following WorkspaceStoreOption to enable the shared gRPC mode
		// terraform.WithProviderRunner(terraform.NewSharedProvider(log, os.Getenv("TERRAFORM_NATIVE_PROVIDER_PATH"), terraform.WithNativeProviderArgs("-debuggable")))
		WorkspaceStore: terraform.NewWorkspaceStore(log),
		SetupFn:        clients.TerraformSetupBuilder(version.TerraformVersion, version.TerraformProviderSource, version.TerraformProviderVersion),
	}

	if *enableExternalSecretStores {
		log.Info("External Secret Stores are no longer supported, ignoring the enable-external-secret-stores flag")
	}

	if *enableManagementPolicies {
		o.Features.Enable(features.EnableBetaManagementPolicies)
		log.Info("Beta feature enabled", "flag", features.EnableBetaManagementPolicies)
	}

	// the managed resources serve v1alpha1 and v1alpha2. The API server calls this webhook to convert between them
	if _, err := os.Stat(filepath.Join(webhookCertsDir, "tls.crt")); err == nil {
		kingpin.FatalIfError(ujconversion.RegisterConversions(o.Provider, nil, mgr.GetScheme()), "Cannot register the API conversions")
		kingpin.FatalIfError(controller.SetupWebhookWithManager(mgr), "Cannot setup the API conversion webhook")
		// The API server cannot read or write the managed resources while the conversion webhook is down, so the provider is only ready when the webhook server has started
		kingpin.FatalIfError(mgr.AddReadyzCheck("webhook", mgr.GetWebhookServer().StartedChecker()), "Cannot add the webhook readiness check")
	} else {
		log.Info("No TLS certificate for the API conversion webhook, the webhook is not started. Only the storage version of the APIs can be used.", "certs-dir", webhookCertsDir)
		kingpin.FatalIfError(mgr.AddReadyzCheck("ping", healthz.Ping), "Cannot add the readiness check")
	}
	kingpin.FatalIfError(mgr.AddHealthzCheck("ping", healthz.Ping), "Cannot add the health check")

	kingpin.FatalIfError(controller.Setup(mgr, o), "Cannot setup Infisical controllers")
	kingpin.FatalIfError(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
