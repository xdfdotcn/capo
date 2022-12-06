/*
Copyright 2022 xdfdotcn
*/
package webhook

import (
	"context"

	cons "github.com/xdfdotcn/capo/pkg/constants"
	"github.com/xdfdotcn/capo/pkg/handler"
	v1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:verbs=delete,path=/pod-ip-reservation,mutating=false,failurePolicy=fail,groups=core,resources=pods,versions=v1,name=pod.ip.io,admissionReviewVersions=v1,sideEffects=none

// podValidator validates Pods
type podValidator struct {
	client client.Client
	keeper *handler.IPKeeper
}

func NewPodValidator(c client.Client, keeper *handler.IPKeeper) admission.Handler {
	return &podValidator{
		client: c,
		keeper: keeper,
	}
}

// podValidator admits a pod if a specific annotation exists.
func (r *podValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("reqKind", req.Kind,
		"reqNamespace", req.Namespace,
		"reqName", req.Name,
		"reqOperation", req.Operation,
		"reqResource", req.Resource,
		"reqSubResource", req.RequestSubResource)
	logger.Info("Request detail")
	if v1.Delete != req.Operation && cons.PodSubResourceEviction != req.RequestSubResource {
		return admission.Allowed("")
	}

	err := r.keeper.IpReserve(ctx, logger, req.Namespace, req.Name)
	if err != nil {
		logger.Error(err, "denied")
		return admission.Denied(err.Error())
	}

	return admission.Allowed("")
}

// podValidator implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
/*func (r *podValidator) InjectDecoder(d *admission.Decoder) error {
	r.decoder = d
	return nil
}*/
