package ipsec

import (
	"fmt"
	"runtime"
)

// NewManager creates a platform-specific IPsec manager
func NewManager() (IPsecManager, error) {
	switch runtime.GOOS {
	case "linux":
		return newLinuxManager()
	case "windows":
		return newWindowsManager()
	case "darwin":
		return newDarwinManager()
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// GetPlatform returns the current operating system
func GetPlatform() string {
	return runtime.GOOS
}

// IsPlatformSupported checks if the current platform is supported
func IsPlatformSupported() bool {
	switch runtime.GOOS {
	case "linux", "windows", "darwin":
		return true
	default:
		return false
	}
}
