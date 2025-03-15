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
	// Leer MBR actual del disco
	file, err := os.OpenFile(pf.diskPath, os.O_RDONLY, 0666)
	if err != nil {
		return fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	diskMBR := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, diskMBR); err != nil {
		return fmt.Errorf("error leyendo MBR: %v", err)
	}

	// Obtener espacios libres
	spaces := pf.getFreeSpaces(diskMBR)
	fmt.Printf("Debug: Encontrados %d espacios libres\n", len(spaces))
	for i, space := range spaces {
		fmt.Printf("  Espacio %d: Start=%d, Size=%d\n", i+1, space.start, space.size)
	}

	// Buscar posición según el algoritmo de ajuste
	var selectedSpace *Space
	switch partition.Fit {
	case FIT_BEST:
		fmt.Printf("Debug: Usando algoritmo Best Fit\n")
		selectedSpace = pf.bestFit(spaces, partition.Size)
	case FIT_WORST:
		fmt.Printf("Debug: Usando algoritmo Worst Fit\n")
		selectedSpace = pf.worstFit(spaces, partition.Size)
	default: // FIT_FIRST por defecto
		fmt.Printf("Debug: Usando algoritmo First Fit\n")
		selectedSpace = pf.firstFit(spaces, partition.Size)
	}

	if selectedSpace == nil {
		return fmt.Errorf("no se encontró espacio suficiente para la partición")
	}

	// Asignar la posición seleccionada
	partition.Start = selectedSpace.start
	fmt.Printf("Debug: Partición será colocada en Start=%d\n", partition.Start)

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

func (pf *PartitionFit) getFreeSpaces(mbr *MBR) []Space {
	var spaces []Space

	// Tamaño del MBR
	mbrSize := int64(binary.Size(mbr))

	// Obtener particiones activas y ordenarlas
	activePartitions := make([]Partition, 0)
	for _, p := range mbr.MbrPartitions {
		if p.Status != PARTITION_NOT_MOUNTED && p.Size > 0 {
			activePartitions = append(activePartitions, p)
		}
	}

	sort.Slice(activePartitions, func(i, j int) bool {
		return activePartitions[i].Start < activePartitions[j].Start
	})

	// Si no hay particiones, todo el espacio después del MBR está disponible
	if len(activePartitions) == 0 {
		spaces = append(spaces, Space{
			start: mbrSize,
			size:  mbr.MbrTamanio - mbrSize,
		})
		return spaces
	}

	// Verificar espacio entre MBR y primera partición
	if activePartitions[0].Start > mbrSize {
		spaces = append(spaces, Space{
			start: mbrSize,
			size:  activePartitions[0].Start - mbrSize,
		})
	}

	// Buscar espacios entre particiones
	for i := 0; i < len(activePartitions)-1; i++ {
		current := activePartitions[i]
		next := activePartitions[i+1]

		gapStart := current.Start + current.Size
		if gapStart < next.Start {
			spaces = append(spaces, Space{
				start: gapStart,
				size:  next.Start - gapStart,
			})
		}
	}

	// Verificar espacio después de la última partición
	lastPartition := activePartitions[len(activePartitions)-1]
	lastEnd := lastPartition.Start + lastPartition.Size
	if lastEnd < mbr.MbrTamanio {
		spaces = append(spaces, Space{
			start: lastEnd,
			size:  mbr.MbrTamanio - lastEnd,
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

func (pf *PartitionFit) firstFit(spaces []Space, size int64) *Space {
	for _, space := range spaces {
		if space.size >= size {
			return &Space{start: space.start, size: space.size}
		}
	}
	return nil
}

func (pf *PartitionFit) bestFit(spaces []Space, size int64) *Space {
	var best *Space
	bestSize := int64(-1)

	for _, space := range spaces {
		if space.size >= size && (bestSize == -1 || space.size < bestSize) {
			bestSize = space.size
			best = &Space{start: space.start, size: space.size}
		}
	}

	return best
}

func (pf *PartitionFit) worstFit(spaces []Space, size int64) *Space {
	var worst *Space
	worstSize := int64(0)

	for _, space := range spaces {
		if space.size >= size && space.size > worstSize {
			worstSize = space.size
			worst = &Space{start: space.start, size: space.size}
		}
	}

	return worst
}
