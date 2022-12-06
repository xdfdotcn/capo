package e2edemo

import (
	"context"
	"log"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

var (
	testenv env.Environment
)

//使用框架
//该框架直接使用内置的 Go 测试框架来定义和运行测试。
//
//设置TestMain
//使用函数TestMain定义包范围的测试步骤和配置行为。以下示例使用预定义的步骤KinD在包中运行任何测试之前创建集群：
func TestMain(m *testing.M) {
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatal(err)
		os.Exit(-1)
	}
	testenv = env.NewWithConfig(cfg)
	//kindClusterName := envconf.RandomName("my-cluster", 16)
	kindClusterName := "my-cluster-b3d07"
	namespace := envconf.RandomName("myns", 16)

	// Use pre-defined environment funcs to create a kind cluster prior to test run
	testenv.Setup(
		envfuncs.CreateKindCluster(kindClusterName),
	)

	// Use pre-defined environment funcs to teardown kind cluster after tests
	testenv.Finish(
		envfuncs.DeleteNamespace(namespace),
	)

	// launch package tests
	os.Exit(testenv.Run(m))
}

//定义测试函数
//使用 Go 测试函数定义要测试的功能，如下所示：
func TestKubernetes(t *testing.T) {
	f1 := features.New("count pod").
		WithLabel("type", "pod-count").
		Assess("pods from kube-system", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var pods corev1.PodList
			err := cfg.Client().Resources("kube-system").List(context.TODO(), &pods)
			if err != nil {
				t.Fatal(err)
			}
			if len(pods.Items) == 0 {
				t.Fatal("no pods in namespace kube-system")
			}
			return ctx
		}).Feature()

	f2 := features.New("count namespaces").
		WithLabel("type", "ns-count").
		Assess("namespace exist", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var nspaces corev1.NamespaceList
			err := cfg.Client().Resources().List(context.TODO(), &nspaces)
			if err != nil {
				t.Fatal(err)
			}
			if len(nspaces.Items) == 1 {
				t.Fatal("no other namespace")
			}
			return ctx
		}).Feature()

	// test feature
	testenv.Test(t, f1, f2)
}
