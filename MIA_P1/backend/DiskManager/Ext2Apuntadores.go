package DiskManager

import (
	"fmt"
)

// Constantes para el bloque de apuntadores
const (
	// POINTERS_PER_BLOCK define cuántos apuntadores caben en un bloque.
	// Alternativas:
	// - Aumentar a 256 para bloques de 1KB maximizaría el uso (256 × 4 = 1024 bytes)
	// - Reducir a 8 si se cambia a int64 para sistemas extremadamente grandes
	POINTERS_PER_BLOCK = 16
	// Valor actual: -1 (representado como 0xFFFFFFFF en complemento a dos)
	// - Si se cambia a uint32: usar 0xFFFFFFFF (valor máximo)
	// - Si se cambia a int64: -1 sigue siendo válido pero ocuparía 8 bytes
	POINTER_UNUSED_VALUE = -1
)

// PointerBlock representa un bloque de apuntadores indirectos en un sistema ext2.
// Especificaciones técnicas:
// - Tamaño total: 64 bytes (16 apuntadores × 4 bytes)
// - Tipo de apuntador: int32 (rango: -2,147,483,648 a 2,147,483,647)
// - Limitación: Máximo ~2 mil millones de bloques direccionables
// - Valor especial: -1 indica "apuntador no utilizado"
//  1. Cambiar a uint32 permitiría hasta ~4 mil millones de bloques
//     pero requeriría usar 0xFFFFFFFF como valor "no utilizado"
//  2. Cambiar a int64 permitiría sistemas extremadamente grandes
//     pero reduciría los apuntadores por bloque a 8 (manteniendo 64 bytes)
//  3. Aumentar POINTERS_PER_BLOCK a 256 optimizaría para bloques de 1KB
//     pero requeriría ajustes en toda la lógica de navegación de bloques
type PointerBlock struct {
	// BPointers contiene índices a otros bloques de datos o apuntadores.
	// - Cada apuntador ocupa 4 bytes exactamente (int32)
	// - El valor -1 (POINTER_UNUSED_VALUE) indica posición no utilizada
	// - Los valores válidos son índices de bloque no negativos
	// - No hay padding adicional en esta estructura (64 bytes exactos)
	BPointers [POINTERS_PER_BLOCK]int32
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
