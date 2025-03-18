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

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/instance"
	"github.com/spacechunks/explorer/controlplane/postgres/query"
	"github.com/spacechunks/explorer/internal/ptr"
)

type instanceParams struct {
	create query.CreateInstanceParams
}

func createInstanceParams(nodeID string, instance instance.Instance) (instanceParams, error) {
	createdAt := pgtype.Timestamptz{}
	if err := createdAt.Scan(instance.CreatedAt); err != nil {
		return instanceParams{}, fmt.Errorf("scan updated at: %w", err)
	}

	updatedAt := pgtype.Timestamptz{}
	if err := updatedAt.Scan(instance.UpdatedAt); err != nil {
		return instanceParams{}, fmt.Errorf("scan updated at: %w", err)
	}

	return instanceParams{
		create: query.CreateInstanceParams{
			ID:        instance.ID,
			ChunkID:   instance.Chunk.ID,
			FlavorID:  instance.ChunkFlavor.ID,
			NodeID:    nodeID,
			State:     query.InstanceState(instance.State),
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}, nil
}

func (db *DB) CreateInstance(ctx context.Context, ins instance.Instance, nodeID string) (instance.Instance, error) {
	params, err := createInstanceParams(nodeID, ins)
	if err != nil {
		return instance.Instance{}, fmt.Errorf("instance params: %w", err)
	}

	var ret instance.Instance
	if err := db.doTX(ctx, func(q *query.Queries) error {
		if err := q.CreateInstance(ctx, params.create); err != nil {
			return fmt.Errorf("create instance: %w", err)
		}

		rows, err := q.GetInstance(ctx, params.create.ID)
		if err != nil {
			return fmt.Errorf("get instance: %w", err)
		}

		// we retrieve multiple rows when we call GetInstance
		// chunk data and instance data will stay the same, what
		// will change is the flavor data. there will be one row
		// for each flavor the chunk has.
		//
		// so it is safe that we use the first row here, because
		// the data will stay the same.
		row := rows[0]

		// instance port is intentionally left out, because it will not be
		// known beforehand atm, thus it will always be nil when creating.
		ret = instance.Instance{
			ID:        row.ID,
			Address:   row.Address,
			State:     instance.State(row.State),
			CreatedAt: row.CreatedAt.Time.UTC(),
			UpdatedAt: row.UpdatedAt.Time.UTC(),
			Chunk: chunk.Chunk{
				ID:          row.ID_3,
				Name:        row.Name_2,
				Description: row.Description,
				Tags:        row.Tags,
				CreatedAt:   row.CreatedAt_3.Time.UTC(),
				UpdatedAt:   row.UpdatedAt_3.Time.UTC(),
			},
		}

		flavors := make([]chunk.Flavor, 0, len(rows))
		for _, instanceRow := range rows {
			f := chunk.Flavor{
				ID:        instanceRow.ID_2,
				Name:      instanceRow.Name,
				CreatedAt: instanceRow.CreatedAt_2.Time.UTC(),
				UpdatedAt: instanceRow.UpdatedAt_2.Time.UTC(),
			}

			if instanceRow.FlavorID == f.ID {
				ret.ChunkFlavor = f
			}

			flavors = append(flavors, f)
		}

		ret.Chunk.Flavors = flavors

		return nil
	}); err != nil {
		return instance.Instance{}, err
	}

	return ret, nil
}

func (db *DB) GetInstancesByNodeID(ctx context.Context, nodeID string) ([]instance.Instance, error) {
	ret := make([]instance.Instance, 0)
	if err := db.do(ctx, func(q *query.Queries) error {
		rows, err := q.GetInstancesByNodeID(ctx, nodeID)
		if err != nil {
			return err
		}

		// FIXME: for now it is okay to not return the full chunk object
		// with all flavors, because we don't need it atm. but for consistency
		// purposes it should be considered.

		for _, row := range rows {
			var port *uint16
			if row.Port != nil {
				port = ptr.Pointer(uint16(*row.Port))
			}
			ret = append(ret, instance.Instance{
				ID: row.ID,
				Chunk: chunk.Chunk{
					ID:          row.ID_3,
					Name:        row.Name_2,
					Description: row.Description,
					Tags:        row.Tags,
					CreatedAt:   row.CreatedAt_3.Time.UTC(),
					UpdatedAt:   row.UpdatedAt_3.Time.UTC(),
				},
				ChunkFlavor: chunk.Flavor{
					ID:        row.ID_2,
					Name:      row.Name,
					CreatedAt: row.CreatedAt_2.Time.UTC(),
					UpdatedAt: row.UpdatedAt_2.Time.UTC(),
				},
				Address:   row.Address,
				State:     instance.State(row.State),
				Port:      port,
				CreatedAt: row.CreatedAt.Time.UTC(),
				UpdatedAt: row.UpdatedAt.Time.UTC(),
			})
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

// ApplyStatusReports updates instances rows that are not in [instance.StateDeleted] state.
// all other instances will be removed from the table.
func (db *DB) ApplyStatusReports(ctx context.Context, reports []instance.StatusReport) error {
	var (
		toUpdate = make([]query.BulkUpdateInstanceStateAndPortParams, 0, len(reports))
		toRemove = make([]string, 0)
	)

	for _, report := range reports {
		if report.State == instance.StateDeleted {
			toRemove = append(toRemove, report.InstanceID)
			continue
		}
		toUpdate = append(toUpdate, query.BulkUpdateInstanceStateAndPortParams{
			State: query.InstanceState(report.State),
			Port:  ptr.Pointer(int32(report.Port)),
		})
	}

	// don't even attempt to open a connection to the db
	if len(toRemove) == 0 && len(toUpdate) == 0 {
		return nil
	}

	if err := db.do(context.Background(), func(q *query.Queries) error {
		// always update all rows, even those that will be deleted
		// afterward. when the database or service dies after the
		// update we still have recorded the last state we observed.
		bulkUpdate := q.BulkUpdateInstanceStateAndPort(ctx, toUpdate)
		if err := db.bulkExecAndClose(bulkUpdate); err != nil {
			return fmt.Errorf("bulk update: %w", err)
		}

		if len(toRemove) > 0 {
			bulkDel := q.BulkDeleteInstances(ctx, toRemove)
			if err := db.bulkExecAndClose(bulkDel); err != nil {
				return fmt.Errorf("bulk delete: %w", err)
			}
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
