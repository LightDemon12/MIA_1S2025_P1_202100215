package DiskManager

import (
	"MIA_P1/backend/utils"
	"fmt"
)

// GetMountedPartitionByID busca una partición montada por su ID
func GetMountedPartitionByID(id string) (utils.MountedPartition, error) {
	for _, mp := range utils.MountedPartitions {
		if mp.ID == id {
			return mp, nil
		}
	}
	return utils.MountedPartition{}, fmt.Errorf("no se encontró una partición montada con ID '%s'", id)
}
