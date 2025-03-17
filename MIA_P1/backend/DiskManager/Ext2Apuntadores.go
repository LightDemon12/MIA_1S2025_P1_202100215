package DiskManager

import (
	"fmt"
)

// Constantes para el bloque de apuntadores
const (
	POINTERS_PER_BLOCK   = 16 // Número de apuntadores en cada bloque
	POINTER_UNUSED_VALUE = -1 // Valor para apuntadores no utilizados
)

// PointerBlock representa un bloque de apuntadores indirectos
type PointerBlock struct {
	BPointers [POINTERS_PER_BLOCK]int32 // Array con los apuntadores a bloques (16 * 4 bytes = 64 bytes)
}

// NewPointerBlock crea un nuevo bloque de apuntadores inicializado
func NewPointerBlock() *PointerBlock {
	pointerBlock := &PointerBlock{}

	// Inicializar todos los apuntadores como no utilizados (-1)
	for i := range pointerBlock.BPointers {
		pointerBlock.BPointers[i] = POINTER_UNUSED_VALUE
	}

	return pointerBlock
}

// SetPointer establece un apuntador en una posición específica
func (pb *PointerBlock) SetPointer(index int, blockNumber int32) error {
	if index < 0 || index >= POINTERS_PER_BLOCK {
		return fmt.Errorf("índice fuera de rango: %d", index)
	}

	pb.BPointers[index] = blockNumber
	return nil
}

// GetPointer obtiene el valor de un apuntador en una posición específica
func (pb *PointerBlock) GetPointer(index int) (int32, error) {
	if index < 0 || index >= POINTERS_PER_BLOCK {
		return POINTER_UNUSED_VALUE, fmt.Errorf("índice fuera de rango: %d", index)
	}

	return pb.BPointers[index], nil
}

// AddPointer añade un apuntador al primer espacio disponible
// Retorna el índice donde se añadió, o -1 si no hay espacio
func (pb *PointerBlock) AddPointer(blockNumber int32) int {
	for i := 0; i < POINTERS_PER_BLOCK; i++ {
		if pb.BPointers[i] == POINTER_UNUSED_VALUE {
			pb.BPointers[i] = blockNumber
			return i
		}
	}
	return -1 // No hay espacio disponible
}

// RemovePointer elimina un apuntador (establece -1)
func (pb *PointerBlock) RemovePointer(index int) error {
	if index < 0 || index >= POINTERS_PER_BLOCK {
		return fmt.Errorf("índice fuera de rango: %d", index)
	}

	pb.BPointers[index] = POINTER_UNUSED_VALUE
	return nil
}

// GetUsedPointers obtiene todos los apuntadores utilizados
func (pb *PointerBlock) GetUsedPointers() []int32 {
	var used []int32

	for i := 0; i < POINTERS_PER_BLOCK; i++ {
		if pb.BPointers[i] != POINTER_UNUSED_VALUE {
			used = append(used, pb.BPointers[i])
		}
	}

	return used
}

// GetUsedCount devuelve el número de apuntadores utilizados
func (pb *PointerBlock) GetUsedCount() int {
	count := 0
	for i := 0; i < POINTERS_PER_BLOCK; i++ {
		if pb.BPointers[i] != POINTER_UNUSED_VALUE {
			count++
		}
	}
	return count
}

// IsFull verifica si el bloque de apuntadores está lleno
func (pb *PointerBlock) IsFull() bool {
	return pb.GetUsedCount() == POINTERS_PER_BLOCK
}

// IsEmpty verifica si el bloque de apuntadores está vacío
func (pb *PointerBlock) IsEmpty() bool {
	return pb.GetUsedCount() == 0
}

// Clear limpia todos los apuntadores del bloque
func (pb *PointerBlock) Clear() {
	for i := range pb.BPointers {
		pb.BPointers[i] = POINTER_UNUSED_VALUE
	}
}

// CalculateIndirectCapacity calcula la capacidad de bloques de datos según el nivel
// de indirección (1=simple, 2=doble, 3=triple)
func CalculateIndirectCapacity(indirectionLevel int) int {
	switch indirectionLevel {
	case 1: // Simple indirecto
		return POINTERS_PER_BLOCK
	case 2: // Doble indirecto
		return POINTERS_PER_BLOCK * POINTERS_PER_BLOCK
	case 3: // Triple indirecto
		return POINTERS_PER_BLOCK * POINTERS_PER_BLOCK * POINTERS_PER_BLOCK
	default:
		return 0
	}
}
