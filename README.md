# Capo
**Kubernetes Calico Networking Reconciler** 🎉

[![Codecov](https://img.shields.io/codecov/c/github/xdfdotcn/capo?color=green)](https://app.codecov.io/gh/xdfdotcn/capo/branch/master)
![Build](https://img.shields.io/github/actions/workflow/status/xdfdotcn/capo/test-build.yml?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/xdfdotcn/capo)](https://goreportcard.com/report/github.com/xdfdotcn/capo)
![License](https://img.shields.io/github/license/xdfdotcn/capo?color=blue)
[![GitHub Repo stars](https://img.shields.io/github/stars/xdfdotcn/capo)](https://github.com/xdfdotcn/capo/stargazers)

![capo-logo](https://github.com/xdfdotcn/capo/blob/master/docs/img/logo_white.png)

[Capo](https://zh.wikipedia.org/zh-cn/%E5%90%89%E4%BB%96%E5%8F%98%E8%B0%83%E5%A4%B9) 是吉他变调夹，又称为移调夹（意大利：capo tasto、德意志：capotaster、西班牙：capotasto，简称：Capo），是一种横向夹紧在吉他琴弦和指板上，用于提升其音调的装置。

变调夹最初的用途是调整吉他的音调，使其与 [演唱者](https://zh.wikipedia.org/w/index.php?title=%E6%BC%94%E5%94%B1%E8%80%85&action=edit&redlink=1) 的 [嗓音](https://zh.wikipedia.org/wiki/%E5%97%93%E9%9F%B3) 相 [协调](https://zh.wikipedia.org/w/index.php?title=%E5%8D%94%E8%AA%BF&action=edit&redlink=1) 。在我们的项目中，它与 Calico 协调工作。

# 解决的问题

Kubernetes PodIP 是变化的，对于无状态服务这没有问题。对于有状态的中间件，由于有些中间件、客户端特性，必须使用 IP 连接集群，此时 IP 的变化就会有问题。

在我们的场景中，大量 Redis 集群 Pod 重建之后，如果 IP 发生交换（在集群 IP 数量紧张时，更容易发生 IP 重用），客户端有可能连接到错误的集群，如果密码错误，会一直重试，这会对客户端系统造成中断。

固定 IP 会有黑洞路由（blackhole）问题：
- Calico 会在分配 IPAMBlock 时，给 IPAMBlock（172-24-143-32-28 16个 IP ） 亲和的主机上增加 blackhole（172.24.143.32/28），当没有优先级更高的明细路由（172.24.143.33 dev calibfccc3056f0 scope link）时，会被 blackhole 丢弃掉。（保证 IP 不存在时，请求不扩展到路由器才知道目标不可达）
- 如果 Pod 重建漂移到其他主机上（B 主机）时，如果要使用原来的 IP，则需要借用原主机（A 主机）上 IPAMBlock 中的 IP，导致旧主机（A 主机）上的 Pod 访问新主机（B 主机）上 PodIP 时不可达。（被旧主机 A 上的 blackhole 给丢掉）。

Capo 项目功能是在 Pod 重建后使用其他 IP，原来的 IP 延迟释放。价值：
- 原集群 PodIP 被其他集群占用后（发生 IP 交换），集群拓扑变的不符预期（Gossip 协议机制），降低 Operator 修复集群的难度
- PodIP 被其他集群占用，导致客户端连接到错误的集群

# 设计

IP 保留：删除 Pod 或者 驱逐 Pod 时，先走 Capo webhook。此时 Pod 还没有到 cmdDel （释放 IP）阶段，首先进入 IP 保留逻辑，将这个 PodIP 放入到 IPReservation 对象中，之后经过 kube-apiserver 走删除逻辑，kubelet 调用 calico CNI 执行 cmdDel 逻辑，将 PodIP 释放掉。Calico 通过 IPReservation 可以预留 IPPool 的指定 IP 不被自动分配给容器。由于 IP 在 IPReservation 对象中，所以不会被新建的其他 Pod 占用。

IP 释放：目前是通过配置指定一段时间后释放，或者保留数量达到指定阈值时释放。（具体下文 Helm 参数配置中会提到）

# 架构

![capo-architecture](https://github.com/xdfdotcn/capo/blob/master/docs/img/capo-architecture.png)

# 前置条件

calico 3.21.6 使用 k8s backend。
Capo 需要调用 projectcalico.org/v3 接口，依赖于 calico-apiserver：v3.21.6
部署请参考：[https://projectcalico.docs.tigera.io/archive/v3.21/maintenance/install-apiserver](https://projectcalico.docs.tigera.io/archive/v3.21/maintenance/install-apiserver)

```shell
kubectl apply -f https://docs.projectcalico.org/archive/v3.21/manifests/apiserver.yaml
```

Capo webhook 证书依赖 cert-manager：v1.2.0 签发：

```shell
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.2.0/cert-manager.yaml
```

[部署 cert-manager 时，请注意和 Kubernetes 版本的 兼容性。](https://cert-manager.io/docs/installation/supported-releases/)

# 部署

目前提供三种部署方式，推荐使用 helm 部署方式。

## Helm

请参考 [capo-helm-chart 页面](https://github.com/xdfdotcn/capo/blob/master/deploy/capo/README.md#values) ，设置 values.yaml 可配置参数。

> 这个是必要的，根据集群节点上 maxPods 设置 ipReserveMaxCount，推荐设置为 `1.2 * maxPods`，保证一个 node 故障后节点上的 Pod IP 保留。

### 配置

在 [capo-helm-chart 页面](https://github.com/xdfdotcn/capo/blob/master/deploy/capo/README.md) ，找到设置 values.yaml 可配置参数。
例如配置 values.yaml 中主要内容如下：

```yaml

# -- Number of instances, high availability configuration Please set it to 3
replicaCount: 3

# -- Set capo config
config:
  # -- enable leaderElect
  leaderElectionEnable: true
  # -- health probe bind address
  healthProbeBindAddress: ":8081"
  # -- webhook port
  webhookPort: 9443
  # -- metrics bind address
  metricsBindAddress: ":8080"
  # -- ip reserve max count
  ipReserveMaxCount: 300
  # -- ip reserve max time
  ipReserveTime: 40m
  # -- ip release period
  ipReleasePeriod: 5s
```

- replicaCount：默认为 3，即启动 3 个实例
- config.leaderElectionEnable：默认为 true，启动 leader 选举，所以请设置 replicaCount 大于 1 是必要的
- config.healthProbeBindAddress：健康检查端口
- config.webhookPort：webhook 端口，用于和 kube-APIServer 通信
- config.metricsBindAddress：metrics 端口，用户 prometheus 抓取监控指标数据 
- config.ipReserveMaxCount：最大保留 IP 数量，推荐设置为 1.2 * maxPods，保证一个 node 故障后节点上的 Pod IP 保留。IP 数量到达最大值时，将开始释放最早的 IP
- config.ipReserveTime：IP 保留最长时间，时间达到设置的值时，将开始释放 IP，默认为 40m
- config.ipReleasePeriod：IP 释放间隔，默认值为 5s，保持默认即可

### 安装

使用 `helm repo` 客户端命令管理存储库：

```shell
# helm repo add  xdfgithubrepo  https://xdfdotcn.github.io/capo
"xdfgithubrepo" has been added to your repositories
```

搜索 Helm 存储库中可用的 Chart 包：

```shell
# helm search repo xdfgithubrepo
NAME                    CHART VERSION   APP VERSION     DESCRIPTION
xdfgithubrepo/capo      0.1.0           1.0.1           A Helm chart for Kubernetes Calico CNI IP Reserve
```

使用下面命令，执行安装。也可以用 --set flag 覆盖默认的参数，或者使用 -f 参数指定修改后的 values.yaml 安装：

```shell
# helm install capo -n ip-reserve xdfgithubrepo/capo --create-namespace
NAME: capo
LAST DEPLOYED: Wed Nov 23 09:32:32 2022
NAMESPACE: ip-reserve
STATUS: deployed
REVISION: 1
NOTES:
1. Get the application URL by running these commands:
kubectl get pods -n ip-reserve -o wide | grep capo

2. Chart version: 0.1.0
Capo version: v2

Thank you for installing capo! Your release is named capo.
```

查看状态：

```shell
$ kubectl get pods -n ip-reserve -o wide | grep capo
capo-78b6899d4d-dxwfh   2/2     Running   0          10s   10.12.1.2   master01   <none>           <none>
```

### 卸载

```shell
$ helm uninstall capo -n ip-reserve
release "capo" uninstalled
```

## Kustomize

进入项目根目录下的 config/default 路径，默认安装到 ip-reserve namespace 下：

### 安装

```shell
$ cd config/default
$ kustomize build | kubectl apply -f -
namespace/ip-reserve created
serviceaccount/ip-reserve-controller-manager created
role.rbac.authorization.k8s.io/ip-reserve-leader-election-role created
clusterrole.rbac.authorization.k8s.io/ip-reserve-manager-role created
clusterrole.rbac.authorization.k8s.io/ip-reserve-metrics-reader created
clusterrole.rbac.authorization.k8s.io/ip-reserve-proxy-role created
rolebinding.rbac.authorization.k8s.io/ip-reserve-leader-election-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/ip-reserve-manager-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/ip-reserve-proxy-rolebinding created
configmap/ip-reserve-manager-config created
service/ip-reserve-controller-manager-metrics-service created
service/ip-reserve-webhook-service created
deployment.apps/ip-reserve-controller-manager created
certificate.cert-manager.io/ip-reserve-serving-cert created
issuer.cert-manager.io/ip-reserve-selfsigned-issuer created
prometheusrule.monitoring.coreos.com/ip-reserve-prometheus-ip-reserve-rules created
servicemonitor.monitoring.coreos.com/ip-reserve-controller-manager-metrics-monitor created
validatingwebhookconfiguration.admissionregistration.k8s.io/ip-reserve-validating-webhook-configuration created
```

### 卸载

```shell
$ cd config/default
$ kustomize build | kubectl delete -f -
namespace "ip-reserve" deleted
serviceaccount "ip-reserve-controller-manager" deleted
role.rbac.authorization.k8s.io "ip-reserve-leader-election-role" deleted
clusterrole.rbac.authorization.k8s.io "ip-reserve-manager-role" deleted
clusterrole.rbac.authorization.k8s.io "ip-reserve-metrics-reader" deleted
clusterrole.rbac.authorization.k8s.io "ip-reserve-proxy-role" deleted
rolebinding.rbac.authorization.k8s.io "ip-reserve-leader-election-rolebinding" deleted
clusterrolebinding.rbac.authorization.k8s.io "ip-reserve-manager-rolebinding" deleted
clusterrolebinding.rbac.authorization.k8s.io "ip-reserve-proxy-rolebinding" deleted
configmap "ip-reserve-manager-config" deleted
service "ip-reserve-controller-manager-metrics-service" deleted
service "ip-reserve-webhook-service" deleted
deployment.apps "ip-reserve-controller-manager" deleted
certificate.cert-manager.io "ip-reserve-serving-cert" deleted
issuer.cert-manager.io "ip-reserve-selfsigned-issuer" deleted
prometheusrule.monitoring.coreos.com "ip-reserve-prometheus-ip-reserve-rules" deleted
servicemonitor.monitoring.coreos.com "ip-reserve-controller-manager-metrics-monitor" deleted
validatingwebhookconfiguration.admissionregistration.k8s.io "ip-reserve-validating-webhook-configuration" deleted
```

## Plain yaml

进入项目根目录下的 deploy/yaml 路径，默认安装到 ip-reserve namespace 下：

### 安装

```shell
cd deploy/yaml
kubectl apply -f install.yaml
```

### 卸载

```shell
kubectl delete -f install.yaml
```

# 使用

通过 namespace 增加 label：ip-reserve=enabled
以及安装在 capo 命令空间下的 configMaps 中 labelSelector 选择 Pod 的方式，选中需要启用 IP 保留的 Pod。

## 示例

如果要实现选中 zookeeper-dev 命名空间下：

- label 中包含 statefulset.kubernetes.io/pod-name 的 Pod （即 StatefulSet 创建出的 Pod）
- 或者 label 中包含 brokerId 的 Pod

只需给 zookeeper-dev 增加 label：

```yaml
kubectl label ns zookeeper-dev ip-reserve=enable
```

以及 ip-reserve 命名空间下的 capo-manager-config ConfigMaps 中 labelSelector 配置如下：

```yaml
$ k get cm -n ip-reserve capo-manager-config  -o yaml
apiVersion: v1
data:
  capo_config.yaml: |-
    apiVersion: config.capo.io/v1
    kind: CapoConfig
    health:
      healthProbeBindAddress: ":8081"
    metrics:
      bindAddress: ":8080"
    webhook:
      port: 9443
    leaderElection:
      leaderElect: true
      resourceName: capo
      resourceNamespace: ip-reserve
    ipReserveMaxCount: 400
    ipReserveTime: 40m
    ipReleasePeriod: 5s
    labelSelector:
      matchExpressions:
        - key: statefulset.kubernetes.io/pod-name
          operator: Exists
        - key: brokerId
          operator: Exists
kind: ConfigMap
metadata:
  annotations:
    meta.helm.sh/release-name: capo
    meta.helm.sh/release-namespace: ip-reserve
  creationTimestamp: "2022-11-22T14:40:19Z"
  labels:
    app.kubernetes.io/managed-by: Helm
  name: capo-manager-config
  namespace: ip-reserve
```

## 可观测

部署时会部署 ServiceMonitor 作为 Prometheus 监控 Target 和 PrometheusRule 配置告警规则；
还提供一个简单的 Grafana 面板展示 IP 保留和释放可观测。
[https://github.com/xdfdotcn/capo/blob/master/deploy/grafana/dashboard.json](https://github.com/xdfdotcn/capo/blob/master/deploy/grafana/dashboard.json)

# 发展规划

- [X] Delete Node 场景下 gc_controller 强制 Delete Pod 时，IP 保留
- [X] 调用驱逐 API pods/eviction （CREATE 请求）时，IP 保留
- [X] Node NotReady 场景下 node_life_cycle controller Delete Pod时，IP 保留
- [X] 抢占 Preemption and Eviction 场景，IP 保留
- [X] 通过监控，查看 IP 保留、释放、驱逐情况
- [X] e2e 测试用例编写
- [X] GitHub Action 触发提交代码时执行静态代码检查、单元测试、e2e测试
- [ ] 目前配置变更，capo 重启后才生效，有待支持热更新
- [ ] 目前支持 Calico 使用 Kubernetes 作为 backend，有待支持 Etcd backend
- [ ] 删除集群，不需要保留 IP
- [ ] 可以给特定的 deploy、statefulset、pod 打 label 或 annotation 的方式做到 IP 保留
- [ ] Calico、Cilium 网络 固定 IP
- [ ] 主机资源不足 kubelet 做驱逐，由于 kubelet 不会发送 Delete 请求，而是直接 Kill 容器，IP 目前没法保留

# 开发贡献

// TODO
