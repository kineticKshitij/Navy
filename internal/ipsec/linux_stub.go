//go:build !linux
// +build !linux

package ipsec

import "fmt"

func newLinuxManager() (IPsecManager, error) {
	return nil, fmt.Errorf("Linux IPsec manager not available on %s", GetPlatform())
}
