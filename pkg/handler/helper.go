package handler

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	v3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	cons "github.com/xdfdotcn/capo/pkg/constants"
	"github.com/xdfdotcn/capo/pkg/metrics"
	"github.com/xdfdotcn/capo/pkg/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type patchMapValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

// 实现 sort.Interface 接口
type byDuration []podIPDuration

type podIPDuration struct {
	podIP    string
	duration time.Duration
}

type byIp []string

func (s byIp) Len() int {
	return len(s)
}
func (s byIp) Less(i, j int) bool {
	return s[i] < s[j]
}
func (s byIp) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (b byDuration) Len() int {
	return len(b)
}

// 从大到小排序
func (b byDuration) Less(i, j int) bool {
	return b[i].duration > b[j].duration
}

func (b byDuration) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func getReserveCIDRs(ipReservation *v3.IPReservation, releaseIPs []string) ([]string, int64) {
	var reserveCIDRs []string
	var totalIP int64
	hasCidr := make(map[string]bool, len(ipReservation.Spec.ReservedCIDRs))
	for _, cidr := range ipReservation.Spec.ReservedCIDRs {
		isRelease := false
		// If only one ip is released from a cidr segment, all IPs in the segment will be released here
		// Excluding human factors, this program will not increase the cidr to IP Reservation, but a single IP
		// todo consider the human factor to add a cidr segment to the IPReservation scenario
		for _, releaseIP := range releaseIPs {
			if cidr == releaseIP {
				isRelease = true
				break
			}
		}

		if !isRelease && !hasCidr[cidr] {
			if cidr != cons.SystemReserveIP {
				totalIP += utils.IPRangeSize(utils.ParseCidr(cidr)).Int64()
			}
			reserveCIDRs = append(reserveCIDRs, cidr)
		}
		hasCidr[cidr] = true
	}

	//The Kubernetes API server does not recursively create nested objects for JSON patch inputs, so when spec.reservedCIDRs is nil,
	//JSONPatch will fail, so add a permanent system reserved IP: 1.1.1.1 in reservedCIDRs
	if !hasCidr[cons.SystemReserveIP] {
		reserveCIDRs = append(reserveCIDRs, cons.SystemReserveIP)
	}

	return reserveCIDRs, totalIP
}

func getPodInfo(podIP, podInfoTime string) (string, string, string, time.Duration, error) {
	split := strings.Split(podInfoTime, cons.SeparatorUnderscore)
	if len(split) != 4 {
		return "", "", "", 0, fmt.Errorf("podInfoTime is invalid, skip podIP %s , podInfoTime %s", podIP, podInfoTime)
	}

	podNamespace := split[0]
	podName := split[1]
	podPlaceNodeName := split[2]
	ipReservedTimeStr := split[3]

	ipReservedTime, err := time.ParseInLocation(cons.TimeLayout, ipReservedTimeStr, time.Local)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("parse podInfoTime is err: %v, skip podIP %s , podInfoTime %s", err.Error(), podIP, podInfoTime)
	}

	// keptTime unit is nanosecond
	keptTime := time.Since(ipReservedTime)
	return podPlaceNodeName, podNamespace, podName, keptTime, err
}

func getReleaseIPs(podIPMap *v1.ConfigMap, logger logr.Logger, r *IPKeeper) []string {
	var remainingIPs []podIPDuration
	var releaseIPs []string
	for podIP, podInfoTime := range podIPMap.Data {
		_, _, _, keptTime, err := getPodInfo(podIP, podInfoTime)
		if err != nil {
			logger.Info(err.Error())
			continue
		}
		/*metrics.IPReserveKeptTime.With(map[string]string{
			cons.LabelPodIP:        podIP,
			cons.LabelPodNamespace: podNamespace,
			cons.LabelPodName:      podName,
			cons.LabelNodeName:     podPlaceNodeName,
		}).Set(keptTime.Seconds())*/

		if keptTime < r.config.IPReserveTime.Duration {
			remainingIPs = append(remainingIPs, podIPDuration{
				podIP:    podIP,
				duration: keptTime,
			})
			continue
		}

		//IP to be released
		releaseIPs = append(releaseIPs, podIP)
		delete(podIPMap.Data, podIP)
	}

	releaseCount := len(remainingIPs) - *r.config.IPReserveMaxCount
	if releaseCount > 0 {
		// Sorted by the saved duration from largest to smallest
		sort.Stable(byDuration(remainingIPs))
		for _, item := range remainingIPs {
			if releaseCount <= 0 {
				break
			}
			podIP := item.podIP
			_, _, _, _, err := getPodInfo(podIP, podIPMap.Data[podIP])
			if err != nil {
				logger.Info(err.Error())
				continue
			}
			// because the count reaches the threshold
			metrics.IPReserveEvictionsCount.Inc()
			/*metrics.IPReserveEvictionsInfo.With(map[string]string{
				cons.LabelPodIP:        podIP,
				cons.LabelPodNamespace: podNamespace,
				cons.LabelPodName:      podName,
				cons.LabelNodeName:     podPlaceNodeName,
				cons.LabelKeptTime:     fmt.Sprintf("%v", keptTime.Seconds()),
			}).Set(1)*/
			delete(podIPMap.Data, podIP)
			releaseIPs = append(releaseIPs, podIP)
			releaseCount--
		}
	}
	return releaseIPs
}

func getResources(pod *v1.Pod) (*v1.ConfigMap, []byte) {
	podIPMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podIPMapNsName.Name,
			Namespace: podIPMapNsName.Namespace,
		},
		Data: map[string]string{},
	}

	// Must not update the time of the reserved IP already
	var patches []patchMapValue
	for _, ip := range pod.Status.PodIPs {
		podIPMap.Data[ip.IP] = buildPodInfo(pod.Namespace, pod.Name, pod.Spec.NodeName, time.Now())

		patch := patchMapValue{
			Op:    "add",
			Path:  "/spec/reservedCIDRs/-",
			Value: ip.IP,
		}
		patches = append(patches, patch)
	}
	patchJson, _ := json.Marshal(patches)

	return podIPMap, patchJson
}

func buildPodInfo(namespace, name, nodeName string, now time.Time) string {
	return fmt.Sprintf(cons.IPInfoPlaceholder, namespace, cons.SeparatorUnderscore,
		name, cons.SeparatorUnderscore,
		nodeName, cons.SeparatorUnderscore,
		now.Format(cons.TimeLayout))
}

func ipDeduplicateAppend(pod *v1.Pod, ipReservation *v3.IPReservation, logger logr.Logger) []string {
	var addedIP []string
	// deduplicate
	for _, ip := range pod.Status.PodIPs {
		exist := false
		for _, cidr := range ipReservation.Spec.ReservedCIDRs {
			ipNet := utils.ParseCidr(cidr)
			if ipNet == nil {
				logger.Info("is invalid", "cidr", cidr)
				continue
			}

			if ipNet.Contains(net.ParseIP(ip.IP)) {
				exist = true
				break
			}
		}
		if !exist {
			addedIP = append(addedIP, ip.IP)
			ipReservation.Spec.ReservedCIDRs = append(ipReservation.Spec.ReservedCIDRs, ip.IP)
		}
	}
	return addedIP
}
