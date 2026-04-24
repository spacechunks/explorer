package workload

import "net/url"

type Config struct {
	MCManagementAPIToken   string
	ServerMonImage         string
	PlatformdListenSockURL *url.URL
	PlatformdSocketUID     uint64
	PlatformdSocketGID     uint64
}
