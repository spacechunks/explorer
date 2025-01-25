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

package functional

import (
	"context"
	"testing"

	workloadv1alpha1 "github.com/spacechunks/platform/api/platformd/workload/v1alpha1"
	"github.com/spacechunks/platform/test/functional/fixture"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunWorkload(t *testing.T) {
	fixture.RunWorkloadAPIFixtures(t)

	var (
		c        = workloadv1alpha1.NewWorkloadServiceClient(fixture.PlatformdClientConn(t))
		expected = fixture.Workload()
	)

	resp, err := c.RunWorkload(context.Background(), &workloadv1alpha1.RunWorkloadRequest{
		Name:                 expected.Name,
		Image:                expected.Image,
		Namespace:            expected.Namespace,
		Hostname:             expected.Hostname,
		Labels:               expected.Labels,
		NetworkNamespaceMode: expected.NetworkNamespaceMode,
	})
	require.NoError(t, err)

	assert.Equal(t, expected.Name, resp.Workload.Name)
	assert.Equal(t, expected.Image, resp.Workload.Image)
	assert.Equal(t, expected.Namespace, resp.Workload.Namespace)
	assert.Equal(t, expected.Hostname, resp.Workload.Hostname)
	assert.Equal(t, expected.Labels, resp.Workload.Labels)
	assert.Equal(t, expected.NetworkNamespaceMode, resp.Workload.NetworkNamespaceMode)
	assert.NotEqual(t, 0, resp.Workload.Port) // makes sure we actually have a valid port set
}

func TestWorkloadStatus(t *testing.T) {
	fixture.RunWorkloadAPIFixtures(t)

	var (
		c        = workloadv1alpha1.NewWorkloadServiceClient(fixture.PlatformdClientConn(t))
		expected = fixture.Workload()
	)

	runResp, err := c.RunWorkload(context.Background(), &workloadv1alpha1.RunWorkloadRequest{
		Name:                 expected.Name,
		Image:                expected.Image,
		Namespace:            expected.Namespace,
		Hostname:             expected.Hostname,
		Labels:               expected.Labels,
		NetworkNamespaceMode: expected.NetworkNamespaceMode,
	})
	require.NoError(t, err)

	statusResp, err := c.WorkloadStatus(context.Background(), &workloadv1alpha1.WorkloadStatusRequest{
		Id: runResp.Workload.Id,
	})
	require.NoError(t, err)

	require.Equal(t, runResp.Workload.Port, statusResp.Status.Port)
}
