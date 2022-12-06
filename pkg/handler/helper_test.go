package handler

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/go-logr/logr"
	v3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	configv1 "github.com/xdfdotcn/capo/apis/config/v1"
	cons "github.com/xdfdotcn/capo/pkg/constants"
	"github.com/xdfdotcn/capo/pkg/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type ExampleTestSuite struct {
	suite.Suite
	logger        logr.Logger
	pod           *v1.Pod
	ipReservation *v3.IPReservation
}

// Make sure that VariableThatShouldStartAtFive is set to five
// before each test
func (suite *ExampleTestSuite) SetupTest() {
	suite.logger = utils.CreateLogger(true, true)

	ip1 := "10.1.1.2"
	suite.pod = &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "redis",
		},
		Spec: v1.PodSpec{
			NodeName: "node01",
		},
		Status: v1.PodStatus{
			PodIP: ip1,
			PodIPs: []v1.PodIP{
				{
					IP: ip1,
				},
			},
		},
	}

	suite.ipReservation = &v3.IPReservation{
		Spec: v3.IPReservationSpec{
			ReservedCIDRs: []string{
				cons.SystemReserveIP,
				ip1,
			},
		},
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestExampleTestSuite(t *testing.T) {
	suite.Run(t, new(ExampleTestSuite))
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *ExampleTestSuite) TestIPDeduplicateAppend() {
	addIps := ipDeduplicateAppend(suite.pod, suite.ipReservation, suite.logger)
	suite.Empty(addIps)

	ip1 := "5.5.5.5"
	pod := &v1.Pod{
		Status: v1.PodStatus{
			PodIPs: []v1.PodIP{
				{
					IP: ip1,
				},
			},
		},
	}
	addIps = ipDeduplicateAppend(pod, suite.ipReservation, suite.logger)
	suite.Contains(addIps, ip1)
}

func TestTimeFormat(t *testing.T) {
	timeStr := "2022-11-24-13:11:50"
	parse, err := time.Parse(cons.TimeLayout, timeStr)
	assert.Nil(t, err)
	format := parse.Format(cons.TimeLayout)
	assert.Equal(t, timeStr, format)
}

func TestSortByDuration(t *testing.T) {
	podIPDurations := []podIPDuration{
		{
			podIP:    "1.1.1.1",
			duration: 10 * time.Second,
		},
		{
			podIP:    "2.1.1.1",
			duration: 30 * time.Second,
		},
		{
			podIP:    "3.1.1.1",
			duration: 20 * time.Second,
		},
	}

	sort.Stable(byDuration(podIPDurations))

	assert.Equal(t, 30*time.Second, podIPDurations[0].duration)
	assert.Equal(t, 10*time.Second, podIPDurations[2].duration)
}

func TestSortByIP(t *testing.T) {
	var (
		ip1 = "1.1.1.1"
		ip2 = "3.1.1.1"
		ip3 = "5.1.1.1"
	)
	bIP := byIp{
		ip2,
		ip1,
		ip3,
	}

	sort.Stable(bIP)

	assert.Equal(t, ip1, bIP[0])
	assert.Equal(t, ip2, bIP[1])
	assert.Equal(t, ip3, bIP[2])
}

func (suite *ExampleTestSuite) TestGetReserveCIDRs() {
	reserveCIDRs, totalIP := getReserveCIDRs(suite.ipReservation, nil)
	suite.Contains(reserveCIDRs, cons.SystemReserveIP)
	suite.Equal(totalIP, int64(1))

	ip1 := "2.3.4.5"
	releaseIPs := []string{
		ip1,
		"2.3.4.6",
	}

	ipr := &v3.IPReservation{
		Spec: v3.IPReservationSpec{
			ReservedCIDRs: []string{
				ip1,
			},
		},
	}

	reserveCIDRs, totalIP = getReserveCIDRs(ipr, releaseIPs)
	suite.NotContains(reserveCIDRs, releaseIPs)
	suite.Contains(reserveCIDRs, cons.SystemReserveIP)
	suite.Zero(totalIP)

	reserveCIDRs, totalIP = getReserveCIDRs(ipr, nil)
	suite.Contains(reserveCIDRs, ip1)
	suite.Equal(totalIP, int64(1))
}

func (suite *ExampleTestSuite) TestGetPodInfo() {
	var (
		nn       = "node01"
		ns       = "redis"
		name     = "test"
		startStr = "2022-11-24-14:33:22"
		ip       = "1.1.4.3"
	)
	startTime, err := time.Parse(cons.TimeLayout, startStr)
	suite.Nil(err)
	podInfo := buildPodInfo(ns, name, nn, startTime)
	nodeName, podNs, podName, _, err := getPodInfo(ip, podInfo)
	suite.Nil(err)
	suite.Equal(nodeName, nodeName)
	suite.Equal(nn, nodeName)
	suite.Equal(ns, podNs)
	suite.Equal(name, podName)

	_, _, _, _, err = getPodInfo(ip, "redis_test-1_node01_xxx_2022-11-24-14:33:22")
	suite.NotNil(err)

	_, _, _, _, err = getPodInfo(ip, "redis_test-1_node01_2022-11-24-14:70:22")
	suite.NotNil(err)
}

func (suite *ExampleTestSuite) TestGetReleaseIPs() {
	keeper := &IPKeeper{
		config: &configv1.CapoConfig{
			IPReserveMaxCount: pointer.IntPtr(300),
			IPReserveTime: metav1.Duration{
				Duration: 40 * time.Minute,
			},
		},
	}

	ips := []string{
		"10.0.1.1",
		"10.0.1.2",
		"10.0.1.3",
	}
	podIPMap := &v1.ConfigMap{
		Data: map[string]string{
			ips[0]: "redis_test0-1_node01_2022-11-24-14:33:22",
			ips[1]: "kafka_test2-1-xxx_node04_2022-11-24-14:33:22",
			ips[2]: "zk_test3-1_node03_2022-11-24-14:33:22",
		},
	}

	releaseIPs := getReleaseIPs(podIPMap, suite.logger, keeper)
	for _, ip := range ips {
		suite.Contains(releaseIPs, ip)
	}

	// did not reach the release time
	ip1 := "1.1.1.3"
	podIPMap.Data[ip1] = buildPodInfo("redis", "test4", "node09", time.Now())
	releaseIPs = getReleaseIPs(podIPMap, suite.logger, keeper)
	suite.NotContains(releaseIPs, ip1)

	// The number of IP reservations reaches the threshold
	suite.Len(podIPMap.Data, 1)
	// max is 1
	max := 1

	keeper.config.IPReserveMaxCount = pointer.Int(max)

	now := time.Now()
	time1 := now.Add(-2 * time.Minute)
	podIPMap.Data[ip1] = buildPodInfo("redis", "test4", "node09", time1)

	time2 := now.Add(-10 * time.Minute)
	ip2 := "1.2.43.5"
	podIPMap.Data[ip2] = buildPodInfo("redis1", "test5", "node07", time2)

	time3 := now.Add(-5 * time.Minute)
	ip3 := "4.5.6.7"
	podIPMap.Data[ip3] = buildPodInfo("redis2", "test6", "node01", time3)

	releaseIPs = getReleaseIPs(podIPMap, suite.logger, keeper)
	suite.Len(podIPMap.Data, max)
	suite.Contains(podIPMap.Data, ip1)
	suite.NotContains(releaseIPs, ip1)
	suite.Contains(releaseIPs, ip2)
	suite.Contains(releaseIPs, ip3)
}

func (suite *ExampleTestSuite) TestGetResources() {
	patchTph := `[{"op":"add","path":"/spec/reservedCIDRs/-","value":"%s"}]`
	podIPMap, patchJson := getResources(suite.pod)
	curTime := time.Now()
	suite.Equal(podIPMap.Data[suite.pod.Status.PodIP], buildPodInfo(suite.pod.Namespace, suite.pod.Name, suite.pod.Spec.NodeName, curTime))
	suite.Equal(podIPMap.Name, podIPMapNsName.Name)
	suite.Equal(podIPMap.Namespace, podIPMapNsName.Namespace)
	suite.Equal(string(patchJson), fmt.Sprintf(patchTph, suite.pod.Status.PodIP))
}
