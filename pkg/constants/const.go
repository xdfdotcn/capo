package cons

const (
	IPReserveMetricNamespace = "ip_reserve"
	IPReserveKey             = "ip-reserve"
	IPReserveValue           = "enabled"
	IPReservationName        = "ip-reserve-delay-release"
	EnvNamespace             = "POD_NAMESPACE"
	TimeLayout               = "2006-01-02-15:04:05"
	SeparatorUnderscore      = "_"
	LabelPodIP               = "pod_ip"
	LabelPodNamespace        = "pod_ip_owner_namespace"
	LabelPodName             = "pod_ip_owner_name"
	LabelNodeName            = "pod_ip_owner_node_name"
	LabelKeptTime            = "pod_ip_kept_time"
	IPInfoPlaceholder        = "%s%s%s%s%s%s%s"
	//LabelSelectorStatefulSetPodKey = v1.StatefulSetPodNameLabel
	LabelSelectorStatefulSetPodKey = "statefulset.kubernetes.io/pod-name"
	LabelSelectorKafkaPodKey       = "brokerId"
	PodSubResourceEviction         = "eviction"
	SystemReserveIP                = "1.1.1.1"
)
