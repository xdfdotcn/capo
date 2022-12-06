/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	v3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	cons "github.com/xdfdotcn/capo/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	e2ewait "sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	policy "k8s.io/api/policy/v1"
	policyv1 "k8s.io/client-go/kubernetes/typed/policy/v1"
)

var (
	testReserveNs                = "redis-jkld"
	testReserveStsName           = "pause-sts"
	testDisableReserveNs         = "disabled-ns-jkld"
	testDisableReserveBrokerName = "pause-dep"
	//kindNetNodeLabelKey          = "kindnet"
	//kindNetNodeLabelValue        = "true"
)

func TestCapo(t *testing.T) {
	testIPCount := 10
	testStsPodOnEnableNsIPReserveFeature := features.New("test Statefulet Pods On Enable Namespace IP Reserve Feature").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			waitStatefulSetPodsReady(t, cfg)
			cleanCRAndConfigMaps(ctx, t, cfg)
			deleteStsPods(ctx, t, cfg, 0, testIPCount, false)
			return ctx
		}).Assess("test ip reserve cr and  configmaps ip data", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
		assertIPCount(t, config, testIPCount)
		return ctx
	}).Feature()

	testBrokerPodOnDisabledIPReserveFeature := features.New("test Broker Pod On Disabled IP Reserve Feature").
		Assess("should not reserve ip", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			cleanCRAndConfigMaps(ctx, t, cfg)
			notExpectIPs := testDisableNsPods(ctx, t, cfg)
			// should not contain podIP
			assertShouldNotIncludeIPs(ctx, t, cfg, notExpectIPs)

			return ctx
		}).Feature()

	testDeletePodForceIPReserveFeature := features.New("test Force Delete Pod IPReserve Feature").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			waitStatefulSetPodsReady(t, cfg)
			cleanCRAndConfigMaps(ctx, t, cfg)
			deleteStsPods(ctx, t, cfg, 0, testIPCount, true)
			return ctx
		}).Assess("test ip reserve cr and  configmaps ip data", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
		assertIPCount(t, config, testIPCount)
		return ctx
	}).Feature()

	testIPReleaseFeature := features.New("test reserve ip reach max count and start release ip").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			waitStatefulSetPodsReady(t, cfg)
			cleanCRAndConfigMaps(ctx, t, cfg)
			deleteStsPods(ctx, t, cfg, 0, testIPCount, false)
			waitStatefulSetPodsReady(t, cfg)
			deleteStsPods(ctx, t, cfg, 5, testIPCount, false)
			waitStatefulSetPodsReady(t, cfg)
			deleteStsPods(ctx, t, cfg, 6, testIPCount, false)

			assertIPRserveShouldNotInclude(t, cfg)
			assertIPCount(t, cfg, testMaxIPCount)
			return ctx
		}).Feature()

	testDrainPodWorkerNodeFeature := features.New("test drain pod on one node").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			waitStatefulSetPodsReady(t, cfg)
			cleanCRAndConfigMaps(ctx, t, cfg)
			drainTestPodsOfWorkerNode(ctx, t, cfg)
			assertIPCount(t, cfg, testMaxIPCount)
			return ctx
		}).Feature()

	// The current way of labeling nodes and adding nodeSelector to kindnet daemonSets cannot simulate node notReady
	/*	testNodeNotReadyFeature := features.New("test node not ready").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		waitStatefulSetPodsReady(t, cfg)
		cleanCRAndConfigMaps(ctx, t, cfg)
			labelNodeForKindNet(ctx, t, cfg)
			kindNetPodNodeSelectorControl(ctx, t, cfg)
			waitAllKindNetPodReady(t, cfg)
			assertIPRserveMaxCount(ctx, t, cfg)
			return ctx
		}).Feature()*/

	testDeleteWorkerNodeFeature := features.New("test delete worker node case").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			waitStatefulSetPodsReady(t, cfg)
			cleanCRAndConfigMaps(ctx, t, cfg)
			allPods := listAllPodInWorkNode(ctx, t, cfg)
			deleteWorkerNode(ctx, t, cfg)
			waitAllPodsDisappearAndRecreate(t, cfg, allPods)
			assertIPCount(t, cfg, testMaxIPCount)
			return ctx
		}).Feature()

	testenv.Test(t, testStsPodOnEnableNsIPReserveFeature,
		testBrokerPodOnDisabledIPReserveFeature,
		testDeletePodForceIPReserveFeature,
		testIPReleaseFeature,
		testDrainPodWorkerNodeFeature,
		//testNodeNotReadyFeature,
		testDeleteWorkerNodeFeature)
}

func deleteWorkerNode(ctx context.Context, t *testing.T, cfg *envconf.Config) {
	deleteNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-worker", kindClusterName),
		},
	}
	err := cfg.Client().Resources().Delete(ctx, deleteNode)
	if err != nil {
		t.Fatal(err)
	}
}

func waitAllPodsDisappearAndRecreate(t *testing.T, cfg *envconf.Config, pods *v1.PodList) {
	err := e2ewait.For(conditions.New(cfg.Client().Resources()).ResourcesMatch(pods, func(object k8s.Object) bool {
		pod := object.(*v1.Pod)
		if !strings.Contains(pod.Name, "pause-sts") {
			return true
		}
		return pod.Status.Phase == v1.PodPending
	}), e2ewait.WithTimeout(time.Minute*10))
	if err != nil {
		t.Logf("wait all pods disappear and recreate timeout")
		t.Fatal(err)
	}
}

func listAllPodInWorkNode(ctx context.Context, t *testing.T, cfg *envconf.Config) *v1.PodList {
	listOption := func(op *metav1.ListOptions) {
		op.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", fmt.Sprintf("%s-worker", kindClusterName)).String()
	}

	allPods := v1.PodList{}
	err := cfg.Client().Resources().List(ctx, &allPods, listOption)
	if err != nil {
		t.Fatal(err)
	}
	return &allPods
}

func drainTestPodsOfWorkerNode(ctx context.Context, t *testing.T, cfg *envconf.Config) {
	policyClient, err := policyv1.NewForConfig(cfg.Client().RESTConfig())
	if err != nil {
		t.Fatal(err)
	}

	allPods := listAllPodInWorkNode(ctx, t, cfg)

	for _, pod := range allPods.Items {
		if !strings.Contains(pod.Name, "pause-sts") {
			continue
		}

		t.Logf("drain pod: %s/%s, so create Eviction object", pod.Namespace, pod.Name)
		err = policyClient.Evictions(testReserveNs).Evict(ctx, &policy.Eviction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pod.Name,
				Namespace: pod.Namespace,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func assertIPCount(t *testing.T, cfg *envconf.Config, testIPCount int) {
	ipr := &v3.IPReservation{
		ObjectMeta: metav1.ObjectMeta{Name: cons.IPReservationName},
	}
	err := e2ewait.For(conditions.New(cfg.Client().Resources()).ResourceMatch(ipr, func(object k8s.Object) bool {
		ip := object.(*v3.IPReservation)
		t.Logf("cr len is %d, ips: %v\n ", len(ip.Spec.ReservedCIDRs), ip.Spec.ReservedCIDRs)
		// exclude systemIP: 1.1.1.1
		return len(ip.Spec.ReservedCIDRs)-1 == testIPCount
	}), e2ewait.WithTimeout(time.Second*40))
	if err != nil {
		t.Logf("wait for the ip reserve cr ip count to: %d error", testIPCount)
		t.Fatal(err)
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: cons.IPReservationName, Namespace: cons.IPReserveKey},
	}
	err = e2ewait.For(conditions.New(cfg.Client().Resources()).ResourceMatch(cm, func(object k8s.Object) bool {
		c := object.(*v1.ConfigMap)
		return len(c.Data) == testIPCount
	}), e2ewait.WithTimeout(time.Second*10))
	if err != nil {
		t.Logf("wait for the configmaps ip count to: %d error", testIPCount)
		t.Fatal(err)
	}

	// ipReserve ip should be included in the config map
	for _, ip := range ipr.Spec.ReservedCIDRs {
		if ip == cons.SystemReserveIP {
			continue
		}
		if _, ok := cm.Data[ip]; !ok {
			t.Fatalf("ipReserve ip should be included in the config map, cr IP: %v \n configmaps: %v\n", ipr.Spec.ReservedCIDRs, cm.Data)
		}
	}
}

func assertShouldNotIncludeIPs(ctx context.Context, t *testing.T, cfg *envconf.Config, notExpectIPs []string) {
	ipr := &v3.IPReservation{
		ObjectMeta: metav1.ObjectMeta{Name: cons.IPReservationName},
	}
	if err := cfg.Client().Resources().Get(ctx, cons.IPReservationName, "", ipr); err != nil {
		t.Fatal(err)
	}
	for _, notExpect := range notExpectIPs {
		for _, ip := range ipr.Spec.ReservedCIDRs {
			if notExpect == ip {
				t.Fatal("ip reserve should not include disable namespace pod ip")
			}
		}
	}

	cm := &v1.ConfigMap{}
	if err := cfg.Client().Resources().Get(ctx, cons.IPReservationName, cons.IPReserveKey, cm); err != nil {
		t.Fatal(err)
	}

	for _, ip := range notExpectIPs {
		if _, ok := cm.Data[ip]; ok {
			t.Fatal("configmaps should not include disable namespace pod ip")
		}
	}
}

func waitStatefulSetPodsReady(t *testing.T, cfg *envconf.Config) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: testReserveStsName, Namespace: testReserveNs},
	}
	// wait for the statefulSet to finish becoming available
	err := e2ewait.For(conditions.New(cfg.Client().Resources()).ResourceMatch(sts, func(object k8s.Object) bool {
		s := object.(*appsv1.StatefulSet)
		return s.Status.ReadyReplicas == *(s.Spec.Replicas)
	}), e2ewait.WithTimeout(time.Minute*10))
	if err != nil {
		t.Logf("wait for the statefulSet to finish becoming available error")
		t.Fatal(err)
	}
}

func deleteStsPods(ctx context.Context, t *testing.T, cfg *envconf.Config, startOrdinal, testIPCount int, force bool) {
	deleteOption := func(option *metav1.DeleteOptions) {}
	if force {
		deleteOption = func(option *metav1.DeleteOptions) {
			option.GracePeriodSeconds = pointer.Int64(0)
		}
	}

	endOrdinal := testIPCount + startOrdinal
	for {
		if startOrdinal >= endOrdinal {
			break
		}
		deletePodName := fmt.Sprintf("pause-sts-%d", startOrdinal)
		deletePod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deletePodName,
				Namespace: testReserveNs,
			},
		}
		t.Logf("delete pod: %s", deletePodName)
		if err := cfg.Client().Resources().Delete(ctx, deletePod, deleteOption); err != nil {
			t.Logf("delete pod: %s err: %v", deletePodName, err)
		}
		time.Sleep(1 * time.Second)
		startOrdinal++
	}
}

func cleanCRAndConfigMaps(ctx context.Context, t *testing.T, cfg *envconf.Config) {
	clean := func() error {
		cm := &v1.ConfigMap{}
		err := cfg.Client().Resources().Get(ctx, cons.IPReservationName, cons.IPReserveKey, cm)
		if err != nil {
			t.Logf("clean ip reserver get configmaps err: %v", err)
			return err
		}

		if len(cm.Data) != 0 {
			cm.Data = nil
			err = cfg.Client().Resources().Update(ctx, cm)
			if err != nil {
				t.Logf("clean ip reserver update configmaps err: %v", err)
				return err
			}
		}

		ipr := &v3.IPReservation{}
		err = cfg.Client().Resources().Get(ctx, cons.IPReservationName, "", ipr)
		if err != nil {
			t.Logf("clean ip reserver get cr err: %v", err)
			return err
		}

		ipr.Spec.ReservedCIDRs = []string{cons.SystemReserveIP}
		err = cfg.Client().Resources().Update(ctx, ipr)
		if err != nil {
			t.Logf("clean ip reserver update cr err: %v", err)
			return err
		}

		return nil
	}

	retryCount := 10
	for {
		err := clean()
		if err == nil {
			return
		}
		retryCount--
		if retryCount == 0 {
			t.Fatal(err)
		}
		time.Sleep(1 * time.Second)
	}
}

func testDisableNsPods(ctx context.Context, t *testing.T, cfg *envconf.Config) []string {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: testDisableReserveBrokerName, Namespace: testDisableReserveNs},
	}
	// wait for the deployment to finish becoming available
	err := e2ewait.For(conditions.New(cfg.Client().Resources()).ResourceMatch(dep, func(object k8s.Object) bool {
		d := object.(*appsv1.Deployment)
		return d.Status.ReadyReplicas == *(d.Spec.Replicas)
	}), e2ewait.WithTimeout(time.Minute*10))
	if err != nil {
		t.Logf("wait for the deployment to finish becoming available error")
		t.Fatal(err)
	}

	// list all broker pods
	pods := v1.PodList{}
	err = cfg.Client().Resources().List(context.TODO(), &pods, resources.WithLabelSelector(labels.FormatLabels(map[string]string{"app": "pause-dep"})))
	if err != nil {
		t.Fatal(err)
	}

	var notExpectIPs []string
	for _, pod := range pods.Items {
		t.Logf("delete pod: %s", pod.Name)
		if err = cfg.Client().Resources().Delete(ctx, &pod); err != nil {
			t.Logf("delete pod: %s err: %v", pod.Name, err)
			continue
		}
		for _, ip := range pod.Status.PodIPs {
			notExpectIPs = append(notExpectIPs, ip.IP)
		}
		time.Sleep(1 * time.Second)
	}

	return notExpectIPs
}

func assertIPRserveShouldNotInclude(t *testing.T, cfg *envconf.Config) {
	shouldNotIncludePodName := "pause-sts-0"
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: cons.IPReservationName, Namespace: cons.IPReserveKey},
	}
	err := e2ewait.For(conditions.New(cfg.Client().Resources()).ResourceMatch(cm, func(object k8s.Object) bool {
		c := object.(*v1.ConfigMap)
		for _, ipInfo := range c.Data {
			if strings.Contains(ipInfo, shouldNotIncludePodName) {
				t.Fatalf("ip release error, should release ip of podName: %s which reserve earliest", shouldNotIncludePodName)
			}
		}

		return true
	}), e2ewait.WithTimeout(time.Second*10))
	if err != nil {
		t.Fatal(err)
	}
}

//
//func labelNodeForKindNet(ctx context.Context, t *testing.T, cfg *envconf.Config) {
//	/*nodes := &v1.NodeList{}
//	err := cfg.Client().Resources().List(ctx, nodes, resources.WithLabelSelector(labels.FormatLabels(map[string]string{"kubernetes.io/os": "linux"})))
//	if err != nil {
//		t.Fatal(err)
//	}*/
//
//	nodes := []v1.Node{
//		{
//			ObjectMeta: metav1.ObjectMeta{
//				Name: fmt.Sprintf("%s-control-plane", kindClusterName),
//			},
//		},
//		{
//			ObjectMeta: metav1.ObjectMeta{
//				Name: fmt.Sprintf("%s-worker", kindClusterName),
//			},
//		},
//	}
//
//	mergePatch, _ := json.Marshal(map[string]interface{}{
//		"metadata": map[string]interface{}{
//			"labels": map[string]string{
//				kindNetNodeLabelKey: kindNetNodeLabelValue,
//			},
//		},
//	})
//
//	for _, node := range nodes {
//		pt := k8s.Patch{PatchType: types.StrategicMergePatchType, Data: mergePatch}
//		if err = cfg.Client().Resources().Patch(ctx, &node, pt); err != nil {
//			t.Fatal(err)
//		}
//	}
//}
//
//func kindNetPodNodeSelectorControl(ctx context.Context, t *testing.T, cfg *envconf.Config) {
//	ds := &appsv1.DaemonSet{}
//	err := cfg.Client().Resources().Get(ctx, "kindnet", "kube-system", ds)
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	ds.Spec.Template.Spec.NodeSelector = map[string]string{
//		kindNetNodeLabelKey: kindNetNodeLabelValue,
//	}
//
//	retryCount := 10
//	for {
//		err = cfg.Client().Resources().Update(ctx, ds)
//		if err == nil {
//			return
//		}
//		retryCount--
//		if retryCount == 0 {
//			t.Fatalf("update kindnet daemonset nodeSelector error: %v", err)
//		}
//		time.Sleep(1 * time.Second)
//	}
//}
//
//func waitAllKindNetPodReady(t *testing.T, cfg *envconf.Config) {
//	ds := &appsv1.DaemonSet{
//		ObjectMeta: metav1.ObjectMeta{Name: "kindnet", Namespace: "kube-system"},
//	}
//	// wait for the daemonSet to finish becoming available
//	err := e2ewait.For(conditions.New(cfg.Client().Resources()).ResourceMatch(ds, func(object k8s.Object) bool {
//		s := object.(*appsv1.DaemonSet)
//		return s.Status.NumberUnavailable == 0
//	}), e2ewait.WithTimeout(time.Minute*1))
//	if err != nil {
//		t.Logf("wait for the statefulSet to finish becoming available error")
//		t.Fatal(err)
//	}
//}
