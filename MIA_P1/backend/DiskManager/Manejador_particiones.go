package DiskManager

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
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
		validator: NewPartitionValidator(mbr, diskPath),
		fit:       NewPartitionFit(mbr, diskPath),
	}, nil
}

func (pm *PartitionManager) CreatePartition(partition *Partition, unit string) error {
	// 1. Convertir tamaño a bytes
	partition.Size = pm.convertToBytes(partition.Size, unit)

	// 2. Leer MBR actualizado del disco
	file, err := os.OpenFile(pm.diskPath, os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	// Leer MBR actual
	currentMBR := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, currentMBR); err != nil {
		return fmt.Errorf("error leyendo MBR: %v", err)
	}

	// 3. Validar reglas de particiones
	primarias := 0
	extendidas := 0

	// Mostrar estado actual
	fmt.Printf("Debug: Particiones actuales en disco:\n")
	for i, p := range currentMBR.MbrPartitions {
		if p.Size > 0 {
			fmt.Printf("  Partición %d: Type=%c, Name=%s\n",
				i+1, p.Type, strings.TrimRight(string(p.Name[:]), " "))

			if p.Type == PARTITION_PRIMARY {
				primarias++
			} else if p.Type == PARTITION_EXTENDED {
				extendidas++
			}
		}
	}

	// 4. Validaciones
	fmt.Printf("Debug: Conteo final - Primarias: %d, Extendidas: %d\n", primarias, extendidas)

	// Validar límite de particiones
	if primarias+extendidas >= 4 {
		return fmt.Errorf("no se pueden crear más particiones: límite máximo alcanzado (4)")
	}

	// Validar partición extendida única
	if partition.Type == PARTITION_EXTENDED && extendidas > 0 {
		return fmt.Errorf("ya existe una partición extendida en el disco")
	}

	// Validar nombre único
	partitionName := strings.TrimRight(string(partition.Name[:]), " ")
	for _, p := range currentMBR.MbrPartitions {
		if p.Size > 0 {
			existingName := strings.TrimRight(string(p.Name[:]), " ")
			if existingName == partitionName {
				return fmt.Errorf("ya existe una partición con el nombre '%s'", partitionName)
			}
		}
	}

	// 5. Buscar espacio según el algoritmo de ajuste
	if err := pm.fit.FindPartitionSpace(partition); err != nil {
		return err
	}

	// 6. Encontrar slot libre
	slotIndex := -1
	for i, p := range currentMBR.MbrPartitions {
		if p.Size == 0 {
			slotIndex = i
			break
		}
	}

	if slotIndex == -1 {
		return fmt.Errorf("no hay slots libres en el MBR")
	}

	// 7. Actualizar MBR con la nueva partición
	currentMBR.MbrPartitions[slotIndex] = *partition

	// 8. Volver a principio y escribir MBR actualizado
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("error posicionando cursor para escribir MBR: %v", err)
	}

	if err := binary.Write(file, binary.LittleEndian, currentMBR); err != nil {
		return fmt.Errorf("error escribiendo MBR: %v", err)
	}

	// 9. Escribir espacio para la partición
	zeros := make([]byte, partition.Size)
	if _, err := file.Seek(partition.Start, 0); err != nil {
		return fmt.Errorf("error posicionando cursor para partición: %v", err)
	}

	if _, err := file.Write(zeros); err != nil {
		return fmt.Errorf("error escribiendo espacio para partición: %v", err)
	}

	// 10. Log
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
