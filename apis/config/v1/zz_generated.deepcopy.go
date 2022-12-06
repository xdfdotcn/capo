//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright 2022 xdfdotcn
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CapoConfig) DeepCopyInto(out *CapoConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.ControllerManagerConfigurationSpec.DeepCopyInto(&out.ControllerManagerConfigurationSpec)
	out.IPReserveTime = in.IPReserveTime
	if in.IPReserveMaxCount != nil {
		in, out := &in.IPReserveMaxCount, &out.IPReserveMaxCount
		*out = new(int)
		**out = **in
	}
	out.IPReleasePeriod = in.IPReleasePeriod
	if in.LabelSelector != nil {
		in, out := &in.LabelSelector, &out.LabelSelector
		*out = new(metav1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CapoConfig.
func (in *CapoConfig) DeepCopy() *CapoConfig {
	if in == nil {
		return nil
	}
	out := new(CapoConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CapoConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}