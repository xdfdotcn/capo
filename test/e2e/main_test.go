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
	"flag"
	"os"
	"testing"

	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
)

var testenv env.Environment

var (
	kindClusterName = "my-cluster-b3d07"
)

func TestMain(m *testing.M) {
	klog.InitFlags(flag.CommandLine)
	defer klog.Flush()

	klog.Infof("start capo e2e test case, setup kind cluster...")

	testenv = env.New()
	//kindClusterName := envconf.RandomName("kind-with-config", 16)
	testenv.Setup(
		envfuncs.CreateKindClusterWithConfig(kindClusterName, "kindest/node:v1.18.20", "kind-config.yaml"),
		envfuncs.SetupCRDs("../../config/test/crd/calico", "*"),
		capoAllSetup(),
	)

	testenv.Finish(
		envfuncs.DestroyKindCluster(kindClusterName),
	)
	os.Exit(testenv.Run(m))
}
