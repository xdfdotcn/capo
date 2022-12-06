package e2e

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	configv1 "github.com/xdfdotcn/capo/apis/config/v1"
	ipreservationctrl "github.com/xdfdotcn/capo/pkg/controllers/ipreservation"
	"github.com/xdfdotcn/capo/pkg/handler"
	"github.com/xdfdotcn/capo/pkg/utils"
	capowebhook "github.com/xdfdotcn/capo/pkg/webhook"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	aggregatorv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	//+kubebuilder:scaffold:imports
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	testMaxIPCount = 20
	sch            = scheme.Scheme
)

func init() {
	RegisterFailHandler(Fail)
	err := admissionv1beta1.AddToScheme(sch)
	Expect(err).NotTo(HaveOccurred())
	err = aggregatorv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = v3.AddToScheme(sch)
	Expect(err).NotTo(HaveOccurred())
}

func StartWebhookServer(cfg *rest.Config, ctx context.Context, servingCertDir string) {
	k8sClient, err := client.New(cfg, client.Options{Scheme: sch})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	localServingHost := "0.0.0.0"
	localServingPort := 9443
	// start webhook server using Manager
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             sch,
		Host:               localServingHost,
		Port:               localServingPort,
		CertDir:            servingCertDir,
		LeaderElection:     false,
		MetricsBindAddress: "0",
	})
	Expect(err).NotTo(HaveOccurred())

	ctrlConfig := &configv1.CapoConfig{
		IPReserveMaxCount: pointer.Int(testMaxIPCount),
		IPReserveTime:     metav1.Duration{Duration: 30 * time.Minute},
		IPReleasePeriod:   metav1.Duration{Duration: 5 * time.Second},
	}
	ctrl.SetLogger(utils.CreateLogger(true, true))
	keeper, err := handler.NewIPKeeper(mgr.GetClient(), ctrlConfig)
	Expect(err).NotTo(HaveOccurred())
	Expect(keeper).NotTo(BeNil())

	ipReservationReconciler := ipreservationctrl.NewIPReservationReconciler(mgr.GetClient(), ctrlConfig, keeper)
	Expect(ipReservationReconciler.SetupWithManager(mgr)).NotTo(HaveOccurred())

	podValidate := capowebhook.NewPodValidator(mgr.GetClient(), keeper)
	mgr.GetWebhookServer().Register("/pod-ip-reservation", &webhook.Admission{Handler: podValidate})
	//Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:webhook

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()

	// wait for the webhook server to get ready
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", localServingHost, localServingPort)
	Eventually(func() error {
		var conn *tls.Conn
		conn, err = tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return err
		}
		_ = conn.Close()
		return nil
	}).Should(Succeed())
}
