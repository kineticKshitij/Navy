//go:build !darwin
// +build !darwin

package ipsec

import "fmt"

func newDarwinManager() (IPsecManager, error) {
	return nil, fmt.Errorf("macOS IPsec manager not available on %s", GetPlatform())
}
