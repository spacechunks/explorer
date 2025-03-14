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

type State string

var (
	StateCreating       State = "CREATING"
	StateRunning        State = "RUNNING"
	StateDeleted        State = "STOPPED"
	StateCreationFailed State = "CREATION_FAILED"
)

type HealthStatus string

var (
	Healthy   HealthStatus = "HEALTY"
	Unhealthy HealthStatus = "UNHEALTY"
)

type Status struct {
	State State
	Port  uint16
}

type Workload struct {
	ID     string
	Status Status

	// below map directly to pod fields
	Name      string
	Image     string
	Namespace string
	Hostname  string
	Labels    map[string]string
	// NetworkNamespaceMode as per [runtimev1.NamespaceMode].
	// keeping this value an int32 is intentional, so the workload
	// api does not rely on runtime version specific value mapping,
	// which would be the case if we were defining enum values for each
	// [runtimev1.NamespaceMode] value.
	NetworkNamespaceMode int32
	Mounts               []Mount
	Args                 []string
}

type Mount struct {
	ContainerPath string
	HostPath      string
}
