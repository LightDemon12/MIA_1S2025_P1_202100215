package DiskManager

import (
	"math"
)

// Constantes fundamentales para EXT2
const (
	SUPERBLOCK_SIZE      = 1024   // Tamaño del superbloque en bytes
	BLOCK_SIZE           = 64     // Tamaño de cada bloque
	EXT2_MAGIC           = 0xEF53 // Número mágico para identificar EXT2
	EXT2_RESERVED_INODES = 3      // Inodos reservados (0-10)
)

// EXT2FormatInfo contiene la información calculada para formatear una partición en EXT2
type EXT2FormatInfo struct {
	PartitionSize      int64   // Tamaño de la partición en bytes
	SuperBlockSize     int64   // Tamaño del superbloque
	InodeSize          int64   // Tamaño de cada inodo
	BlockSize          int64   // Tamaño de cada bloque
	InodeCount         int     // Número de inodos (n)
	BlockCount         int     // Número de bloques (3n)
	InodeBitmapSize    int64   // Tamaño del bitmap de inodos en bytes (n)
	BlockBitmapSize    int64   // Tamaño del bitmap de bloques en bytes (3n)
	InodeTableSize     int64   // Tamaño de la tabla de inodos (n * INODE_SIZE)
	DataBlocksSize     int64   // Tamaño de bloques de datos (3n * BLOCK_SIZE)
	FreeSpace          int64   // Espacio libre restante
	UsedPercentage     float64 // Porcentaje utilizado
	FirstDataBlockAddr int64   // Dirección del primer bloque de datos
}

// CalculateEXT2Format calcula la estructura según la fórmula:
// tamaño_particion = sizeOf(superblock) + n + 3n + n*sizeOf(inodos) + 3n*sizeOf(block)
func CalculateEXT2Format(partitionSize int64) *EXT2FormatInfo {
	// Despejar n de la ecuación
	// partitionSize = SUPERBLOCK_SIZE + n + 3n + n*INODE_SIZE + 3n*BLOCK_SIZE
	// partitionSize = SUPERBLOCK_SIZE + n(1 + 3 + INODE_SIZE + 3*BLOCK_SIZE)
	// n = (partitionSize - SUPERBLOCK_SIZE) / (1 + 3 + INODE_SIZE + 3*BLOCK_SIZE)

	// Según la especificación: 1 byte por inodo y 1 byte por bloque en los bitmaps
	divisor := float64(1 + 3 + INODE_SIZE + 3*BLOCK_SIZE)
	n := float64(partitionSize-SUPERBLOCK_SIZE) / divisor

	// Aplicar floor para obtener n, según especificación
	inodeCount := int(math.Floor(n))

	// Calcular bloques (siempre 3 veces el número de inodos)
	blockCount := inodeCount * 3

	// Calcular tamaños según especificación:
	// - Bitmap de inodos: 1 byte por inodo (no 1 bit)
	// - Bitmap de bloques: 1 byte por bloque (no 1 bit)
	inodeBitmapSize := int64(inodeCount)
	blockBitmapSize := int64(blockCount)
	inodeTableSize := int64(inodeCount) * INODE_SIZE
	dataBlocksSize := int64(blockCount) * BLOCK_SIZE

	// Calcular espacio total usado
	totalUsed := int64(SUPERBLOCK_SIZE) + inodeBitmapSize + blockBitmapSize +
		inodeTableSize + dataBlocksSize
	freeSpace := partitionSize - totalUsed
	usedPercentage := (float64(totalUsed) / float64(partitionSize)) * 100.0

	// Calcular dirección del primer bloque de datos
	firstDataBlockAddr := int64(SUPERBLOCK_SIZE) + inodeBitmapSize +
		blockBitmapSize + inodeTableSize

	return &EXT2FormatInfo{
		PartitionSize:      partitionSize,
		SuperBlockSize:     int64(SUPERBLOCK_SIZE),
		InodeSize:          INODE_SIZE,
		BlockSize:          BLOCK_SIZE,
		InodeCount:         inodeCount,
		BlockCount:         blockCount,
		InodeBitmapSize:    inodeBitmapSize,
		BlockBitmapSize:    blockBitmapSize,
		InodeTableSize:     inodeTableSize,
		DataBlocksSize:     dataBlocksSize,
		FreeSpace:          freeSpace,
		UsedPercentage:     usedPercentage,
		FirstDataBlockAddr: firstDataBlockAddr,
	}
}

// ValidateEXT2Format verifica si el formato propuesto es válido
func ValidateEXT2Format(info *EXT2FormatInfo) bool {
	// Verificar que haya espacio para las estructuras mínimas
	minSize := int64(SUPERBLOCK_SIZE) +
		int64(EXT2_RESERVED_INODES) + // Bitmap inodos (1 byte por inodo)
		int64(EXT2_RESERVED_INODES*3) + // Bitmap bloques (1 byte por bloque)
		int64(EXT2_RESERVED_INODES)*INODE_SIZE +
		int64(EXT2_RESERVED_INODES*3)*BLOCK_SIZE

	if info.PartitionSize < minSize {
		return false
	}

	// Verificar que el formato sea válido
	if info.InodeCount < EXT2_RESERVED_INODES {
		return false
	}

	// Verificar que haya espacio suficiente para las estructuras
	if info.FreeSpace < 0 {
		return false
	}

	return true
}

// GetInodesAndBlocksStart calcula las direcciones de inicio de cada sección
func GetInodesAndBlocksStart(info *EXT2FormatInfo) (inodeBitmapStart, blockBitmapStart, inodeTableStart, dataBlocksStart int64) {
	inodeBitmapStart = info.SuperBlockSize
	blockBitmapStart = inodeBitmapStart + info.InodeBitmapSize
	inodeTableStart = blockBitmapStart + info.BlockBitmapSize
	dataBlocksStart = inodeTableStart + info.InodeTableSize
	return
}

// CalcInodeAddress calcula la dirección de un inodo específico
func CalcInodeAddress(info *EXT2FormatInfo, inodeNum int) int64 {
	if inodeNum < 0 || inodeNum >= info.InodeCount {
		return -1 // Error: número de inodo fuera de rango
	}

	_, _, inodeTableStart, _ := GetInodesAndBlocksStart(info)
	return inodeTableStart + int64(inodeNum)*info.InodeSize
}

// CalcBlockAddress calcula la dirección de un bloque específico
func CalcBlockAddress(info *EXT2FormatInfo, blockNum int) int64 {
	if blockNum < 0 || blockNum >= info.BlockCount {
		return -1 // Error: número de bloque fuera de rango
	}

	_, _, _, dataBlocksStart := GetInodesAndBlocksStart(info)
	return dataBlocksStart + int64(blockNum)*info.BlockSize
}

// GetFormattingStats genera estadísticas del formateo para reportes
func GetFormattingStats(info *EXT2FormatInfo) map[string]interface{} {
	return map[string]interface{}{
		"partition_size":    info.PartitionSize,
		"superblock_size":   info.SuperBlockSize,
		"inode_count":       info.InodeCount,
		"block_count":       info.BlockCount,
		"inode_size":        info.InodeSize,
		"block_size":        info.BlockSize,
		"inode_bitmap_size": info.InodeBitmapSize,
		"block_bitmap_size": info.BlockBitmapSize,
		"inode_table_size":  info.InodeTableSize,
		"data_blocks_size":  info.DataBlocksSize,
		"free_space":        info.FreeSpace,
		"used_percentage":   info.UsedPercentage,
		"first_data_block":  info.FirstDataBlockAddr,
	}
}
