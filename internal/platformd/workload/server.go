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

package workload

import (
	"context"

	workloadv1alpha1 "github.com/spacechunks/platform/api/platformd/workload/v1alpha1"
)

type Server struct {
	workloadv1alpha1.UnimplementedWorkloadServiceServer
	svc Service
}

func NewServer(svc Service) *Server {
	return &Server{
		svc: svc,
	}
}

func (s *Server) RunWorkload(
	ctx context.Context,
	req *workloadv1alpha1.RunWorkloadRequest,
) (*workloadv1alpha1.RunWorkloadResponse, error) {
	opts := RunOptions{
		Name:                 req.Name,
		Image:                req.Image,
		Namespace:            req.Namespace,
		Hostname:             req.Hostname,
		Labels:               req.Labels,
		NetworkNamespaceMode: req.NetworkNamespaceMode,
		DNSServer:            "10.0.0.53", // TODO: refactor
	}

	w, err := s.svc.RunWorkload(ctx, opts)
	if err != nil {
		return nil, err
	}

	// FIXME(yannic): if we have more objects create codec package
	//                which contains conversion logic from domain
	//                to grpc object
	//
	return &workloadv1alpha1.RunWorkloadResponse{
		Workload: &workloadv1alpha1.Workload{
			Id:                   w.ID,
			Name:                 w.Name,
			Image:                w.Image,
			Namespace:            w.Namespace,
			Hostname:             w.Hostname,
			Labels:               w.Labels,
			NetworkNamespaceMode: int32(w.NetworkNamespaceMode),
		},
	}, nil
}
