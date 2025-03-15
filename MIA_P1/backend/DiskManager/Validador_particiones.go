package DiskManager

import "fmt"

type PartitionValidator struct {
	mbr *MBR
}

func NewPartitionValidator(mbr *MBR) *PartitionValidator {
	return &PartitionValidator{mbr: mbr}
}

func (pv *PartitionValidator) ValidateNewPartition(partition *Partition) error {
	primarias, extendida := pv.countPartitions()

	if primarias+extendida >= 4 {
		return fmt.Errorf("no se pueden crear más particiones: límite máximo alcanzado (4)")
	}

	if partition.Type == PARTITION_EXTENDED && extendida > 0 {
		return fmt.Errorf("ya existe una partición extendida en el disco")
	}

	if partition.Type == PARTITION_LOGIC && extendida == 0 {
		return fmt.Errorf("no se puede crear una partición lógica sin una partición extendida")
	}

	return nil
}

func (pv *PartitionValidator) countPartitions() (primarias int, extendida int) {
	for i := 0; i < 4; i++ {
		if pv.mbr.MbrPartitions[i].Status != PARTITION_NOT_MOUNTED {
			if pv.mbr.MbrPartitions[i].Type == PARTITION_EXTENDED {
				extendida++
			} else if pv.mbr.MbrPartitions[i].Type == PARTITION_PRIMARY {
				primarias++
			}
		}
	}
	return
}
