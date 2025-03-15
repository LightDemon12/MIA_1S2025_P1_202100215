package DiskManager

import (
	"encoding/binary"
	"fmt"
	"os"
)

type PartitionManager struct {
	diskPath  string
	mbr       *MBR
	validator *PartitionValidator
	fit       *PartitionFit
}

func NewPartitionManager(diskPath string) (*PartitionManager, error) {
	file, err := os.OpenFile(diskPath, os.O_RDWR, 0666)
	if err != nil {
		return nil, fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	mbr := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, mbr); err != nil {
		return nil, fmt.Errorf("error leyendo MBR: %v", err)
	}

	return &PartitionManager{
		diskPath:  diskPath,
		mbr:       mbr,
		validator: NewPartitionValidator(mbr),
		fit:       NewPartitionFit(mbr, diskPath),
	}, nil
}

func (pm *PartitionManager) CreatePartition(partition *Partition, unit string) error {
	// Convertir tamaño a bytes
	partition.Size = pm.convertToBytes(partition.Size, unit)

	// Calcular posición correcta desde el disco
	nextStart := pm.calculateNextStartPosition()
	partition.Start = nextStart

	fmt.Printf("Debug: Creando partición %s en posición %d con tamaño %d\n",
		string(partition.Name[:]), partition.Start, partition.Size)

	// Verificar espacio disponible
	if partition.Start+partition.Size > pm.mbr.MbrTamanio {
		return fmt.Errorf("no hay espacio suficiente en el disco")
	}

	// Encontrar slot libre en MBR
	slotIndex := -1
	for i, p := range pm.mbr.MbrPartitions {
		if p.Status == PARTITION_NOT_MOUNTED && p.Size == 0 {
			slotIndex = i
			break
		}
	}

	if slotIndex == -1 {
		return fmt.Errorf("no hay slots libres en el MBR")
	}

	// Actualizar MBR local
	pm.mbr.MbrPartitions[slotIndex] = *partition

	// Escribir cambios al disco
	if err := pm.writePartitionToDisk(partition); err != nil {
		return err
	}

	// Log
	LogMBR(pm.diskPath)

	return nil
}
func (pm *PartitionManager) calculateNextStartPosition() int64 {
	// Leer MBR actual directamente del disco
	file, err := os.OpenFile(pm.diskPath, os.O_RDONLY, 0666)
	if err != nil {
		fmt.Printf("Error abriendo disco: %v\n", err)
		return int64(binary.Size(pm.mbr))
	}
	defer file.Close()

	diskMBR := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, diskMBR); err != nil {
		fmt.Printf("Error leyendo MBR: %v\n", err)
		return int64(binary.Size(pm.mbr))
	}

	// Posición inicial después del MBR
	mbrSize := int64(binary.Size(diskMBR))
	lastEndPosition := mbrSize

	// Mostrar todas las particiones del disco
	fmt.Printf("Debug: Particiones leídas directamente del disco:\n")
	for i, p := range diskMBR.MbrPartitions {
		if p.Size > 0 {
			fmt.Printf("  Partición %d: Start=%d, Size=%d, End=%d, Name=%s\n",
				i+1, p.Start, p.Size, p.Start+p.Size, string(p.Name[:]))

			endPos := p.Start + p.Size
			if endPos > lastEndPosition {
				lastEndPosition = endPos
			}
		}
	}

	fmt.Printf("Debug: Última posición calculada: %d\n", lastEndPosition)
	return lastEndPosition
}
func (pm *PartitionManager) convertToBytes(size int64, unit string) int64 {
	switch unit {
	case "B":
		return size
	case "K":
		return size * 1024
	case "M":
		return size * 1024 * 1024
	default:
		return size * 1024 // Default to KB
	}
}

func (pm *PartitionManager) updateMBR(partition *Partition) error {
	file, err := os.OpenFile(pm.diskPath, os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	if err := binary.Write(file, binary.LittleEndian, pm.mbr); err != nil {
		return fmt.Errorf("error escribiendo MBR: %v", err)
	}

	return nil
}

func (pm *PartitionManager) writePartitionToDisk(partition *Partition) error {
	file, err := os.OpenFile(pm.diskPath, os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	// Escribir MBR completo con todas las particiones
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("error posicionando cursor para MBR: %v", err)
	}

	if err := binary.Write(file, binary.LittleEndian, pm.mbr); err != nil {
		return fmt.Errorf("error escribiendo MBR: %v", err)
	}

	// Reservar espacio para la partición
	zeros := make([]byte, partition.Size)
	if _, err := file.Seek(partition.Start, 0); err != nil {
		return fmt.Errorf("error posicionando cursor para partición: %v", err)
	}

	if _, err := file.Write(zeros); err != nil {
		return fmt.Errorf("error escribiendo espacio para partición: %v", err)
	}

	// Manejar partición extendida
	if partition.Type == PARTITION_EXTENDED {
		ebr := NewEBR()
		ebr.Start = partition.Start

		if _, err := file.Seek(partition.Start, 0); err != nil {
			return fmt.Errorf("error posicionando cursor para EBR: %v", err)
		}

		if err := binary.Write(file, binary.LittleEndian, ebr); err != nil {
			return fmt.Errorf("error escribiendo EBR: %v", err)
		}
	}

	return nil
}

func (pm *PartitionManager) findFreePartitionSlot() int {
	for i, p := range pm.mbr.MbrPartitions {
		if p.Status == PARTITION_NOT_MOUNTED {
			return i
		}
	}
	return -1
}
