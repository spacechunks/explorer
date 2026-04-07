package workload

type Config struct {
	MCManagementAPIToken string
	ServerMonImage       string
	PlatformdListenSock  string
	PlatformdSocketUID   uint64
	PlatformdSocketGID   uint64
}
