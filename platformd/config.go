package platformd

import (
	"time"
)

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
	RegistryUser               string
	RegistryPass               string
	ControlPlaneEndpoint       string
	CheckpointConfig           struct {
		CPUPeriod                int64
		CPUQuota                 int64
		MemoryLimitBytes         int64
		CheckpointFileDir        string
		CheckpointTimeoutSeconds int64
		RegistryUser             string
		RegistryPass             string
		ListenAddr               string
		StatusRetentionPeriod    time.Duration
		ContainerReadyTimeout    time.Duration
		WaitAfterLogPeriod       time.Duration
	}
}
