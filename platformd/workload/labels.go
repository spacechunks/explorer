package workload

import instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"

const (
	LabelWorkloadID   = "explorer.chunks.cloud/workload-id"
	LabelWorkloadName = "explorer.chunks.cloud/workload-name"
	LabelWorkloadType = "explorer.chunks.cloud/workload-type"

	LabelChunkID   = "explorer.chunks.cloud/chunk-id"
	LabelChunkName = "explorer.chunks.cloud/chunk-name"

	LabelFlavorVersionID = "explorer.chunks.cloud/flavor-version-id"

	LabelWorkloadPort = "explorer.chunks.cloud/workload-port"
)

// SystemWorkloadLabels returns the labels used by system workloads
func SystemWorkloadLabels(name string) map[string]string {
	return map[string]string{
		LabelWorkloadName: name,
		LabelWorkloadType: "system",
	}
}

func InstanceLabels(instance *instancev1alpha1.Instance) map[string]string {
	return map[string]string{
		LabelWorkloadID:      instance.GetId(),
		LabelWorkloadType:    "instance",
		LabelChunkID:         instance.GetChunk().GetId(),
		LabelChunkName:       instance.GetChunk().GetName(),
		LabelFlavorVersionID: instance.FlavorVersion.Id,
	}
}
