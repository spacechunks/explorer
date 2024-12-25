package platformd

type Config struct {
	ManagementServerListenSock string
	CRIListenSock              string
	EnvoyImage                 string
	GetsockoptCGroup           string
	DNSServer                  string
}
