package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	// 初始化内置控制器指标
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	//这个包获取leader选举metrics失败, 因为和这里的metrics.Registry不是同一个对象
	// 这里是kubebuilder controller runtime包内置的，而leader选举 metrics位于 k8s 代码中
	//_ "k8s.io/component-base/metrics/prometheus/clientgo"
	cons "github.com/xdfdotcn/capo/pkg/constants"
)

var (
	IPReserveCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: cons.IPReserveMetricNamespace,
			Name:      "count",
			Help:      "Number of ip reserve",
		},
	)

	IPReserveEvictionsCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: cons.IPReserveMetricNamespace,
			Name:      "evictions_count",
			Help:      "Number of IP addresses that were released forcibly because the number of held IP addresses reached the retention threshold",
		},
	)

	IPReserveCountMaxLimit = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: cons.IPReserveMetricNamespace,
			Name:      "count_max",
			Help:      "ip reserve count max number",
		},
	)

	/*IPReserveKeptTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: cons.IPReserveMetricNamespace,
			Name:      "kept_time",
			Help:      "ip reserve kept time",
		},
		[]string{cons.LabelPodIP, cons.LabelPodNamespace, cons.LabelPodName, cons.LabelNodeName},
	)*/

	/*IPReserveEvictionsInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: cons.IPReserveMetricNamespace,
			Name:      "evictions_info",
			Help:      "ip reserve evictions info",
		},
		[]string{cons.LabelPodIP, cons.LabelPodNamespace, cons.LabelPodName, cons.LabelNodeName, cons.LabelKeptTime},
	)*/
)

func init() {
	// Register custom metrics with the global prometheus registry
	// Registry is a prometheus registry for storing metrics within the controller-runtime.
	// todo 导入 _ "k8s.io/component-base/metrics/prometheus/clientgo" 这个包获取leader选举metrics失败, 因为和这里的metrics.Registry不是同一个对象
	// 这里是kubebuilder controller runtime包内置的，而leader选举 metrics位于 k8s 代码中
	metrics.Registry.MustRegister(IPReserveCount, IPReserveCountMaxLimit, IPReserveEvictionsCount)
}
