package DiskManager

import (
	"fmt"
	"math"
)

// BitmapManager gestiona los bitmaps de inodos y bloques para el sistema de archivos EXT2
type BitmapManager struct {
	InodeBitmap []byte // Bitmap para inodos (1 bit por inodo)
	BlockBitmap []byte // Bitmap para bloques (1 bit por bloque)
	InodeCount  int    // Número total de inodos
	BlockCount  int    // Número total de bloques
}

// NewBitmapManager crea una nueva instancia del gestor de bitmaps
func NewBitmapManager(inodeCount, blockCount int) *BitmapManager {
	// Calcular el tamaño en bytes (redondeando hacia arriba)
	inodeBitmapSize := (inodeCount + 7) / 8
	blockBitmapSize := (blockCount + 7) / 8

	// Crear e inicializar los bitmaps
	bm := &BitmapManager{
		InodeBitmap: make([]byte, inodeBitmapSize),
		BlockBitmap: make([]byte, blockBitmapSize),
		InodeCount:  inodeCount,
		BlockCount:  blockCount,
	}

	// Inicializar todos los bits a 0 (libres)
	for i := range bm.InodeBitmap {
		bm.InodeBitmap[i] = 0
	}
	for i := range bm.BlockBitmap {
		bm.BlockBitmap[i] = 0
	}

	return bm
}

// SetBit establece un bit específico a 1 (ocupado)
func (bm *BitmapManager) SetBit(bitmap []byte, position int) error {
	bitmapSize := len(bitmap) * 8
	if position < 0 || position >= bitmapSize {
		return fmt.Errorf("posición %d fuera de rango (0-%d)", position, bitmapSize-1)
	}

	byteIndex := position / 8
	bitOffset := position % 8
	bitmap[byteIndex] |= (1 << bitOffset)

	return nil
}

// ClearBit establece un bit específico a 0 (libre)
func (bm *BitmapManager) ClearBit(bitmap []byte, position int) error {
	bitmapSize := len(bitmap) * 8
	if position < 0 || position >= bitmapSize {
		return fmt.Errorf("posición %d fuera de rango (0-%d)", position, bitmapSize-1)
	}

	byteIndex := position / 8
	bitOffset := position % 8
	bitmap[byteIndex] &= ^(1 << bitOffset)

	return nil
}

// IsBitSet verifica si un bit específico está a 1 (ocupado)
func (bm *BitmapManager) IsBitSet(bitmap []byte, position int) (bool, error) {
	bitmapSize := len(bitmap) * 8
	if position < 0 || position >= bitmapSize {
		return false, fmt.Errorf("posición %d fuera de rango (0-%d)", position, bitmapSize-1)
	}

	byteIndex := position / 8
	bitOffset := position % 8
	return (bitmap[byteIndex] & (1 << bitOffset)) != 0, nil
}

// AllocateInode busca y marca el primer inodo libre
// Retorna el número de inodo asignado o -1 si no hay inodos disponibles
func (bm *BitmapManager) AllocateInode() int {
	// Buscar el primer bit libre (0)
	for i := 0; i < bm.InodeCount; i++ {
		isBusy, _ := bm.IsBitSet(bm.InodeBitmap, i)
		if !isBusy {
			// Marcar como ocupado
			_ = bm.SetBit(bm.InodeBitmap, i)
			return i
		}
	}
	return -1 // No hay inodos disponibles
}

// FreeInode marca un inodo como libre
func (bm *BitmapManager) FreeInode(inodeNum int) error {
	if inodeNum < 0 || inodeNum >= bm.InodeCount {
		return fmt.Errorf("número de inodo %d fuera de rango (0-%d)", inodeNum, bm.InodeCount-1)
	}

	return bm.ClearBit(bm.InodeBitmap, inodeNum)
}

// AllocateBlock busca y marca el primer bloque libre
// Retorna el número de bloque asignado o -1 si no hay bloques disponibles
func (bm *BitmapManager) AllocateBlock() int {
	// Buscar el primer bit libre (0)
	for i := 0; i < bm.BlockCount; i++ {
		isBusy, _ := bm.IsBitSet(bm.BlockBitmap, i)
		if !isBusy {
			// Marcar como ocupado
			_ = bm.SetBit(bm.BlockBitmap, i)
			return i
		}
	}
	return -1 // No hay bloques disponibles
}

// FreeBlock marca un bloque como libre
func (bm *BitmapManager) FreeBlock(blockNum int) error {
	if blockNum < 0 || blockNum >= bm.BlockCount {
		return fmt.Errorf("número de bloque %d fuera de rango (0-%d)", blockNum, bm.BlockCount-1)
	}

	return bm.ClearBit(bm.BlockBitmap, blockNum)
}

// ReserveInitialBlocks reserva los bloques e inodos iniciales del sistema
// (superbloque, bitmaps, inodos raíz, etc.)
func (bm *BitmapManager) ReserveInitialBlocks(reservedInodes, reservedBlocks int) {
	// Reservar inodos iniciales (0, 1, 2, ...)
	for i := 0; i < reservedInodes && i < bm.InodeCount; i++ {
		_ = bm.SetBit(bm.InodeBitmap, i)
	}

	// Reservar bloques iniciales (0, 1, 2, ...)
	for i := 0; i < reservedBlocks && i < bm.BlockCount; i++ {
		_ = bm.SetBit(bm.BlockBitmap, i)
	}
}

// GetFreeInodeCount devuelve el número de inodos libres
func (bm *BitmapManager) GetFreeInodeCount() int {
	count := 0
	for i := 0; i < bm.InodeCount; i++ {
		isBusy, _ := bm.IsBitSet(bm.InodeBitmap, i)
		if !isBusy {
			count++
		}
	}
	return count
}

// GetFreeBlockCount devuelve el número de bloques libres
func (bm *BitmapManager) GetFreeBlockCount() int {
	count := 0
	for i := 0; i < bm.BlockCount; i++ {
		isBusy, _ := bm.IsBitSet(bm.BlockBitmap, i)
		if !isBusy {
			count++
		}
	}
	return count
}

// GetUsageStats devuelve estadísticas de uso del sistema de archivos
func (bm *BitmapManager) GetUsageStats() map[string]interface{} {
	freeInodes := bm.GetFreeInodeCount()
	freeBlocks := bm.GetFreeBlockCount()

	return map[string]interface{}{
		"total_inodes":      bm.InodeCount,
		"free_inodes":       freeInodes,
		"used_inodes":       bm.InodeCount - freeInodes,
		"inodes_usage_pct":  math.Round((float64(bm.InodeCount-freeInodes) / float64(bm.InodeCount)) * 100.0),
		"total_blocks":      bm.BlockCount,
		"free_blocks":       freeBlocks,
		"used_blocks":       bm.BlockCount - freeBlocks,
		"blocks_usage_pct":  math.Round((float64(bm.BlockCount-freeBlocks) / float64(bm.BlockCount)) * 100.0),
		"inode_bitmap_size": len(bm.InodeBitmap),
		"block_bitmap_size": len(bm.BlockBitmap),
	}
}

// GetInodeBitmap obtiene una copia del bitmap de inodos
func (bm *BitmapManager) GetInodeBitmap() []byte {
	bitmapCopy := make([]byte, len(bm.InodeBitmap))
	copy(bitmapCopy, bm.InodeBitmap)
	return bitmapCopy
}

// GetBlockBitmap obtiene una copia del bitmap de bloques
func (bm *BitmapManager) GetBlockBitmap() []byte {
	bitmapCopy := make([]byte, len(bm.BlockBitmap))
	copy(bitmapCopy, bm.BlockBitmap)
	return bitmapCopy
}

// FindFirstFreeInode encuentra el primer inodo libre
// Retorna el número de inodo o -1 si no hay inodos disponibles
func (bm *BitmapManager) FindFirstFreeInode() int {
	for i := 0; i < bm.InodeCount; i++ {
		isBusy, _ := bm.IsBitSet(bm.InodeBitmap, i)
		if !isBusy {
			return i
		}
	}
	return -1 // No hay inodos disponibles
}

// FindFirstFreeBlock encuentra el primer bloque libre
// Retorna el número de bloque o -1 si no hay bloques disponibles
func (bm *BitmapManager) FindFirstFreeBlock() int {
	for i := 0; i < bm.BlockCount; i++ {
		isBusy, _ := bm.IsBitSet(bm.BlockBitmap, i)
		if !isBusy {
			return i
		}
	}
	return -1 // No hay bloques disponibles
}
