package webhook

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	configv1 "github.com/xdfdotcn/capo/apis/config/v1"
	cons "github.com/xdfdotcn/capo/pkg/constants"
	"github.com/xdfdotcn/capo/pkg/handler"
	"github.com/xdfdotcn/capo/pkg/utils"
	"github.com/xdfdotcn/capo/pkg/webhook"
	admissionv1 "k8s.io/api/admission/v1"
	authv1 "k8s.io/api/authentication/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	testPodNamespace = "test-redis-test"
	testPodName      = "test-pod-test"
	testNodeName     = "node01"
)

func init() {
	RegisterFailHandler(Fail)
}

func TestWebhook(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Webhook Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = Describe("Webhook", func() {
	var (
		podIP        = "1.2.4.5"
		ctrlConfig   *configv1.CapoConfig
		fakeClient   client.Client
		podsResource = metav1.GroupVersionResource{
			Group:    v1.SchemeGroupVersion.Group,
			Version:  v1.SchemeGroupVersion.Version,
			Resource: "pods",
		}
		validator admission.Handler
	)
	tests := []struct {
		operation          admissionv1.Operation
		requestSubResource string
		resource           metav1.GroupVersionResource
		expect             bool
	}{
		{
			operation:          admissionv1.Create,
			requestSubResource: "",
			resource:           podsResource,
			expect:             true,
		},
		{
			operation:          admissionv1.Update,
			requestSubResource: "",
			resource:           podsResource,
			expect:             true,
		},
		{
			operation:          admissionv1.Delete,
			requestSubResource: "",
			resource:           podsResource,
			expect:             true,
		},
		{
			operation:          admissionv1.Create,
			requestSubResource: cons.PodSubResourceEviction,
			resource:           podsResource,
			expect:             true,
		},
		{
			operation:          admissionv1.Update,
			requestSubResource: cons.PodSubResourceEviction,
			resource:           podsResource,
			expect:             true,
		},
		{
			operation:          admissionv1.Delete,
			requestSubResource: cons.PodSubResourceEviction,
			resource:           podsResource,
			expect:             true,
		},
	}

	testNs := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testPodNamespace,
			Labels: map[string]string{
				cons.IPReserveKey: cons.IPReserveValue,
			},
		},
	}

	testPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testPodName,
			Namespace: testPodNamespace,
			Labels: map[string]string{
				cons.LabelSelectorStatefulSetPodKey: "1",
			},
		},
		Spec: v1.PodSpec{
			NodeName: testNodeName,
		},
		Status: v1.PodStatus{
			PodIPs: []v1.PodIP{
				{
					IP: podIP,
				},
			},
			PodIP: podIP,
		},
	}

	var (
		keeper            *handler.IPKeeper
		testIPConfigMaps  = &v1.ConfigMap{}
		testIPReservation = &v3.IPReservation{}
	)

	BeforeEach(func() {
		Expect(v3.AddToScheme(scheme.Scheme)).To(Succeed())
		fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

		ctrlConfig = &configv1.CapoConfig{
			IPReserveMaxCount: pointer.Int(200),
			IPReserveTime:     metav1.Duration{Duration: 30 * time.Minute},
			IPReleasePeriod:   metav1.Duration{Duration: 5 * time.Second},
		}
		var err error
		keeper, err = handler.NewIPKeeper(fakeClient, ctrlConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(keeper).NotTo(BeNil())
		validator = webhook.NewPodValidator(fakeClient, keeper)
	})

	JustBeforeEach(func() {
		// 查询 ipreservation 应该存在
		err := fakeClient.Get(context.TODO(), types.NamespacedName{
			Name: cons.IPReservationName,
		}, testIPReservation)
		Expect(err).NotTo(HaveOccurred())
		Expect(testIPReservation.Name).To(Equal(cons.IPReservationName))

		// 查询 ipconfigmaps 应该存在
		err = fakeClient.Get(context.TODO(), types.NamespacedName{
			Name:      cons.IPReservationName,
			Namespace: cons.IPReserveKey,
		}, testIPConfigMaps)
		Expect(err).NotTo(HaveOccurred())

		// 创建 namespace
		err = fakeClient.Create(context.TODO(), testNs)
		Expect(err).NotTo(HaveOccurred())

		//查询上面创建的 namespace 应该成功
		err = fakeClient.Get(context.TODO(), types.NamespacedName{
			Name: testPodNamespace,
		}, testNs)
		Expect(err).NotTo(HaveOccurred())

		// 创建测试 Pod
		err = fakeClient.Create(context.TODO(), testPod)
		Expect(err).NotTo(HaveOccurred())

		// 查询上面创建的Pod应该存在
		err = fakeClient.Get(context.TODO(), types.NamespacedName{
			Namespace: testPodNamespace,
			Name:      testPodName,
		}, testPod)
		Expect(err).NotTo(HaveOccurred())
	})

	It("fake client test webhook, ip reserve and release", func() {
		for _, te := range tests {
			req := newAdmissionRequest(te.operation, te.requestSubResource, te.resource)
			res := validator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(Equal(te.expect))
		}

		// 查询 ipreservation
		err := fakeClient.Get(context.TODO(), types.NamespacedName{
			Name: cons.IPReservationName,
		}, testIPReservation)
		Expect(err).NotTo(HaveOccurred())
		Expect(testIPReservation.Spec.ReservedCIDRs).To(ContainElement(podIP))

		// 查询 ipconfigmaps
		err = fakeClient.Get(context.TODO(), types.NamespacedName{
			Name:      cons.IPReservationName,
			Namespace: cons.IPReserveKey,
		}, testIPConfigMaps)
		Expect(err).NotTo(HaveOccurred())
		Expect(testIPConfigMaps.Data).To(HaveKey(podIP))
		Expect(testIPConfigMaps.Data).To(ContainElement(ContainSubstring(testPodNamespace)),
			ContainElement(ContainSubstring(testPodName)),
			ContainElement(ContainSubstring(testNodeName)))

		// test ip release
		err = keeper.IpRelease(context.TODO(), utils.CreateLogger(false, true))
		Expect(err).NotTo(HaveOccurred())

		// 查询 ipreservation
		err = fakeClient.Get(context.TODO(), types.NamespacedName{
			Name: cons.IPReservationName,
		}, testIPReservation)
		Expect(err).NotTo(HaveOccurred())
		Expect(testIPReservation.Spec.ReservedCIDRs).To(HaveLen(2))
		Expect(testIPReservation.Spec.ReservedCIDRs).To(ContainElement(podIP))
		Expect(testIPReservation.Spec.ReservedCIDRs).To(ContainElement(cons.SystemReserveIP))

		// 查询 ipconfigmaps
		err = fakeClient.Get(context.TODO(), types.NamespacedName{
			Name:      cons.IPReservationName,
			Namespace: cons.IPReserveKey,
		}, testIPConfigMaps)
		Expect(err).NotTo(HaveOccurred())
		Expect(testIPConfigMaps.Data).To(HaveKey(podIP))
		Expect(testIPConfigMaps.Data).To(HaveLen(1))
	})
})

func newAdmissionRequest(operation admissionv1.Operation, requestSubResource string, resource metav1.GroupVersionResource) admission.Request {
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Kind: metav1.GroupVersionKind{
				Kind: "Pod",
			},
			Resource:           resource,
			RequestSubResource: requestSubResource,
			Namespace:          testPodNamespace,
			Name:               testPodName,
			UID:                "test-uid",
			Operation:          operation,
			UserInfo:           authv1.UserInfo{},
		},
	}
}
