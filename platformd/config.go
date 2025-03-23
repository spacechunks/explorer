package platformd

import "time"

type Config struct {
	ManagementServerListenSock string
	CRIListenSock              string
	EnvoyImage                 string
	CoreDNSImage               string
	GetsockoptCGroup           string
	DNSServer                  string
	HostIface                  string
	MaxAttempts                uint
	SyncInterval               time.Duration
	NodeID                     string
	MinPort                    uint16
	MaxPort                    uint16
	WorkloadNamespace          string
	RegistryEndpoint           string
	ControlPlaneEndpoint       string
}
