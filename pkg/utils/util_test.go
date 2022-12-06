package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	cons "github.com/xdfdotcn/capo/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestCreateLogger(t *testing.T) {
	assert.NotNil(t, CreateLogger(true, true))
	assert.NotNil(t, CreateLogger(false, false))
	assert.NotNil(t, CreateLogger(true, false))
	assert.NotNil(t, CreateLogger(false, true))
}

func TestIPRangeCount(t *testing.T) {
	assert.Zero(t, IPRangeSize(nil).Int64())
	assert.Zero(t, IPRangeSize(ParseCidr("17.2.2.20.0")).Int64())
	assert.Equal(t, int64(1), IPRangeSize(ParseCidr("17.2.2.20")).Int64())
	assert.Equal(t, int64(4096), IPRangeSize(ParseCidr("17.2.2.0/20")).Int64())
}

func TestLabelSelector(t *testing.T) {
	labelSelector := &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      cons.LabelSelectorStatefulSetPodKey,
				Operator: metav1.LabelSelectorOpExists,
			},
			{
				Key:      cons.LabelSelectorKafkaPodKey,
				Operator: metav1.LabelSelectorOpExists,
			},
		},
	}
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	assert.Nil(t, err)

	requirements, _ := selector.Requirements()
	anySelector := NewAnyMatchSelector(selector, requirements)

	matchLabels := []map[string]string{
		{

			cons.LabelSelectorStatefulSetPodKey: "1",
			"otherLabelKey":                     "true",
		},
		{
			cons.LabelSelectorKafkaPodKey: "1",
			"otherLabelKey":               "true",
		},
	}

	for _, label := range matchLabels {
		// true
		assert.True(t, anySelector.Matches(labels.Set(label)))
	}

	notMatchLabels := []map[string]string{
		{
			"otherLabelKey": "true",
		},
	}

	for _, label := range notMatchLabels {
		// false
		assert.False(t, anySelector.Matches(labels.Set(label)))
	}
}

func TestGetOutboundIP(t *testing.T) {
	ip := GetOutboundIP()
	t.Log(ip.To4().String())
}
