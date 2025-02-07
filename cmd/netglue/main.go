/*
Explorer Platform, a platform for hosting and discovering Minecraft servers.
Copyright (C) 2024 Yannic Rieger <oss@76k.io>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/version"
	proxyv1alpha1 "github.com/spacechunks/platform/api/platformd/proxy/v1alpha1"
	workloadv1alpha1 "github.com/spacechunks/platform/api/platformd/workload/v1alpha1"
	"github.com/spacechunks/platform/internal/cni"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	handler, err := cni.NewHandler()
	if err != nil {
		log.Fatalf("failed to create handler: %v", err)
	}
	c := cni.NewCNI(handler)
	skel.PluginMainFuncs(skel.CNIFuncs{
		Add: func(args *skel.CmdArgs) error {
			var conf cni.Conf
			if err := json.Unmarshal(args.StdinData, &conf); err != nil {
				return fmt.Errorf("parse config: %v", err)
			}

			conn, err := grpc.NewClient(
				conf.PlatformdListenSock,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				return fmt.Errorf("failed to create proxy service grpc proxyCLient: %w", err)
			}

			return c.ExecAdd(
				args,
				conf,
				proxyv1alpha1.NewProxyServiceClient(conn),
				workloadv1alpha1.NewWorkloadServiceClient(conn),
			)
		},
		Del: c.ExecDel,
	}, version.All, "netglue: provides networking for chunks")
}
