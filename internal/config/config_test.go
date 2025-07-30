package config

import (
	"testing"
)

func TestProxyConfig_IsIPBlacklisted(t *testing.T) {
	tests := []struct {
		name           string
		blacklistedIPs []string
		testIP         string
		expected       bool
	}{
		{
			name:           "Empty blacklist",
			blacklistedIPs: []string{},
			testIP:         "192.168.1.100",
			expected:       false,
		},
		{
			name:           "IP in blacklist",
			blacklistedIPs: []string{"192.168.1.100", "10.0.0.1"},
			testIP:         "192.168.1.100",
			expected:       true,
		},
		{
			name:           "IP not in blacklist",
			blacklistedIPs: []string{"192.168.1.100", "10.0.0.1"},
			testIP:         "192.168.1.101",
			expected:       false,
		},
		{
			name:           "IP in CIDR range",
			blacklistedIPs: []string{"192.168.1.0/24"},
			testIP:         "192.168.1.50",
			expected:       true,
		},
		{
			name:           "IP not in CIDR range",
			blacklistedIPs: []string{"192.168.1.0/24"},
			testIP:         "192.168.2.50",
			expected:       false,
		},
		{
			name:           "Invalid IP",
			blacklistedIPs: []string{"192.168.1.100"},
			testIP:         "invalid-ip",
			expected:       false,
		},
		{
			name:           "Mixed individual and CIDR",
			blacklistedIPs: []string{"192.168.1.100", "10.0.0.0/8"},
			testIP:         "10.5.5.5",
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ProxyConfig{
				BlacklistedIPs: tt.blacklistedIPs,
			}
			result := config.IsIPBlacklisted(tt.testIP)
			if result != tt.expected {
				t.Errorf("IsIPBlacklisted() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestProxyConfig_IsIPWhitelisted(t *testing.T) {
	tests := []struct {
		name           string
		whitelistedIPs []string
		testIP         string
		expected       bool
	}{
		{
			name:           "Empty whitelist",
			whitelistedIPs: []string{},
			testIP:         "192.168.1.100",
			expected:       false,
		},
		{
			name:           "IP in whitelist",
			whitelistedIPs: []string{"192.168.1.100", "10.0.0.1"},
			testIP:         "192.168.1.100",
			expected:       true,
		},
		{
			name:           "IP not in whitelist",
			whitelistedIPs: []string{"192.168.1.100", "10.0.0.1"},
			testIP:         "192.168.1.101",
			expected:       false,
		},
		{
			name:           "IP in CIDR range",
			whitelistedIPs: []string{"192.168.1.0/24"},
			testIP:         "192.168.1.50",
			expected:       true,
		},
		{
			name:           "IP not in CIDR range",
			whitelistedIPs: []string{"192.168.1.0/24"},
			testIP:         "192.168.2.50",
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ProxyConfig{
				WhitelistedIPs: tt.whitelistedIPs,
			}
			result := config.IsIPWhitelisted(tt.testIP)
			if result != tt.expected {
				t.Errorf("IsIPWhitelisted() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
