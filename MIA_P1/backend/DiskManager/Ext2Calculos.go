package DiskManager

import (
	"math"
	"unsafe"
)

// Constantes para EXT2
const (
	EXT2_SUPERBLOCK_SIZE    = 1024   // Tamaño del superbloque en bytes
	EXT2_DEFAULT_BLOCK_SIZE = 1024   // Tamaño predeterminado de bloque (1KB)
	EXT2_MIN_INODES         = 100    // Mínimo número de inodos para cualquier sistema EXT2
	EXT2_RESERVED_INODES    = 10     // Número de inodos reservados para el sistema
	EXT2_ROOT_INODE         = 2      // Inodo para el directorio raíz
	EXT2_BLOCKS_PER_GROUP   = 8192   // Bloques por grupo
	EXT2_MAGIC              = 0xEF53 // Número mágico para identificar EXT2
)

// EXT2FormatInfo contiene la información calculada para formatear una partición en EXT2
type EXT2FormatInfo struct {
	PartitionSize      int64   // Tamaño de la partición en bytes
	SuperBlockSize     int64   // Tamaño del superbloque
	InodeSize          int64   // Tamaño de cada inodo
	BlockSize          int64   // Tamaño de cada bloque
	InodeCount         int     // Número total de inodos
	BlockCount         int     // Número total de bloques (3 * InodeCount)
	BlocksPerType      int     // Bloques por cada tipo (carpetas, archivos, contenido)
	InodeBitmapSize    int64   // Tamaño del bitmap de inodos en bytes
	BlockBitmapSize    int64   // Tamaño del bitmap de bloques en bytes
	InodeTableSize     int64   // Tamaño de la tabla de inodos en bytes
	DataBlocksSize     int64   // Tamaño de los bloques de datos en bytes
	FreeSpace          int64   // Espacio libre no utilizado en la partición
	UsedPercentage     float64 // Porcentaje de la partición utilizado
	FirstDataBlockAddr int64   // Dirección del primer bloque de datos
}

// CalculateEXT2Format calcula la estructura óptima de EXT2 para una partición
// basándose en las fórmulas proporcionadas:
// tamaño_particion = sizeOf(superblock) + n + 3*n + n*sizeOf(inodos) + 3*n*sizeOf(block)
// donde n representa el número de inodos a crear
func CalculateEXT2Format(partitionSize int64) *EXT2FormatInfo {
	// Tamaños de estructuras
	superBlockSize := int64(EXT2_SUPERBLOCK_SIZE)
	inodeSize := int64(unsafe.Sizeof(Inode{}))
	blockSize := int64(EXT2_DEFAULT_BLOCK_SIZE)

	// Calcular n basado en la fórmula proporcionada
	// tamaño_particion = sizeOf(superblock) + n + 3*n + n*sizeOf(inodos) + 3*n*sizeOf(block)
	// Despejando n:
	denominator := float64(4 + inodeSize + 3*blockSize)
	n := float64(partitionSize-superBlockSize) / denominator

	// Aplicar floor y garantizar mínimos
	inodeCount := int(math.Floor(n))
	if inodeCount < EXT2_MIN_INODES {
		inodeCount = EXT2_MIN_INODES
	}

	// Calcular bloques (3 tipos: carpetas, archivos, contenido)
	blocksPerType := inodeCount
	blockCount := 3 * blocksPerType

	// Calcular tamaños de cada sección
	inodeBitmapSizeInBits := inodeCount
	inodeBitmapSize := int64(math.Ceil(float64(inodeBitmapSizeInBits) / 8.0)) // Convertir bits a bytes

	blockBitmapSizeInBits := blockCount
	blockBitmapSize := int64(math.Ceil(float64(blockBitmapSizeInBits) / 8.0)) // Convertir bits a bytes

	inodeTableSize := int64(inodeCount) * inodeSize
	dataBlocksSize := int64(blockCount) * blockSize

	// Cálculo de espacio total utilizado
	totalUsed := superBlockSize + inodeBitmapSize + blockBitmapSize + inodeTableSize + dataBlocksSize
	freeSpace := partitionSize - totalUsed
	usedPercentage := (float64(totalUsed) / float64(partitionSize)) * 100.0

	// Calcular la dirección del primer bloque de datos
	firstDataBlockAddr := superBlockSize + inodeBitmapSize + blockBitmapSize + inodeTableSize

	// Crear y devolver la estructura de información
	return &EXT2FormatInfo{
		PartitionSize:      partitionSize,
		SuperBlockSize:     superBlockSize,
		InodeSize:          inodeSize,
		BlockSize:          blockSize,
		InodeCount:         inodeCount,
		BlockCount:         blockCount,
		BlocksPerType:      blocksPerType,
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
	// Verificar que haya suficiente espacio para las estructuras mínimas
	minSize := int64(EXT2_SUPERBLOCK_SIZE) +
		int64(math.Ceil(float64(EXT2_MIN_INODES)/8.0)) + // Bitmap inodos
		int64(math.Ceil(float64(EXT2_MIN_INODES*3)/8.0)) + // Bitmap bloques
		int64(EXT2_MIN_INODES)*info.InodeSize +
		int64(EXT2_MIN_INODES*3)*info.BlockSize

	if info.PartitionSize < minSize {
		return false
	}

	// Verificar que el porcentaje de uso sea razonable
	if info.UsedPercentage > 99.9 || info.UsedPercentage < 50.0 {
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
		"partition_size":  info.PartitionSize,
		"superblock_size": info.SuperBlockSize,
		"inode_count":     info.InodeCount,
		"block_count":     info.BlockCount,
		"inode_size":      info.InodeSize,
		"block_size":      info.BlockSize,
		"used_space":      info.PartitionSize - info.FreeSpace,
		"free_space":      info.FreeSpace,
		"used_percentage": info.UsedPercentage,
		"blocks_per_type": info.BlocksPerType,
	}
}
