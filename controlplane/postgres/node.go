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

package postgres

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/spacechunks/explorer/controlplane/node"
	"github.com/spacechunks/explorer/controlplane/postgres/query"
)

func (db *DB) RandomNode(ctx context.Context) (node.Node, error) {
	var ret node.Node
	if err := db.do(ctx, func(q *query.Queries) error {
		n, err := q.RandomNode(ctx)
		if err != nil {
			return fmt.Errorf("random node: %w", err)
		}

		addrPort, err := netip.ParseAddrPort(n.CheckpointApiEndpoint)
		if err != nil {
			return fmt.Errorf("invalid address port: %w", err)
		}

		ret = node.Node{
			ID:                    n.ID,
			Name:                  n.Name,
			Addr:                  n.Address,
			CheckpointAPIEndpoint: addrPort,
		}

		return nil
	}); err != nil {
		return ret, err
	}

	return ret, nil
}
