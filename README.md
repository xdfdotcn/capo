# capo helm chart

![](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square#crop=0&crop=0&crop=1&crop=1&id=PWE20&originHeight=20&originWidth=90&originalType=binary&ratio=1&rotation=0&showTitle=false&status=done&style=none&title=) ![](https://img.shields.io/badge/Type-application-informational?style=flat-square#crop=0&crop=0&crop=1&crop=1&id=VMmc6&originHeight=20&originWidth=106&originalType=binary&ratio=1&rotation=0&showTitle=false&status=done&style=none&title=) ![](https://img.shields.io/badge/AppVersion-1.0.1-informational?style=flat-square#crop=0&crop=0&crop=1&crop=1&id=eZKBV&originHeight=20&originWidth=112&originalType=binary&ratio=1&rotation=0&showTitle=false&status=done&style=none&title=)

A Helm chart for Kubernetes Calico CNI IP Reserve.
## Maintainers
| Name | Email | Url |
| --- | --- | --- |
| xdfdotcn |  | [https://github.com/xdfdotcn](https://github.com/xdfdotcn) |


## Source Code

- [https://github.com/xdfdotcn/capo](https://github.com/xdfdotcn/capo)

## Requirements

Kubernetes: `>=1.18.0-0`

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{"podAntiAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":[{"labelSelector":{"matchLabels":{"control-plane":"controller-manager"}},"topologyKey":"kubernetes.io/hostname"}]}}` | Set affinity |
| autoscaling | object | `{"enabled":false,"maxReplicas":7,"minReplicas":1,"targetCPUUtilizationPercentage":80}` | configure hpa |
| autoscaling.targetCPUUtilizationPercentage | int | `80` | cpu threshold |
| config | object | `{"healthProbeBindAddress":":8081","ipReleasePeriod":"5s","ipReserveMaxCount":300,"ipReserveTime":"40m","leaderElectionEnable":true,"metricsBindAddress":":8080","webhookPort":9443}` | Set capo config |
| config.healthProbeBindAddress | string | `":8081"` | health probe bind address |
| config.ipReleasePeriod | string | `"5s"` | ip release period |
| config.ipReserveMaxCount | int | `300` | ip reserve max count |
| config.ipReserveTime | string | `"40m"` | ip reserve max time |
| config.leaderElectionEnable | bool | `true` | enable leaderElect |
| config.metricsBindAddress | string | `":8080"` | metrics bind address |
| config.webhookPort | int | `9443` | webhook port |
| fullnameOverride | string | `""` | Override the expanded name of the chart |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.pullSecrets | list | `[]` |  |
| image.repository | string | `"xdfdotcn/ip-reserve-delay-release"` |  |
| image.tag | string | `"v1.0.1"` |  |
| nameOverride | string | `""` | Override the name of the chart |
| namespace | string | `nil` | Namespace the chart deploys to |
| nodeSelector | object | `{}` | Which nodes the Set pod will be scheduled to |
| podAnnotations | object | `{}` | Set additional annotation |
| podSecurityContext | object | `{"runAsNonRoot":true}` | Set POD level security context |
| rbacImage.pullPolicy | string | `"IfNotPresent"` |  |
| rbacImage.pullSecrets | list | `[]` |  |
| rbacImage.repository | string | `"xdfdotcn/kube-rbac-proxy"` |  |
| rbacImage.tag | string | `"v0.12.0"` |  |
| replicaCount | int | `3` | Number of instances, high availability configuration Please set it to 3 |
| resources.limits.cpu | string | `"500m"` |  |
| resources.limits.memory | string | `"1024Mi"` |  |
| resources.requests.cpu | string | `"100m"` |  |
| resources.requests.memory | string | `"256Mi"` |  |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}` | Set container level security context |
| service | object | `{"port":443,"type":"ClusterIP"}` | Set the service |
| service.port | int | `443` | Default https port |
| tolerations | list | `[]` | Set tolerations node taint |

## 添加 chart 仓库

在客户端，存储库使用以下`$ helm repo`命令添加仓库：
```shell
# helm repo add  xdfgithubrepo  https://xdfdotcn.github.io/capo 
"xdfgithubrepo" has been added to your repositories
```
搜索 Helm chart 存储库
```shell
# helm search repo xdfgithubrepo
NAME                    CHART VERSION   APP VERSION     DESCRIPTION                                      
xdfgithubrepo/capo      0.1.0           1.0.1           A Helm chart for Kubernetes Calico CNI IP Reserve
```

## 部署
通过下面的命令，将安装 Release 名称为 my-capo 到 capo namespace，如果不存在，会自动创建命名空间。
```shell
# helm install my-capo -n capo xdfgithubrepo/capo --create-namespace
NAME: my-capo
LAST DEPLOYED: Wed Nov 23 09:32:32 2022
NAMESPACE: capo
STATUS: deployed
REVISION: 1
NOTES:
1. Get the application URL by running these commands:
kubectl get pods -n capo -o wide | grep my-capo

2. Chart version: 0.1.0
Capo version: v2

Thank you for installing capo! Your release is named ip.

💡 Note: There is a trade-off when deciding which approach to take regarding Namespace exclusions. Please see the documentation at https://capo.io/installation/#security-vs-operability to understand the risks.
```

## 卸载
从 k8s 中卸载：
```shell
helm uninstall my-capo -n capo
```

