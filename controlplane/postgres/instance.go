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
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/spacechunks/explorer/controlplane/chunk"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/instance"
	"github.com/spacechunks/explorer/controlplane/postgres/query"
	"github.com/spacechunks/explorer/internal/ptr"
)

func (db *DB) CreateInstance(ctx context.Context, ins instance.Instance, nodeID string) (instance.Instance, error) {
	params := query.CreateInstanceParams{
		ID:              ins.ID,
		ChunkID:         ins.Chunk.ID,
		FlavorVersionID: ins.FlavorVersion.ID,
		NodeID:          nodeID,
		State:           query.InstanceState(ins.State),
		CreatedAt:       ins.CreatedAt,
		UpdatedAt:       ins.UpdatedAt,
	}

	var ret instance.Instance
	if err := db.doTX(ctx, func(tx pgx.Tx, q *query.Queries) error {
		if err := q.CreateInstance(ctx, params); err != nil {
			return fmt.Errorf("create instance: %w", err)
		}

		ins, err := db.getInstanceByID(ctx, q, params.ID)
		if err != nil {
			return fmt.Errorf("get instance: %w", err)
		}

		ret = ins
		return nil
	}); err != nil {
		return instance.Instance{}, err
	}

	return ret, nil
}

func (db *DB) ListInstances(ctx context.Context) ([]instance.Instance, error) {
	var ret []instance.Instance
	if err := db.do(ctx, func(q *query.Queries) error {
		rows, err := q.ListInstances(ctx)
		if err != nil {
			return err
		}

		m := make(map[string][]query.ListInstancesRow)
		for _, r := range rows {
			m[r.ID] = append(m[r.ID], r)
		}

		ret = make([]instance.Instance, 0, len(m))

		for _, v := range m {
			// we retrieve multiple rows when we call GetInstance
			// chunk data and instance data will stay the same, what
			// will change is the flavor data. there will be one row
			// for each flavor the chunk has.
			//
			// so it is safe that we use the first row here, because
			// the data will stay the same.
			row := v[0]

			// instance port is intentionally left out, because it will not be
			// known beforehand atm, thus it will always be nil when creating.
			i := instance.Instance{
				ID:        row.ID,
				Address:   row.Address,
				State:     instance.State(row.State),
				CreatedAt: row.CreatedAt.UTC(),
				UpdatedAt: row.UpdatedAt.UTC(),
				Chunk: chunk.Chunk{
					ID:          row.ID_3,
					Name:        row.Name,
					Description: row.Description,
					Tags:        row.Tags,
					CreatedAt:   row.CreatedAt_3.UTC(),
					UpdatedAt:   row.UpdatedAt_2.UTC(),
				},
				FlavorVersion: chunk.FlavorVersion{
					ID:         row.ID_2,
					Version:    row.Version,
					Hash:       row.Hash,
					ChangeHash: row.ChangeHash,

					// FIXME: for now those are not needed anywhere, so they are not included in the query
					FileHashes: nil,

					FilesUploaded: row.FilesUploaded,
					BuildStatus:   chunk.BuildStatus(row.BuildStatus),
					CreatedAt:     row.CreatedAt_2.UTC(),
				},
			}

			flavors := make([]chunk.Flavor, 0, len(rows))
			for _, instanceRow := range v {
				f := chunk.Flavor{
					ID:        instanceRow.ID_4,
					Name:      instanceRow.Name_2,
					CreatedAt: instanceRow.CreatedAt_4.UTC(),
					UpdatedAt: instanceRow.UpdatedAt_3.UTC(),
				}
				flavors = append(flavors, f)
			}

			sort.Slice(flavors, func(i, j int) bool {
				// the latest flavor will be the first entry in the slice
				return flavors[i].CreatedAt.Before(flavors[j].CreatedAt)
			})

			i.Chunk.Flavors = flavors
			ret = append(ret, i)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (db *DB) GetInstanceByID(ctx context.Context, id string) (instance.Instance, error) {
	var ret instance.Instance
	if err := db.do(ctx, func(q *query.Queries) error {
		ins, err := db.getInstanceByID(ctx, q, id)
		if err != nil {
			return err
		}
		ret = ins
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
					Name:        row.Name,
					Description: row.Description,
					Tags:        row.Tags,
					CreatedAt:   row.CreatedAt_3.UTC(),
					UpdatedAt:   row.UpdatedAt_2.UTC(),
				},
				FlavorVersion: chunk.FlavorVersion{
					ID:            row.ID_2,
					Version:       row.Version,
					Hash:          row.Hash,
					ChangeHash:    row.ChangeHash,
					FileHashes:    nil,
					FilesUploaded: row.FilesUploaded,
					BuildStatus:   chunk.BuildStatus(row.BuildStatus),
					CreatedAt:     row.CreatedAt_2.UTC(),
				},
				Address:   row.Address,
				State:     instance.State(row.State),
				Port:      port,
				CreatedAt: row.CreatedAt.UTC(),
				UpdatedAt: row.UpdatedAt.UTC(),
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
			ID:    report.InstanceID,
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

func (db *DB) getInstanceByID(ctx context.Context, q *query.Queries, id string) (instance.Instance, error) {
	rows, err := q.GetInstance(ctx, id)
	if err != nil {
		return instance.Instance{}, err
	}

	if len(rows) == 0 {
		return instance.Instance{}, apierrs.ErrInstanceNotFound
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
	ret := instance.Instance{
		ID:        row.ID,
		Address:   row.Address,
		State:     instance.State(row.State),
		CreatedAt: row.CreatedAt.UTC(),
		UpdatedAt: row.UpdatedAt.UTC(),
		Chunk: chunk.Chunk{
			ID:          row.ID_3,
			Name:        row.Name,
			Description: row.Description,
			Tags:        row.Tags,
			CreatedAt:   row.CreatedAt_3.UTC(),
			UpdatedAt:   row.UpdatedAt_2.UTC(),
		},
		FlavorVersion: chunk.FlavorVersion{
			ID:            row.ID_2,
			Version:       row.Version,
			Hash:          row.Hash,
			ChangeHash:    row.ChangeHash,
			FileHashes:    nil,
			FilesUploaded: row.FilesUploaded,
			BuildStatus:   chunk.BuildStatus(row.BuildStatus),
			CreatedAt:     row.CreatedAt_2.UTC(),
		},
	}

	var port *uint16
	if row.Port != nil {
		port = ptr.Pointer(uint16(*row.Port))
	}

	ret.Port = port

	flavors := make([]chunk.Flavor, 0, len(rows))
	for _, instanceRow := range rows {
		f := chunk.Flavor{
			ID:        instanceRow.ID_2,
			Name:      instanceRow.Name_2,
			CreatedAt: instanceRow.CreatedAt_2.UTC(),
			UpdatedAt: instanceRow.UpdatedAt_2.UTC(),
		}

		flavors = append(flavors, f)
	}

	sort.Slice(flavors, func(i, j int) bool {
		// the latest flavor will be the first entry in the slice
		return flavors[i].CreatedAt.Before(flavors[j].CreatedAt)
	})

	ret.Chunk.Flavors = flavors
	return ret, nil
}
