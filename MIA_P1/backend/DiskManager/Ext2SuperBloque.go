package DiskManager

import (
	"time"
)

// Constantes para el superbloque EXT2
const (
	EXT2_FILESYSTEM_TYPE = 2 // Tipo de sistema de archivos (2 para EXT2)
)

// SuperBlock representa la estructura del superbloque en el sistema de archivos EXT2
type SuperBlock struct {
	SFilesystemType  int32     // Número identificador del sistema de archivos
	SInodesCount     int32     // Número total de inodos
	SBlocksCount     int32     // Número total de bloques
	SFreeBlocksCount int32     // Número de bloques libres
	SFreeInodesCount int32     // Número de inodos libres
	SMtime           time.Time // Última fecha en que el sistema fue montado
	SUmtime          time.Time // Última fecha en que el sistema fue desmontado
	SMntCount        int32     // Número de veces que se ha montado el sistema
	SMagic           int32     // Valor mágico que identifica al sistema (0xEF53)
	SInodeSize       int32     // Tamaño de cada inodo
	SBlockSize       int32     // Tamaño de cada bloque
	SFirstIno        int32     // Dirección del primer inodo libre
	SFirstBlo        int32     // Dirección del primer bloque libre
	SBmInodeStart    int32     // Inicio del bitmap de inodos
	SBmBlockStart    int32     // Inicio del bitmap de bloques
	SInodeStart      int32     // Inicio de la tabla de inodos
	SBlockStart      int32     // Inicio de la tabla de bloques
	// Padding para asegurar un tamaño total de 1024 bytes
	SPadding [808]byte // Ajustar este valor según sea necesario
}

// NewSuperBlock crea un nuevo superbloque inicializado para EXT2
func NewSuperBlock(
	inodeCount int32,
	blockCount int32,
	inodeSize int32,
	blockSize int32,
	bmInodeStart int32,
	bmBlockStart int32,
	inodeStart int32,
	blockStart int32) *SuperBlock {

	now := time.Now()

	sb := &SuperBlock{
		SFilesystemType:  EXT2_FILESYSTEM_TYPE,
		SInodesCount:     inodeCount,
		SBlocksCount:     blockCount,
		SFreeBlocksCount: blockCount,  // Inicialmente todos están libres
		SFreeInodesCount: inodeCount,  // Inicialmente todos están libres
		SMtime:           now,         // Primera montada ahora
		SUmtime:          time.Time{}, // Nunca desmontado
		SMntCount:        1,           // Primera montada
		SMagic:           EXT2_MAGIC,
		SInodeSize:       inodeSize,
		SBlockSize:       blockSize,
		SFirstIno:        0, // Se actualizará al formatear
		SFirstBlo:        0, // Se actualizará al formatear
		SBmInodeStart:    bmInodeStart,
		SBmBlockStart:    bmBlockStart,
		SInodeStart:      inodeStart,
		SBlockStart:      blockStart,
	}

	// Resetear el padding
	for i := range sb.SPadding {
		sb.SPadding[i] = 0
	}

	return sb
}

// UpdateMountInfo actualiza la información de montaje
func (sb *SuperBlock) UpdateMountInfo() {
	sb.SMtime = time.Now()
	sb.SMntCount++
}

// UpdateUnmountInfo actualiza la información al desmontar
func (sb *SuperBlock) UpdateUnmountInfo() {
	sb.SUmtime = time.Now()
}

// AllocateInode marca un inodo como utilizado y actualiza contadores
func (sb *SuperBlock) AllocateInode() {
	if sb.SFreeInodesCount > 0 {
		sb.SFreeInodesCount--
	}
}

// FreeInode marca un inodo como libre y actualiza contadores
func (sb *SuperBlock) FreeInode() {
	if sb.SFreeInodesCount < sb.SInodesCount {
		sb.SFreeInodesCount++
	}
}

// AllocateBlock marca un bloque como utilizado y actualiza contadores
func (sb *SuperBlock) AllocateBlock() {
	if sb.SFreeBlocksCount > 0 {
		sb.SFreeBlocksCount--
	}
}

// FreeBlock marca un bloque como libre y actualiza contadores
func (sb *SuperBlock) FreeBlock() {
	if sb.SFreeBlocksCount < sb.SBlocksCount {
		sb.SFreeBlocksCount++
	}
}

// SetFirstFreeInode establece el primer inodo libre
func (sb *SuperBlock) SetFirstFreeInode(inodeNum int32) {
	sb.SFirstIno = inodeNum
}

// SetFirstFreeBlock establece el primer bloque libre
func (sb *SuperBlock) SetFirstFreeBlock(blockNum int32) {
	sb.SFirstBlo = blockNum
}

// GetFilesystemStats obtiene estadísticas del sistema de archivos
func (sb *SuperBlock) GetFilesystemStats() map[string]interface{} {
	return map[string]interface{}{
		"magic":           sb.SMagic,
		"filesystem_type": sb.SFilesystemType,
		"total_inodes":    sb.SInodesCount,
		"free_inodes":     sb.SFreeInodesCount,
		"total_blocks":    sb.SBlocksCount,
		"free_blocks":     sb.SFreeBlocksCount,
		"mount_count":     sb.SMntCount,
		"last_mount":      sb.SMtime,
		"last_unmount":    sb.SUmtime,
		"inode_size":      sb.SInodeSize,
		"block_size":      sb.SBlockSize,
		"inode_usage":     float64(sb.SInodesCount-sb.SFreeInodesCount) / float64(sb.SInodesCount) * 100,
		"block_usage":     float64(sb.SBlocksCount-sb.SFreeBlocksCount) / float64(sb.SBlocksCount) * 100,
	}
}
