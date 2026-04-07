package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/peterbourgon/ff/v3"
	workloadv1alpha2 "github.com/spacechunks/explorer/api/platformd/workload/v1alpha2"
	"github.com/spacechunks/explorer/servermon"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	var (
		logger                   = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		ctx, cancel              = context.WithCancel(context.Background())
		fs                       = flag.NewFlagSet("servermon", flag.ContinueOnError)
		playerCountCheckInterval = fs.Duration("player-count-check-interval", 2*time.Minute, "in what interval the player count of the server will be checked")                     //nolint:lll
		mgmtEndpoint             = fs.String("mc-server-management-api-endpoint", "ws://localhost:26656", "the endpoint at which the minecraft server management api is available") //nolint:lll
		mgmtAPIToken             = fs.String("mc-server-management-api-token", "", "token to use for the minecraft server management api")                                          //nolint:lll
		platformdListenSock      = fs.String("platformd-listen-sock", "", "path to the platformd management api unix socket file")                                                  //nolint:lll
	)

	if err := ff.Parse(fs, os.Args[1:],
		ff.WithEnvVarPrefix("SERVERMON"),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.JSONParser),
	); err != nil {
		logger.ErrorContext(ctx, "failed to parse config", "err", err)
		os.Exit(1)
	}

	conn, err := grpc.NewClient(
		*platformdListenSock,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to dial platformd management api", "err", err)
		os.Exit(1)
	}

	var (
		cfg = servermon.Config{
			PlayerCountCheckInterval:      *playerCountCheckInterval,
			MCServerManagementAPIEndpoint: *mgmtEndpoint,
			MCServerManagementAPIToken:    *mgmtAPIToken,
		}
		mon = servermon.New(logger, cfg, workloadv1alpha2.NewWorkloadServiceClient(conn))
	)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		s := <-c
		logger.Info("received shutdown signal", "signal", s.String())
		cancel()
	}()

	if err := mon.Run(ctx); err != nil {
		logger.ErrorContext(ctx, "error running servermon", "err", err)
		os.Exit(1)
	}
}
