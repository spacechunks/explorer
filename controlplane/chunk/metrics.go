/*
A basic matchmaking service for the Chunk Explorer.
Copyright (C) 2026 Yannic Rieger <oss@76k.io>

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
package chunk

import (
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type metrics struct {
	createdChunksCount metric.Int64Counter
}

func initMetrics() (metrics, error) {
	meter := otel.Meter("github.com/spacechunks/explorer/controlplane/chunk")

	createdCount, err := meter.Int64Counter(
		"explorer.control_plane.chunk.created.count",
		metric.WithDescription("Total number of chunks created"),
	)
	if err != nil {
		return metrics{}, fmt.Errorf("chunk created counter: %w", err)
	}

	return metrics{
		createdChunksCount: createdCount,
	}, nil
}
