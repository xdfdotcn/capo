/*
Copyright 2022 xdfdotcn
*/

package main

import (
	"flag"
	"os"
	"time"

	v3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	cons "github.com/xdfdotcn/capo/pkg/constants"
	ipreservationctrl "github.com/xdfdotcn/capo/pkg/controllers/ipreservation"
	"github.com/xdfdotcn/capo/pkg/handler"
	"github.com/xdfdotcn/capo/pkg/utils"
	wh "github.com/xdfdotcn/capo/pkg/webhook"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	_ "github.com/xdfdotcn/capo/pkg/metrics"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	configv1 "github.com/xdfdotcn/capo/apis/config/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v3.AddToScheme(scheme))
	utilruntime.Must(configv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var (
		metricsAddr          string
		enableLeaderElection bool
		probeAddr            string
		developmentLogging   bool
		verboseLogging       bool
		webhookEnable        bool
		configFile           string
	)

	flag.BoolVar(&developmentLogging, "development", false, "Enable development logging")
	flag.BoolVar(&verboseLogging, "verbose", false, "Enable verbose logging")

	flag.StringVar(&configFile, "config", "",
		"The controller will load its initial configuration from this file. "+
			"Omit this flag to use the default configuration values. "+
			"Command-line flags override configuration from this file.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&webhookEnable, "webhook-enable", true, "Enable webhook")
	flag.Parse()

	ctrl.SetLogger(utils.CreateLogger(verboseLogging, developmentLogging))

	options := ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       cons.IPReserveKey,
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	}

	var err error
	// default CapoConfig
	ctrlConfig := configv1.CapoConfig{
		IPReserveMaxCount: pointer.Int(200),
		IPReserveTime:     metav1.Duration{Duration: 30 * time.Minute},
		IPReleasePeriod:   metav1.Duration{Duration: 5 * time.Minute},
	}
	if configFile != "" {
		options, err = options.AndFrom(ctrl.ConfigFile().AtPath(configFile).OfKind(&ctrlConfig))
		if err != nil {
			setupLog.Error(err, "unable to load the config file")
			os.Exit(1)
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	keeper, err := handler.NewIPKeeper(mgr.GetClient(), &ctrlConfig)
	if err != nil {
		setupLog.Error(err, "unable to new IPKeeper")
	}

	if webhookEnable {
		podValidate := wh.NewPodValidator(mgr.GetClient(), keeper)
		mgr.GetWebhookServer().Register("/pod-ip-reservation", &webhook.Admission{Handler: podValidate})
	}

	if err = ipreservationctrl.NewIPReservationReconciler(mgr.GetClient(), &ctrlConfig, keeper).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IPReservation")
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	//// start cron job
	//go func() {
	//	handler.NewHousekeepingJob("0/5 * * * * ?", mgr.GetClient()).ExecCronJob()
	//}()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
