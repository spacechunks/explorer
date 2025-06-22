package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/peterbourgon/ff/v3"
	"github.com/spacechunks/explorer/platformd"
	"github.com/spacechunks/explorer/platformd/checkpoint"
)

func main() {
	var (
		logger                       = slog.New(slog.NewTextHandler(os.Stdout, nil))
		fs                           = flag.NewFlagSet("platformd", flag.ContinueOnError)
		proxyServiceListenSock       = fs.String("management-server-listen-sock", "/var/run/platformd/platformd.sock", "path to the unix domain socket to listen on")          //nolint:lll
		criListenSock                = fs.String("cri-listen-sock", "/var/run/crio/crio.sock", "path to the unix domain socket the CRI is listening on")                       //nolint:lll
		envoyImage                   = fs.String("envoy-image", "", "container image to use for envoy")                                                                        //nolint:lll
		coreDNSImage                 = fs.String("coredns-image", "", "container image to use for CoreDNS")                                                                    //nolint:lll
		getsockoptCgroup             = fs.String("getsockopt-cgroup", "", "container image to use for coredns")                                                                //nolint:lll
		dnsServer                    = fs.String("dns-server", "", "dns server used by the containers")                                                                        //nolint:lll
		hostIface                    = fs.String("host-iface", "", "internet-facing network interface for ingress and egress traffic")                                         //nolint:lll
		maxAttempts                  = fs.Uint("max-attempts", 5, "maximum number of attempts workload creation attempts")                                                     //nolint:lll
		syncInterval                 = fs.Duration("sync-interval", 200*time.Millisecond, "i")                                                                                 //nolint:lll
		nodeID                       = fs.String("node-id", "", "unique node id")                                                                                              //nolint:lll
		minPort                      = fs.Uint("min-port", 30000, "start of the port range")                                                                                   //nolint:lll
		maxPort                      = fs.Uint("max-port", 40000, "end of the port range")                                                                                     //nolint:lll
		workloadNamespace            = fs.String("workload-namespace", "", "namespace where the workload is deployed")                                                         //nolint:lll
		registryEndpoint             = fs.String("registry-endpoint", "", "registry endpoint where base images will be pulled from and checkpoints pushed to")                 //nolint:lll
		registryUser                 = fs.String("registry-user", "", "user for the registry")                                                                                 //nolint:lll
		registryPass                 = fs.String("registry-password", "", "password for the registry")                                                                         //nolint:lll
		controlPlaneEndpoint         = fs.String("control-plane-endpoint", "", "control plane endpoint")                                                                       //nolint:lll
		checkCPUPeriod               = fs.Uint64("checkpoint-cpu-period", 0, "period of checking CPU period")                                                                  //nolint:lll
		checkCPUQuota                = fs.Uint64("checkpoint-cpu-quota", 0, "quota of checking CPU quota")                                                                     //nolint:lll
		checkMemoryLimitInBytes      = fs.Uint64("checkpoint-memory-limit-bytes", 0, "memory limit of the container that will be checkpointed")                                //nolint:lll
		checkLocationDir             = fs.String("checkpoint-file-dir", "/tmp/platformd", "directory where checkpoint files will be stored")                                   //nolint:lll
		checkTimeout                 = fs.Uint64("checkpoint-timeout-seconds", 60, "timeout for checkpoint creation")                                                          //nolint:lll
		checkListenAddr              = fs.String("checkpoint-listen-addr", "", "timeout for checkpoint creation")                                                              //nolint:lll
		checkStatusRetentionDuration = fs.Duration("checkpoint-status-retention-period", 1*time.Minute, "timeout for checkpoint creation")                                     //nolint:lll
		checkContainerReadyTimeout   = fs.Duration("checkpoint-container-ready-timeout", 1*time.Minute, "maximum time to wait until the container is ready for checkpointing") //nolint:lll
		_                            = fs.String("config", "/etc/platformd/config.json", "path to the config file")                                                            //nolint:lll
	)
	if err := ff.Parse(fs, os.Args[1:],
		ff.WithEnvVarPrefix("PLATFORMD"),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.JSONParser),
	); err != nil {
		die(logger, "failed to parse config", err)
	}

	var (
		cfg = platformd.Config{
			ManagementServerListenSock: *proxyServiceListenSock,
			CRIListenSock:              *criListenSock,
			EnvoyImage:                 *envoyImage,
			CoreDNSImage:               *coreDNSImage,
			GetsockoptCGroup:           *getsockoptCgroup,
			DNSServer:                  *dnsServer,
			HostIface:                  *hostIface,
			MaxAttempts:                *maxAttempts,
			SyncInterval:               *syncInterval,
			NodeID:                     *nodeID,
			MinPort:                    uint16(*minPort), // TODO: validation
			MaxPort:                    uint16(*maxPort), // TODO: validation
			WorkloadNamespace:          *workloadNamespace,
			RegistryEndpoint:           *registryEndpoint,
			RegistryUser:               *registryUser,
			RegistryPass:               *registryPass,
			ControlPlaneEndpoint:       *controlPlaneEndpoint,
			CheckpointConfig: checkpoint.Config{
				CPUPeriod:                int64(*checkCPUPeriod),          // TODO: validation
				CPUQuota:                 int64(*checkCPUQuota),           // TODO: validation
				MemoryLimitBytes:         int64(*checkMemoryLimitInBytes), // TODO: validation
				CheckpointFileDir:        *checkLocationDir,
				CheckpointTimeoutSeconds: int64(*checkTimeout), // TODO: validation
				ListenAddr:               *checkListenAddr,
				StatusRetentionPeriod:    *checkStatusRetentionDuration,
				ContainerReadyTimeout:    *checkContainerReadyTimeout,
			},
		}
		ctx    = context.Background()
		server = platformd.NewServer(logger)
	)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		s := <-c
		logger.Info("received shutdown signal", "signal", s)
		server.Stop()
	}()

	if err := server.Run(ctx, cfg); err != nil {
		var multi *multierror.Error
		if errors.As(err, &multi) {
			errs := make([]string, 0, len(multi.WrappedErrors()))
			for _, err := range multi.WrappedErrors() {
				errs = append(errs, err.Error())
			}
			die(logger, "failed to run server", errors.New(strings.Join(errs, ",")))
			return
		}
		die(logger, "failed to run server", err)
	}
}

func die(logger *slog.Logger, msg string, err error) {
	logger.Error(msg, "err", err)
	os.Exit(1)
}
