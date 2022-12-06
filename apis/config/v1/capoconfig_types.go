/*
Copyright 2022 xdfdotcn
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cfg "sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CapoConfig is the Schema for the capoconfigs API
type CapoConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// ControllerManagerConfigurationSpec returns the contfigurations for controllers
	cfg.ControllerManagerConfigurationSpec `json:",inline"`
	//IP Reserve Time, default 30m
	IPReserveTime metav1.Duration `json:"ipReserveTime,omitempty"`
	//IP Reserve Max Count, default 200
	IPReserveMaxCount *int `json:"ipReserveMaxCount,omitempty"`

	//IP Release Period, default 5m
	IPReleasePeriod metav1.Duration `json:"ipReleasePeriod,omitempty"`

	// A label query over a set of resources, in this case pods.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

func init() {
	SchemeBuilder.Register(&CapoConfig{})
	//SchemeBuilder.SchemeBuilder.Register(addDefaultingFuncs)
}
