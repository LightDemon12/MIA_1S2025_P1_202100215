package DiskManager

import (
	"time"
)

// EXT2_FILESYSTEM_TYPE identifica el tipo de sistema de archivos
const (
	EXT2_FILESYSTEM_TYPE = 2 // Valor 2 para EXT2 - 4 bytes al serializar como int32
	// Posible cambio: usar uint8 (1 byte) si solo se necesita un número pequeño
)

// SuperBlock representa la estructura del superbloque en el sistema de archivos EXT2
// Tamaño total: 1024 bytes exactos (estándar EXT2)
type SuperBlock struct {
	SFilesystemType  int32     // Tipo FS: 4 bytes, valor 2=EXT2 (posible cambio: uint8 ahorra 3 bytes)
	SInodesCount     int32     // Total inodos: 4 bytes, hasta ~2 mil millones (cambio: uint32 evita negativos)
	SBlocksCount     int32     // Total bloques: 4 bytes, igual que inodos
	SFreeBlocksCount int32     // Bloques libres: 4 bytes, debe ser ≤ SBlocksCount
	SFreeInodesCount int32     // Inodos libres: 4 bytes, debe ser ≤ SInodesCount
	SMtime           time.Time // Montaje: 8-24 bytes según plataforma (cambio: int64 timestamp usa 8 bytes fijos)
	SUmtime          time.Time // Desmontaje: igual que SMtime
	SMntCount        int32     // Contador montajes: 4 bytes (cambio: uint16 suficiente para conteo)
	SMagic           int32     // Magic: 4 bytes, valor 0xEF53 (cambio: uint16 suficiente y más preciso)
	SInodeSize       int32     // Tamaño inodo: 4 bytes (cambio: uint16 suficiente para tamaños comunes)
	SBlockSize       int32     // Tamaño bloque: 4 bytes (cambio: uint16 suficiente para tamaños comunes)
	SFirstIno        int32     // Primer inodo libre: 4 bytes, índice (cambio: uint32 evita negativos)
	SFirstBlo        int32     // Primer bloque libre: 4 bytes, índice (cambio: uint32 evita negativos)
	SBmInodeStart    int32     // Inicio bitmap inodos: 4 bytes, offset (cambio: uint32 para offsets grandes)
	SBmBlockStart    int32     // Inicio bitmap bloques: 4 bytes, offset (cambio: uint32 para offsets grandes)
	SInodeStart      int32     // Inicio tabla inodos: 4 bytes, offset (cambio: uint32 para offsets grandes)
	SBlockStart      int32     // Inicio tabla bloques: 4 bytes, offset (cambio: uint32 para offsets grandes)
	SPadding         [808]byte // Padding: 808 bytes para completar 1024 bytes exactos
	// Ajustar SPadding si cambian otros campos para mantener 1024 bytes totales
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
