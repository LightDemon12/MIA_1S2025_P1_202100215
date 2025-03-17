package DiskManager

import (
	"bytes"
	"fmt"
)

// Constantes para el bloque de archivos
const (
	FILE_BLOCK_SIZE = 64 // Tamaño del bloque de archivo en bytes
)

// FileBlock representa un bloque de contenido de archivo
type FileBlock struct {
	BContent [FILE_BLOCK_SIZE]byte // Array con el contenido del archivo (64 bytes)
}

// NewFileBlock crea un nuevo bloque de archivo vacío
func NewFileBlock() *FileBlock {
	fileBlock := &FileBlock{}

	// Inicializar el contenido con bytes nulos
	for i := range fileBlock.BContent {
		fileBlock.BContent[i] = 0
	}

	return fileBlock
}

// WriteContent escribe contenido en el bloque
// Retorna el número de bytes escritos
func (fb *FileBlock) WriteContent(content []byte) int {
	// Limpia el contenido actual
	for i := range fb.BContent {
		fb.BContent[i] = 0
	}

	// Determina cuántos bytes copiar (el mínimo entre el tamaño del bloque y el contenido)
	copySize := len(content)
	if copySize > FILE_BLOCK_SIZE {
		copySize = FILE_BLOCK_SIZE
	}

	// Copia el contenido
	copy(fb.BContent[:], content[:copySize])

	return copySize
}

// AppendContent añade contenido al final del bloque existente
// Retorna el número de bytes añadidos
func (fb *FileBlock) AppendContent(content []byte) int {
	// Encuentra la posición del primer byte nulo
	position := 0
	for i, b := range fb.BContent {
		if b == 0 {
			position = i
			break
		}
	}

	// Si el bloque está lleno, no se puede añadir nada
	if position >= FILE_BLOCK_SIZE {
		return 0
	}

	// Determina cuántos bytes se pueden añadir
	remainingSpace := FILE_BLOCK_SIZE - position
	appendSize := len(content)
	if appendSize > remainingSpace {
		appendSize = remainingSpace
	}

	// Añade el contenido
	copy(fb.BContent[position:], content[:appendSize])

	return appendSize
}

// GetContent obtiene todo el contenido del bloque hasta el primer byte nulo
// o el tamaño completo si está todo utilizado
func (fb *FileBlock) GetContent() []byte {
	// Encuentra el contenido hasta el primer byte nulo
	return bytes.TrimRight(fb.BContent[:], "\x00")
}

// GetRawContent obtiene todo el contenido del bloque incluyendo bytes nulos
func (fb *FileBlock) GetRawContent() []byte {
	// Copia el contenido completo
	content := make([]byte, FILE_BLOCK_SIZE)
	copy(content, fb.BContent[:])
	return content
}

// GetContentSlice obtiene una porción específica del contenido
func (fb *FileBlock) GetContentSlice(start, length int) ([]byte, error) {
	if start < 0 || start >= FILE_BLOCK_SIZE {
		return nil, fmt.Errorf("posición de inicio inválida: %d", start)
	}

	if length <= 0 {
		return []byte{}, nil
	}

	// Ajustar longitud si excede el límite
	if start+length > FILE_BLOCK_SIZE {
		length = FILE_BLOCK_SIZE - start
	}

	// Copiar la porción solicitada
	slice := make([]byte, length)
	copy(slice, fb.BContent[start:start+length])

	return slice, nil
}

// GetContentSize devuelve el tamaño del contenido (hasta el primer byte nulo)
func (fb *FileBlock) GetContentSize() int {
	for i, b := range fb.BContent {
		if b == 0 {
			return i
		}
	}
	return FILE_BLOCK_SIZE // Si no hay bytes nulos, está lleno
}

// IsEmpty verifica si el bloque está vacío (solo contiene bytes nulos)
func (fb *FileBlock) IsEmpty() bool {
	for _, b := range fb.BContent {
		if b != 0 {
			return false
		}
	}
	return true
}

// Clear limpia todo el contenido del bloque
func (fb *FileBlock) Clear() {
	for i := range fb.BContent {
		fb.BContent[i] = 0
	}
}
