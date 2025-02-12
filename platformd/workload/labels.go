package workload

const (
	LabelID   = "platform.chunks.cloud/workload-id"
	LabelName = "platform.chunks.cloud/workload-name"
	LabelType = "platform.chunks.cloud/workload-type"
)

// SystemWorkloadLabels returns the labels used by system workloads
func SystemWorkloadLabels(name string) map[string]string {
	return map[string]string{
		LabelName: name,
		LabelType: "system",
	}
}
