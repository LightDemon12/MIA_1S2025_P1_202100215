package DiskManager

import (
	"encoding/binary"
	"fmt"
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
	fmt.Printf("\n=== DEBUG: Buscando espacio para partición ===\n")
	fmt.Printf("Tamaño requerido: %d bytes\n", partition.Size)

	spaces := pf.getFreeSpaces(pf.mbr)
	if len(spaces) == 0 {
		return fmt.Errorf("no hay espacios libres disponibles")
	}

	var selectedSpace *Space
	switch partition.Fit {
	case FIT_BEST:
		selectedSpace = pf.bestFit(spaces, partition.Size)
	case FIT_WORST:
		selectedSpace = pf.worstFit(spaces, partition.Size)
	default:
		selectedSpace = pf.firstFit(spaces, partition.Size)
	}

	if selectedSpace == nil {
		return fmt.Errorf("no se encontró espacio suficiente")
	}

	partition.Start = selectedSpace.start

	// Validar la posición seleccionada
	if err := pf.validatePartitionPosition(partition); err != nil {
		return err
	}

	fmt.Printf("Posición seleccionada: Start=%d, Size=%d, End=%d\n",
		partition.Start, partition.Size, partition.Start+partition.Size)

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
	mbrSize := int64(binary.Size(mbr))

	// Estructura para mantener track de los espacios reservados
	type reservedSpace struct {
		start int64
		end   int64
		inUse bool
	}

	var reserved []reservedSpace

	// 1. Agregar el MBR como espacio reservado
	reserved = append(reserved, reservedSpace{
		start: 0,
		end:   mbrSize,
		inUse: true,
	})

	fmt.Printf("\n=== DEBUG: Espacios Reservados ===\n")
	fmt.Printf("MBR: start=0, end=%d\n", mbrSize)

	// 2. Procesar todas las particiones (activas e inactivas)
	for i, p := range mbr.MbrPartitions {
		if p.Size > 0 { // Si tiene tamaño asignado, está reservado
			reserved = append(reserved, reservedSpace{
				start: p.Start,
				end:   p.Start + p.Size,
				inUse: p.Status == PARTITION_MOUNTED,
			})
			fmt.Printf("Partición %d: start=%d, end=%d, activa=%v\n",
				i+1, p.Start, p.Start+p.Size, p.Status == PARTITION_MOUNTED)
		}
	}

	// 3. Ordenar espacios reservados
	sort.Slice(reserved, func(i, j int) bool {
		return reserved[i].start < reserved[j].start
	})

	// 4. Encontrar espacios verdaderamente libres
	lastEnd := mbrSize
	fmt.Printf("\n=== DEBUG: Espacios Libres ===\n")

	for i := 1; i < len(reserved); i++ {
		current := reserved[i]

		// Verificar si hay espacio entre la última posición y esta
		if current.start > lastEnd {
			space := Space{
				start: lastEnd,
				size:  current.start - lastEnd,
			}
			spaces = append(spaces, space)
			fmt.Printf("Libre: start=%d, size=%d\n", space.start, space.size)
		}

		// Actualizar última posición solo si es mayor
		if current.end > lastEnd {
			lastEnd = current.end
		}
	}

	// 5. Verificar espacio libre al final del disco
	if lastEnd < mbr.MbrTamanio {
		space := Space{
			start: lastEnd,
			size:  mbr.MbrTamanio - lastEnd,
		}
		spaces = append(spaces, space)
		fmt.Printf("Libre final: start=%d, size=%d\n", space.start, space.size)
	}

	return spaces
}

func (pf *PartitionFit) validatePartitionPosition(partition *Partition) error {
	for i, p := range pf.mbr.MbrPartitions {
		if p.Status == PARTITION_MOUNTED && p.Size > 0 {
			// Verificar si hay superposición
			if (partition.Start >= p.Start && partition.Start < p.Start+p.Size) ||
				(partition.Start+partition.Size > p.Start &&
					partition.Start+partition.Size <= p.Start+p.Size) {
				return fmt.Errorf("la partición se superpondría con la partición %d", i+1)
			}
		}
	}
	return nil
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
			return &Space{
				start: space.start,
				size:  size,
			}
		}
	}
	return nil
}

func (pf *PartitionFit) bestFit(spaces []Space, size int64) *Space {
	var best *Space
	bestWaste := int64(-1)

	for _, space := range spaces {
		if space.size >= size {
			waste := space.size - size
			if bestWaste == -1 || waste < bestWaste {
				bestWaste = waste
				best = &Space{
					start: space.start,
					size:  size,
				}
			}
		}
	}
	return best
}

func (pf *PartitionFit) worstFit(spaces []Space, size int64) *Space {
	var worst *Space
	worstWaste := int64(-1)

	for _, space := range spaces {
		if space.size >= size {
			waste := space.size - size
			if waste > worstWaste {
				worstWaste = waste
				worst = &Space{
					start: space.start,
					size:  size,
				}
			}
		}
	}
	return worst
}
