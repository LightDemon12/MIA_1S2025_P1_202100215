package DiskManager

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
)

type Space struct {
	start int64
	size  int64
}

type PartitionFit struct {
	mbr      *MBR
	diskPath string
}

type PartitionMetadata struct {
	Start int64
	Size  int64
	Used  bool
}

func NewPartitionFit(mbr *MBR, diskPath string) *PartitionFit {
	return &PartitionFit{
		mbr:      mbr,
		diskPath: diskPath,
	}
}

func (pf *PartitionFit) FindPartitionSpace(partition *Partition) error {
	// Abrir archivo para lectura
	file, err := os.OpenFile(pf.diskPath, os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	// Leer MBR actual
	currentMBR := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, currentMBR); err != nil {
		return fmt.Errorf("error leyendo MBR: %v", err)
	}

	// Buscar la última posición ocupada
	var lastEndPosition int64 = int64(binary.Size(currentMBR))

	fmt.Printf("Debug: Particiones actuales:\n")
	for i, p := range currentMBR.MbrPartitions {
		if p.Status != PARTITION_NOT_MOUNTED && p.Size > 0 {
			fmt.Printf("  Partición %d: Start=%d, Size=%d, End=%d\n",
				i+1, p.Start, p.Size, p.Start+p.Size)
			if p.Start+p.Size > lastEndPosition {
				lastEndPosition = p.Start + p.Size
			}
		}
	}

	// Asignar nueva posición después de la última partición
	partition.Start = lastEndPosition
	fmt.Printf("Debug: Nueva partición será colocada en Start=%d\n", lastEndPosition)

	// Verificar espacio disponible
	if partition.Start+partition.Size > currentMBR.MbrTamanio {
		return fmt.Errorf("no hay espacio suficiente en el disco")
	}

	return nil
}

func (pf *PartitionFit) calculateNextStartPosition(mbr *MBR) int64 {
	mbrSize := int64(binary.Size(mbr))
	nextStart := mbrSize

	// Verificar cada partición en el MBR
	for _, p := range mbr.MbrPartitions {
		if p.Status != PARTITION_NOT_MOUNTED && p.Size > 0 {
			// Si la partición está activa y tiene tamaño
			if p.Start == nextStart {
				// Si encontramos una partición que empieza donde queremos empezar
				// movemos la posición después de esta partición
				nextStart = p.Start + p.Size
			}
		}
	}

	fmt.Printf("Debug: MBR Size=%d, Next Start=%d\n", mbrSize, nextStart)
	return nextStart
}

func (pf *PartitionFit) getLastUsedPosition() int64 {
	var lastPosition int64 = 0

	// Obtener particiones activas
	for _, p := range pf.mbr.MbrPartitions {
		if p.Status != PARTITION_NOT_MOUNTED && p.Size > 0 {
			endPosition := p.Start + p.Size
			if endPosition > lastPosition {
				lastPosition = endPosition
			}
		}
	}

	return lastPosition
}

func (pf *PartitionFit) getFreeSpaces() []Space {
	var spaces []Space
	var lastEnd int64 = int64(binary.Size(pf.mbr)) // Start after MBR

	// Get active partitions and sort by start position
	partitions := make([]Partition, 0)
	for _, p := range pf.mbr.MbrPartitions {
		if p.Status != PARTITION_NOT_MOUNTED && p.Size > 0 {
			partitions = append(partitions, p)
		}
	}
	sort.Slice(partitions, func(i, j int) bool {
		return partitions[i].Start < partitions[j].Start
	})

	// Find free spaces between partitions
	for _, p := range partitions {
		if p.Start > lastEnd {
			spaces = append(spaces, Space{
				start: lastEnd,
				size:  p.Start - lastEnd,
			})
		}
		lastEnd = p.Start + p.Size
	}

	// Add remaining space at the end if available
	if lastEnd < pf.mbr.MbrTamanio {
		spaces = append(spaces, Space{
			start: lastEnd,
			size:  pf.mbr.MbrTamanio - lastEnd,
		})
	}

	return spaces
}

func (pf *PartitionFit) getActivePartitions() []Partition {
	partitions := make([]Partition, 0)
	for _, p := range pf.mbr.MbrPartitions {
		if p.Status != PARTITION_NOT_MOUNTED {
			partitions = append(partitions, p)
		}
	}
	sort.Slice(partitions, func(i, j int) bool {
		return partitions[i].Start < partitions[j].Start
	})
	return partitions
}

func (pf *PartitionFit) bestFit(spaces []Space, size int64) *Space {
	var best *Space
	minSize := pf.mbr.MbrTamanio + 1

	for _, space := range spaces {
		if space.size >= size && space.size < minSize {
			minSize = space.size
			spaceCopy := space
			best = &spaceCopy
		}
	}
	return best
}

func (pf *PartitionFit) worstFit(spaces []Space, size int64) *Space {
	var worst *Space
	maxSize := int64(0)

	for _, space := range spaces {
		if space.size >= size && space.size > maxSize {
			maxSize = space.size
			spaceCopy := space
			worst = &spaceCopy
		}
	}
	return worst
}
