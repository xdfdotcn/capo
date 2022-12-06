package e2e

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"time"

	log "k8s.io/klog/v2"

	cons "github.com/xdfdotcn/capo/pkg/constants"
	"github.com/xdfdotcn/capo/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	aggregatorv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	e2ewait "sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"

	v1 "k8s.io/api/core/v1"
)

var (
	//secretName        = "calico-apiserver-certs"
	calicoApiServerNs = "calico-apiserver"
	apiServerKey      = "apiserver.crt"
	apiServiceName    = "v3.projectcalico.org"
	webhookCertDir    = "../../config/test/cert"
)

func capoAllSetup() env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		log.Infof("kind cluster setup success, will setup capo all relative resources")
		calicoApiServerSetup(ctx, cfg)
		capoWebhookResourceSetup(ctx, cfg)
		capoWebhookSetup(ctx, cfg)
		testPodSetup(ctx, cfg)
		return ctx, nil
	}
}

func calicoApiServerSetup(ctx context.Context, cfg *envconf.Config) {
	// read a secret
	content, err := ioutil.ReadFile("./calico-apiserver-certs.yaml")
	if err != nil {
		log.Fatal(err)
	}

	certSecret := &v1.Secret{}
	err = yaml.Unmarshal(content, certSecret)
	if err != nil {
		log.Fatal(err)
	}

	// create a secret
	err = cfg.Client().Resources(calicoApiServerNs).Create(ctx, certSecret)
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			log.Fatal(err)
		}
	}

	// kubectl patch apiservice v3.projectcalico.org -p \\n    "{\"spec\": {\"caBundle\": \"$(kubectl get secret -n calico-apiserver calico-apiserver-certs -o go-template='{{ index .data "apiserver.crt" }}')\"}}"
	apiService := aggregatorv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: apiServiceName,
		},
	}
	mergePatch, _ := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"caBundle": certSecret.Data[apiServerKey],
		},
	})
	pt := k8s.Patch{PatchType: types.StrategicMergePatchType, Data: mergePatch}
	if err = cfg.Client().Resources().Patch(ctx, &apiService, pt); err != nil {
		log.Fatal(err)
	}

	// wait calico-apiserver pod ready
	dep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: calicoApiServerNs, Namespace: calicoApiServerNs},
	}
	// wait for the deployment to finish becoming available
	err = e2ewait.For(conditions.New(cfg.Client().Resources()).DeploymentConditionMatch(&dep, appsv1.DeploymentAvailable, v1.ConditionTrue), e2ewait.WithTimeout(time.Minute*10))
	if err != nil {
		log.Fatal(err)
	}
}

func capoWebhookResourceSetup(ctx context.Context, cfg *envconf.Config) {
	if _, err := envfuncs.SetupCRDs("./", "capo-resource.yaml")(ctx, cfg); err != nil {
		log.Fatal(err)
	}

	ep := &v1.Endpoints{}
	// update endpoint ip
	if err := cfg.Client().Resources().Get(ctx, "capo-webhook-service", cons.IPReserveKey, ep); err != nil {
		log.Fatal(err)
	}

	ep.Subsets[0].Addresses[0].IP = utils.GetOutboundIP().To4().String()
	if err := cfg.Client().Resources().Update(ctx, ep); err != nil {
		log.Fatal(err)
	}
}

func testPodSetup(ctx context.Context, cfg *envconf.Config) {
	if _, err := envfuncs.SetupCRDs("./", "test-case.yaml")(ctx, cfg); err != nil {
		log.Fatal(err)
	}
}

func capoWebhookSetup(ctx context.Context, cfg *envconf.Config) {
	log.Infof("start capo webhook server")
	StartWebhookServer(cfg.Client().RESTConfig(), ctx, webhookCertDir)
	log.Infof("capo webhook server started")
}
