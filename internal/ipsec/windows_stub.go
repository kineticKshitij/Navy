//go:build !windows
// +build !windows

package ipsec

import "fmt"

func newWindowsManager() (IPsecManager, error) {
	return nil, fmt.Errorf("Windows IPsec manager not available on %s", GetPlatform())
}
