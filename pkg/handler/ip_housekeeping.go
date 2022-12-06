package handler

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/go-logr/logr"
	v3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	configv1 "github.com/xdfdotcn/capo/apis/config/v1"
	cons "github.com/xdfdotcn/capo/pkg/constants"
	"github.com/xdfdotcn/capo/pkg/metrics"
	"github.com/xdfdotcn/capo/pkg/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IPKeeper struct {
	client   client.Client
	config   *configv1.CapoConfig
	selector *utils.AnyMatchSelector
}

var (
	podIPMapNsName = types.NamespacedName{
		Name:      cons.IPReservationName,
		Namespace: cons.IPReserveKey,
	}

	ipReservationNsName = types.NamespacedName{
		Name: cons.IPReservationName,
	}
)

func init() {
	// update namespace
	ns := os.Getenv(cons.EnvNamespace)
	if ns != "" {
		podIPMapNsName.Namespace = ns
	}
}

func NewIPKeeper(client client.Client, config *configv1.CapoConfig) (*IPKeeper, error) {
	if config.LabelSelector == nil {
		config.LabelSelector = &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      cons.LabelSelectorStatefulSetPodKey,
					Operator: metav1.LabelSelectorOpExists,
				},
				{
					Key:      cons.LabelSelectorKafkaPodKey,
					Operator: metav1.LabelSelectorOpExists,
				},
			},
		}
	}
	selector, err := metav1.LabelSelectorAsSelector(config.LabelSelector)
	if err != nil {
		return nil, err
	}
	requirements, _ := selector.Requirements()
	anySelector := utils.NewAnyMatchSelector(selector, requirements)

	keeper := &IPKeeper{
		client:   client,
		config:   config,
		selector: anySelector,
	}

	err = keeper.initResources(context.TODO())
	if err != nil {
		return nil, err
	}

	return keeper, nil
}

func (r *IPKeeper) IpRelease(ctx context.Context, logger logr.Logger) error {
	logger.V(1).Info("IpRelease start")
	defer logger.V(1).Info("IpRelease end")

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		ipReservation, podIPMap, err := r.getResources(ctx)
		if err != nil {
			return err
		}

		//The existing CIDR and the new one cannot be repeat and need to be merged.
		//At present, only consider the scenario of a single IP in IPReservation CR
		releaseIPs := getReleaseIPs(podIPMap, logger, r)
		var reserveCIDRs byIp
		reserveCIDRs, totalIP := getReserveCIDRs(ipReservation, releaseIPs)
		metrics.IPReserveCount.Set(float64(totalIP))
		if len(reserveCIDRs) != len(ipReservation.Spec.ReservedCIDRs) {
			sort.Sort(reserveCIDRs)
			ipReservation.Spec.ReservedCIDRs = reserveCIDRs
			err = r.client.Update(ctx, ipReservation)
			if err != nil {
				logger.V(1).Info("ipRelease update ipReservation failed", "err", err.Error())
				return err
			}
		}

		//update configmaps
		err = r.client.Update(ctx, podIPMap)
		if err != nil {
			logger.V(1).Info("ipRelease update podIPMap failed", "err", err.Error())
			return err
		}
		return nil
	})
}

func (r *IPKeeper) getResources(ctx context.Context) (*v3.IPReservation, *v1.ConfigMap, error) {
	podIPMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podIPMapNsName.Name,
			Namespace: podIPMapNsName.Namespace,
		},
	}
	err := r.client.Get(ctx, podIPMapNsName, podIPMap)
	if errors.IsNotFound(err) {
		// create podIPInfo configmaps
		err = r.client.Create(ctx, podIPMap)
	} else if err != nil {
		return nil, nil, err
	}

	if err != nil {
		return nil, podIPMap, err
	}

	ipReservation := &v3.IPReservation{}
	err = r.client.Get(ctx, ipReservationNsName, ipReservation)
	if errors.IsNotFound(err) {
		// create ipReservation
		ipReservation.Name = podIPMapNsName.Name
		ipReservation.Namespace = podIPMapNsName.Namespace
		//add a permanent system reserved IP: 1.1.1.1 in reservedCIDRs
		ipReservation.Spec.ReservedCIDRs = []string{cons.SystemReserveIP}
		err = r.client.Create(ctx, ipReservation)
		return ipReservation, podIPMap, err
	} else if err != nil {
		return nil, nil, err
	}

	return ipReservation, podIPMap, nil
}

func (r *IPKeeper) IpReserve(ctx context.Context, logger logr.Logger, namespace, name string) error {
	//Do not process if there is no ip reserve flag: ip-reserve=enabled on the namespace
	podNamespace := &v1.Namespace{}
	err := r.client.Get(ctx, types.NamespacedName{
		Name: namespace,
	}, podNamespace)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get pod namespace error: %v", err.Error())
	}

	if value := podNamespace.Labels[cons.IPReserveKey]; value != cons.IPReserveValue {
		return nil
	}

	pod := &v1.Pod{}
	err = r.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, pod)
	if errors.IsNotFound(err) {
		return fmt.Errorf("get pod not found")
	}
	if err != nil {
		return fmt.Errorf("get pod error: %v", err.Error())
	}

	if !r.selector.Matches(labels.Set(pod.Labels)) {
		logger.Info("Pod", "msg", "not match selector")
		return nil
	}

	//IP reservation is required, check the IP of the Pod
	if len(pod.Status.PodIPs) == 0 {
		// Maybe the pod was deleted before it was assigned an IP address
		logger.Info("Pod", "msg", "no pod ip")
		return nil
	}

	podIPMap, patchJson := getResources(pod)

	// ip relation persistent to configmaps
	//In order to avoid update conflicts when deleting Pods in parallel( delete node or node not ready ), causing the deletion
	//to fail all the time, the Update function cannot be used here.
	err = r.client.Patch(ctx, podIPMap, client.Merge)
	if err != nil {
		return err
	}

	ipReservation := &v3.IPReservation{
		ObjectMeta: metav1.ObjectMeta{
			Name: ipReservationNsName.Name,
		},
	}
	//CRD does not support StrategicMergePatchType, we only append Pod IP, and do not cover other IPs,
	//so MergePatchType cannot be used, and JSONPatchType can only be used here.
	//The Kubernetes API server does not recursively create nested objects for JSON patch inputs, so when spec.reservedCIDRs is nil,
	//JSONPatch will fail, so add a permanent reserved IP: 1.1.1.1 in reservedCIDRs
	err = r.client.Patch(ctx, ipReservation, client.RawPatch(types.JSONPatchType, patchJson))
	if err != nil {
		return err
	}
	return nil
}

func (r *IPKeeper) initResources(ctx context.Context) error {
	podIPMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podIPMapNsName.Name,
			Namespace: podIPMapNsName.Namespace,
		},
	}
	err := r.client.Create(ctx, podIPMap)
	if errors.IsAlreadyExists(err) {
	} else if err != nil {
		return err
	}

	// create ipReservation
	ipReservation := &v3.IPReservation{
		ObjectMeta: metav1.ObjectMeta{
			Name: podIPMapNsName.Name,
		},
		Spec: v3.IPReservationSpec{
			//add a permanent system reserved IP: 1.1.1.1 in reservedCIDRs
			ReservedCIDRs: []string{cons.SystemReserveIP},
		},
	}

	err = r.client.Create(ctx, ipReservation)
	if errors.IsAlreadyExists(err) {
	} else if err != nil {
		return err
	}

	return nil
}
