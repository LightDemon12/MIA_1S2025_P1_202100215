package DiskManager

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// GetPartitionStartByte obtiene la posición física de inicio de una partición en el disco
func GetPartitionStartByte(diskPath string, partitionName string) (int64, error) {
	file, err := os.Open(diskPath)
	if err != nil {
		return 0, fmt.Errorf("error abriendo disco: %w", err)
	}
	defer file.Close()

	// Leer el MBR
	mbr := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, mbr); err != nil {
		return 0, fmt.Errorf("error leyendo MBR: %w", err)
	}

	// Buscar la partición por nombre
	for i := 0; i < 4; i++ {
		partition := mbr.MbrPartitions[i]
		name := strings.TrimRight(string(partition.Name[:]), " \x00")

		if name == partitionName {
			return int64(partition.Start), nil
		}

		// Si es una partición extendida, buscar en particiones lógicas
		if partition.Type == PARTITION_EXTENDED {
			logicals, err := getLogicalPartitions(file, int64(partition.Start))
			if err != nil {
				continue
			}

			for _, ebr := range logicals {
				ebrName := strings.TrimRight(string(ebr.Name[:]), " \x00")
				if ebrName == partitionName {
					return int64(ebr.Start), nil
				}
			}
		}
	}

	return 0, fmt.Errorf("no se encontró la partición '%s' en el disco", partitionName)
}
