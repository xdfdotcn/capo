/*
Copyright 2022 xdfdotcn
*/

package ipreservationctrl

import (
	"context"

	v3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	configv1 "github.com/xdfdotcn/capo/apis/config/v1"
	cons "github.com/xdfdotcn/capo/pkg/constants"
	"github.com/xdfdotcn/capo/pkg/handler"
	"github.com/xdfdotcn/capo/pkg/metrics"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// IPReservationReconciler reconciles a IPReservation object
type IPReservationReconciler struct {
	keeper *handler.IPKeeper
	client.Client
	config *configv1.CapoConfig
}

func NewIPReservationReconciler(client client.Client,
	config *configv1.CapoConfig,
	keeper *handler.IPKeeper) *IPReservationReconciler {
	metrics.IPReserveCountMaxLimit.Set(float64(*config.IPReserveMaxCount))
	return &IPReservationReconciler{
		keeper: keeper,
		Client: client,
		config: config,
	}
}

//+kubebuilder:rbac:groups=projectcalico.org,resources=ipreservations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=projectcalico.org,resources=ipreservations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=projectcalico.org,resources=ipreservations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the IPReservation object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *IPReservationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ipReserveLogger := log.FromContext(ctx)

	// TODO(user): your logic here
	ipReserveLogger.V(1).Info("ipReserveLogger", "req", req.String())

	// like cron job
	// we recommend that, instead of changing the default period, the
	// controller requeue, with a constant duration `t`, whenever the controller
	// is "done" with an object, and would otherwise not requeue it, i.e., we
	// recommend the `Reconcile` function return `reconcile.Result{RequeueAfter: t}`,
	// instead of `reconcile.Result{}`
	return ctrl.Result{RequeueAfter: r.config.IPReleasePeriod.Duration}, r.keeper.IpRelease(ctx, ipReserveLogger)
}

// SetupWithManager sets up the controller with the Manager.
func (r *IPReservationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	predicate := predicate.Funcs{
		// The synchronization process is performed only once at startup and is triggered
		// by scheduled tasks thereafter, avoiding the triggering of update events
		CreateFunc: func(e event.CreateEvent) bool {
			//  that only handles one IPReservation reconciler
			return cons.IPReservationName == e.Object.GetName()
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return false
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		For(&v3.IPReservation{}).
		WithEventFilter(predicate).
		Complete(r)
}
