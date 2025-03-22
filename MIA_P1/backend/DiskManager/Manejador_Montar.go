package DiskManager

import (
	"MIA_P1/backend/utils"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

const CARNET = "202100215"

// MountPartition monta una partición y devuelve su ID
func MountPartition(diskPath, partitionName string) (string, error) {
	// Limpiar nombre de la partición de comillas y espacios
	partitionName = strings.Trim(partitionName, "\"")

	fmt.Printf("Debug: Buscando partición '%s' en disco '%s'\n", partitionName, diskPath)

	// 1. Abrir el disco
	file, err := os.OpenFile(diskPath, os.O_RDWR, 0666)
	if err != nil {
		return "", fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	// 2. Leer el MBR
	mbr := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, mbr); err != nil {
		return "", fmt.Errorf("error leyendo MBR: %v", err)
	}

	// Mostrar todas las particiones para depuración
	fmt.Printf("Debug: Particiones encontradas en el disco:\n")
	for i, p := range mbr.MbrPartitions {
		pName := strings.TrimRight(string(p.Name[:]), " \x00")
		if p.Size > 0 {
			fmt.Printf("  Partición %d: Type=%c, Start=%d, Size=%d, Name='%s'\n",
				i+1, p.Type, p.Start, p.Size, pName)
		}
	}

	// 3. Buscar la partición por nombre
	var partition *Partition

	// a. Buscar en particiones primarias y extendida
	for i, p := range mbr.MbrPartitions {
		pName := strings.TrimRight(string(p.Name[:]), " \x00")
		if pName == partitionName && p.Size > 0 {
			fmt.Printf("Debug: Encontrada coincidencia: '%s' = '%s'\n", pName, partitionName)
			if p.Type == PARTITION_PRIMARY {
				partition = &mbr.MbrPartitions[i]
				break
			} else if p.Type == PARTITION_EXTENDED {
				// Si es extendida, no se puede montar directamente
				return "", fmt.Errorf("no se puede montar una partición extendida, solo primarias y lógicas")
			}
		}
	}

	// b. Si no se encontró en primarias, buscar en lógicas
	if partition == nil {
		// Buscar la partición extendida primero
		var extendedPartition *Partition
		for i, p := range mbr.MbrPartitions {
			if p.Type == PARTITION_EXTENDED && p.Size > 0 {
				extendedPartition = &mbr.MbrPartitions[i]
				break
			}
		}

		// Si hay partición extendida, buscar en lógicas
		if extendedPartition != nil {
			// Navegar por los EBRs
			currentPos := extendedPartition.Start
			for currentPos != -1 {
				if _, err := file.Seek(currentPos, 0); err != nil {
					break
				}

				ebr := &EBR{}
				if err := binary.Read(file, binary.LittleEndian, ebr); err != nil {
					break
				}

				ebrName := strings.TrimRight(string(ebr.Name[:]), " ")
				if ebrName == partitionName && ebr.Size > 0 {
					// Encontró la partición lógica

					// 4. Verificar si ya está montada
					for _, mp := range utils.MountedPartitions {
						if mp.DiskPath == diskPath && mp.PartitionName == partitionName {
							return mp.ID, fmt.Errorf("la partición '%s' ya está montada con ID: %s", partitionName, mp.ID)
						}
					}

					// 5. Actualizar el estado de la partición lógica
					ebr.Status = PARTITION_MOUNTED

					// 6. Generar ID para la partición lógica
					letter := utils.GetNextLetter(diskPath)
					number := utils.GetNextPartitionNumber(diskPath, letter)
					id := utils.GenerateID(CARNET, number, letter)

					// 7. Registrar la partición montada
					utils.MountedPartitions = append(utils.MountedPartitions, utils.MountedPartition{
						ID:            id,
						DiskPath:      diskPath,
						PartitionName: partitionName,
						PartitionType: PARTITION_LOGIC,
						Status:        PARTITION_MOUNTED,
						Letter:        letter,
						Number:        number,
					})

					// 8. Escribir el EBR actualizado
					if _, err := file.Seek(currentPos, 0); err != nil {
						return "", fmt.Errorf("error posicionando cursor: %v", err)
					}

					if err := binary.Write(file, binary.LittleEndian, ebr); err != nil {
						return "", fmt.Errorf("error actualizando EBR: %v", err)
					}

					fmt.Printf("Partición lógica '%s' montada exitosamente con ID: %s\n", partitionName, id)
					return id, nil
				}

				// Avanzar al siguiente EBR
				if ebr.Next == -1 {
					break
				}
				currentPos = ebr.Next
			}
		}
	}

	// Si no se encontró la partición
	if partition == nil {
		return "", fmt.Errorf("no se encontró la partición '%s' en el disco", partitionName)
	}

	// 4. Verificar que sea primaria
	if partition.Type != PARTITION_PRIMARY {
		return "", fmt.Errorf("solo se pueden montar particiones primarias y lógicas")
	}

	// 5. Verificar si ya está montada
	for _, mp := range utils.MountedPartitions {
		if mp.DiskPath == diskPath && mp.PartitionName == partitionName {
			return mp.ID, fmt.Errorf("la partición '%s' ya está montada con ID: %s", partitionName, mp.ID)
		}
	}

	// 6. Actualizar el estado de la partición
	partition.Status = PARTITION_MOUNTED

	// 7. Generar ID
	letter := utils.GetNextLetter(diskPath)
	number := utils.GetNextPartitionNumber(diskPath, letter)
	id := utils.GenerateID(CARNET, number, letter)

	// 8. Actualizar correlativo de la partición
	partition.Correlative = int32(number)

	// 9. Registrar la partición montada
	utils.MountedPartitions = append(utils.MountedPartitions, utils.MountedPartition{
		ID:            id,
		DiskPath:      diskPath,
		PartitionName: partitionName,
		PartitionType: PARTITION_PRIMARY,
		Status:        PARTITION_MOUNTED,
		Letter:        letter,
		Number:        number,
	})

	// 10. Guardar cambios en el MBR
	if _, err := file.Seek(0, 0); err != nil {
		return "", fmt.Errorf("error posicionando cursor: %v", err)
	}

	if err := binary.Write(file, binary.LittleEndian, mbr); err != nil {
		return "", fmt.Errorf("error actualizando MBR: %v", err)
	}

	fmt.Printf("Partición primaria '%s' montada exitosamente con ID: %s\n", partitionName, id)
	return id, nil
}

// GetMountedPartitions retorna la lista de particiones montadas
func GetMountedPartitions() []utils.MountedPartition {
	return utils.MountedPartitions
}
func IsPartitionMounted(id string) (bool, error) {
	// Obtener la lista de particiones montadas
	mountedPartitions := GetMountedPartitions()

	// Buscar el ID en la lista
	for _, mp := range mountedPartitions {
		if mp.ID == id {
			return true, nil
		}
	}

	return false, nil
}

// UnmountPartition desmonta una partición por su ID
func UnmountPartition(id string) error {
	// Buscar la partición por ID
	var foundIndex int = -1
	var mountedPartition utils.MountedPartition

	for i, mount := range utils.MountedPartitions {
		if mount.ID == id {
			foundIndex = i
			mountedPartition = mount
			break
		}
	}

	if foundIndex == -1 {
		return fmt.Errorf("no se encontró una partición montada con ID '%s'", id)
	}

	// Actualizar el estado de la partición en el disco
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	if mountedPartition.PartitionType == PARTITION_PRIMARY {
		// Si es primaria, actualizar en el MBR
		mbr := &MBR{}
		if err := binary.Read(file, binary.LittleEndian, mbr); err != nil {
			return fmt.Errorf("error leyendo MBR: %v", err)
		}

		// Buscar la partición por nombre
		for i, p := range mbr.MbrPartitions {
			pName := strings.TrimRight(string(p.Name[:]), " ")
			if pName == mountedPartition.PartitionName {
				mbr.MbrPartitions[i].Status = PARTITION_NOT_MOUNTED
				mbr.MbrPartitions[i].Correlative = -1
				break
			}
		}

		// Guardar cambios
		if _, err := file.Seek(0, 0); err != nil {
			return fmt.Errorf("error posicionando cursor: %v", err)
		}

		if err := binary.Write(file, binary.LittleEndian, mbr); err != nil {
			return fmt.Errorf("error actualizando MBR: %v", err)
		}
	} else if mountedPartition.PartitionType == PARTITION_LOGIC {
		// Si es lógica, buscar el EBR
		mbr := &MBR{}
		if err := binary.Read(file, binary.LittleEndian, mbr); err != nil {
			return fmt.Errorf("error leyendo MBR: %v", err)
		}

		// Buscar la partición extendida
		var extendedPartition *Partition
		for i, p := range mbr.MbrPartitions {
			if p.Type == PARTITION_EXTENDED && p.Size > 0 {
				extendedPartition = &mbr.MbrPartitions[i]
				break
			}
		}

		if extendedPartition != nil {
			// Navegar por los EBRs
			currentPos := extendedPartition.Start
			for currentPos != -1 {
				if _, err := file.Seek(currentPos, 0); err != nil {
					break
				}

				ebr := &EBR{}
				if err := binary.Read(file, binary.LittleEndian, ebr); err != nil {
					break
				}

				ebrName := strings.TrimRight(string(ebr.Name[:]), " ")
				if ebrName == mountedPartition.PartitionName {
					// Actualizar estado
					ebr.Status = PARTITION_NOT_MOUNTED

					// Escribir el EBR actualizado
					if _, err := file.Seek(currentPos, 0); err != nil {
						return fmt.Errorf("error posicionando cursor: %v", err)
					}

					if err := binary.Write(file, binary.LittleEndian, ebr); err != nil {
						return fmt.Errorf("error actualizando EBR: %v", err)
					}

					break
				}

				// Avanzar al siguiente EBR
				if ebr.Next == -1 {
					break
				}
				currentPos = ebr.Next
			}
		}
	}

	// Eliminar la partición del array de montadas
	utils.MountedPartitions = append(utils.MountedPartitions[:foundIndex], utils.MountedPartitions[foundIndex+1:]...)

	return nil
}
