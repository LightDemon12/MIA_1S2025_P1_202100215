package DiskManager

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

type PartitionValidator struct {
	mbr      *MBR
	diskPath string
}

func NewPartitionValidator(mbr *MBR, diskPath string) *PartitionValidator {
	return &PartitionValidator{
		mbr:      mbr,
		diskPath: diskPath,
	}
}
func (pv *PartitionValidator) ValidateNewPartition(partition *Partition) error {
	// Primero leemos el MBR actual del disco para tener datos actualizados
	file, err := os.OpenFile(pv.diskPath, os.O_RDONLY, 0666)
	if err != nil {
		return fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	currentMBR := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, currentMBR); err != nil {
		return fmt.Errorf("error leyendo MBR: %v", err)
	}

	// Contar particiones actuales
	primarias := 0
	extendida := 0

	// Mostrar las particiones que se están detectando
	fmt.Printf("Debug: Particiones en disco detectadas:\n")
	for i, p := range currentMBR.MbrPartitions {
		if p.Size > 0 {
			fmt.Printf("  Partición %d: Type=%c, Size=%d, Name=%s\n",
				i+1, p.Type, p.Size, strings.TrimRight(string(p.Name[:]), " "))

			if p.Type == PARTITION_EXTENDED {
				extendida++
			} else if p.Type == PARTITION_PRIMARY {
				primarias++
			}
		}
	}

	fmt.Printf("Debug Validador: Particiones primarias=%d, extendida=%d\n", primarias, extendida)

	// Validar límite de particiones
	if primarias+extendida >= 4 {
		return fmt.Errorf("no se pueden crear más particiones: límite máximo alcanzado (4)")
	}

	// Validar particiones extendidas
	if partition.Type == PARTITION_EXTENDED && extendida > 0 {
		return fmt.Errorf("ya existe una partición extendida en el disco")
	}

	// Validar particiones lógicas
	if partition.Type == PARTITION_LOGIC && extendida == 0 {
		return fmt.Errorf("no se puede crear una partición lógica sin una partición extendida")
	}

	// Validar que el nombre no se repita
	partitionName := string(partition.Name[:])
	partitionName = strings.TrimRight(partitionName, " ")

	for _, p := range currentMBR.MbrPartitions {
		if p.Status != PARTITION_NOT_MOUNTED && p.Size > 0 {
			existingName := string(p.Name[:])
			existingName = strings.TrimRight(existingName, " ")

			if existingName == partitionName {
				return fmt.Errorf("ya existe una partición con el nombre '%s' en este disco", partitionName)
			}
		}
	}

	// Validar tamaño
	if partition.Size <= 0 {
		return fmt.Errorf("el tamaño de la partición debe ser mayor que cero")
	}

	// Validar que el tamaño no exceda el disco
	if partition.Size > currentMBR.MbrTamanio {
		return fmt.Errorf("el tamaño de la partición excede el espacio disponible en el disco")
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
