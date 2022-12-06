/*
Copyright 2022 xdfdotcn
*/

package utils

import (
	"log"
	"math/big"
	"net"

	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/labels"
	k8szap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func CreateLogger(debug bool, development bool) logr.Logger {
	// create encoder config
	var config zapcore.EncoderConfig
	if development {
		config = zap.NewDevelopmentEncoderConfig()
	} else {
		config = zap.NewProductionEncoderConfig()
	}
	// set human readable timestamp format regardless whether development mode is on
	config.EncodeTime = zapcore.ISO8601TimeEncoder

	// create the encoder
	var encoder zapcore.Encoder
	if development {
		encoder = zapcore.NewConsoleEncoder(config)
	} else {
		encoder = zapcore.NewJSONEncoder(config)
	}

	// set the log level
	level := zap.InfoLevel
	if debug {
		level = zap.DebugLevel
	}

	return k8szap.New(k8szap.UseDevMode(development), k8szap.Encoder(encoder), k8szap.Level(level))
}

func ParseCidr(ipOrCidr string) *net.IPNet {
	var (
		err   error
		ipNet *net.IPNet
	)
	parsedIP := net.ParseIP(ipOrCidr)
	if parsedIP == nil {
		_, ipNet, err = net.ParseCIDR(ipOrCidr)
		if err == nil {
			return ipNet
		}
		return nil
	}

	ipNet = &net.IPNet{
		IP:   parsedIP,
		Mask: net.IPv4Mask(255, 255, 255, 255),
	}

	return ipNet
}

// github.com/netdata/go.d.plugin@v0.38.0/pkg/iprange/range.go
func IPRangeSize(cidr *net.IPNet) *big.Int {
	if cidr == nil {
		return big.NewInt(0)
	}
	ones, bits := cidr.Mask.Size()
	return big.NewInt(0).Lsh(big.NewInt(1), uint(bits-ones))
}

type AnyMatchSelector struct {
	r []labels.Requirement
	s labels.Selector
}

func NewAnyMatchSelector(s labels.Selector, r []labels.Requirement) *AnyMatchSelector {
	return &AnyMatchSelector{
		r: r,
		s: s,
	}
}

func (a *AnyMatchSelector) Matches(l labels.Labels) bool {
	for ix := range a.r {
		if matches := a.r[ix].Matches(l); matches {
			return true
		}
	}
	return false
}

//get the local IP address, Here is a better solution to retrieve the preferred outbound ip address when there are multiple ip interfaces exist on the machine.
//https://stackoverflow.com/questions/23558425/how-do-i-get-the-local-ip-address-in-go
// Get preferred outbound ip of this machine
func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}
