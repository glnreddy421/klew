package kube

import (
	"os"
	"strings"
	"sync"
)

// NetworkProxyOptions are HTTP(S) proxy settings applied to Kubernetes API clients.
type NetworkProxyOptions struct {
	HTTPProxy  string `json:"httpProxy"`
	HTTPSProxy string `json:"httpsProxy"`
	NoProxy    string `json:"noProxy"`
}

var (
	networkMu    sync.RWMutex
	proxyHTTP    string
	proxyHTTPS   string
	proxyNoProxy string
)

// InitNetworkProxyFromEnv seeds in-memory proxy settings from the process environment
// after login-shell bootstrap (GUI apps may inherit proxy vars from the user's shell).
func InitNetworkProxyFromEnv() {
	networkMu.Lock()
	defer networkMu.Unlock()
	if proxyHTTP == "" {
		proxyHTTP = firstEnv("HTTP_PROXY", "http_proxy")
	}
	if proxyHTTPS == "" {
		proxyHTTPS = firstEnv("HTTPS_PROXY", "https_proxy")
	}
	if proxyNoProxy == "" {
		proxyNoProxy = firstEnv("NO_PROXY", "no_proxy")
	}
}

// SetNetworkProxy applies proxy settings to the process environment for client-go.
// Empty strings unset the corresponding variables.
func SetNetworkProxy(httpProxy, httpsProxy, noProxy string) NetworkProxyOptions {
	networkMu.Lock()
	defer networkMu.Unlock()
	proxyHTTP = strings.TrimSpace(httpProxy)
	proxyHTTPS = strings.TrimSpace(httpsProxy)
	proxyNoProxy = strings.TrimSpace(noProxy)
	applyNetworkProxyEnvLocked()
	return networkProxyLocked()
}

// GetNetworkProxy returns the active proxy configuration.
func GetNetworkProxy() NetworkProxyOptions {
	networkMu.RLock()
	defer networkMu.RUnlock()
	return networkProxyLocked()
}

func networkProxyLocked() NetworkProxyOptions {
	return NetworkProxyOptions{
		HTTPProxy:  proxyHTTP,
		HTTPSProxy: proxyHTTPS,
		NoProxy:    proxyNoProxy,
	}
}

func applyNetworkProxyEnvLocked() {
	setOrUnsetEnv("HTTP_PROXY", proxyHTTP)
	setOrUnsetEnv("http_proxy", proxyHTTP)
	setOrUnsetEnv("HTTPS_PROXY", proxyHTTPS)
	setOrUnsetEnv("https_proxy", proxyHTTPS)
	setOrUnsetEnv("NO_PROXY", proxyNoProxy)
	setOrUnsetEnv("no_proxy", proxyNoProxy)
}

func setOrUnsetEnv(key, val string) {
	if val == "" {
		_ = os.Unsetenv(key)
		return
	}
	_ = os.Setenv(key, val)
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if val := strings.TrimSpace(os.Getenv(key)); val != "" {
			return val
		}
	}
	return ""
}
