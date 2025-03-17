package DiskManager

import (
	"bytes"
	"fmt"
)

// Constantes para el bloque de carpetas
const (
	B_NAME_SIZE            = 12   // Tamaño del nombre en bytes
	B_CONTENT_COUNT        = 4    // Número de entradas de contenido por bloque
	DIRECTORY_ENTRY_SELF   = "."  // Nombre para la entrada de la carpeta actual
	DIRECTORY_ENTRY_PARENT = ".." // Nombre para la entrada de la carpeta padre
)

// BContent representa una entrada dentro del bloque de carpetas
type BContent struct {
	BName  [B_NAME_SIZE]byte // Nombre de la carpeta o archivo (12 bytes)
	BInodo int32             // Apuntador hacia un inodo asociado (4 bytes)
}

// DirectoryBlock representa un bloque de carpetas
type DirectoryBlock struct {
	BContent [B_CONTENT_COUNT]BContent // Array con el contenido de la carpeta (4 entradas)
}

// NewDirectoryBlock crea un nuevo bloque de carpetas inicializado
func NewDirectoryBlock() *DirectoryBlock {
	dirBlock := &DirectoryBlock{}

	// Inicializar todas las entradas con valores por defecto
	for i := range dirBlock.BContent {
		// Inicializar nombre con bytes nulos
		for j := range dirBlock.BContent[i].BName {
			dirBlock.BContent[i].BName[j] = 0
		}
		// Inicializar inodo con -1 (no utilizado)
		dirBlock.BContent[i].BInodo = -1
	}

	return dirBlock
}

// InitializeAsDirectory inicializa el bloque como una carpeta con sus entradas "." y ".."
func (db *DirectoryBlock) InitializeAsDirectory(selfInodeNum, parentInodeNum int32) {
	// Primera entrada: carpeta actual "."
	db.SetEntry(0, DIRECTORY_ENTRY_SELF, selfInodeNum)

	// Segunda entrada: carpeta padre ".."
	db.SetEntry(1, DIRECTORY_ENTRY_PARENT, parentInodeNum)
}

// SetEntry establece una entrada en el bloque de carpetas
func (db *DirectoryBlock) SetEntry(index int, name string, inodeNum int32) error {
	if index < 0 || index >= B_CONTENT_COUNT {
		return fmt.Errorf("índice fuera de rango: %d", index)
	}

	if len(name) > B_NAME_SIZE {
		return fmt.Errorf("nombre demasiado largo: %s (máx %d caracteres)", name, B_NAME_SIZE)
	}

	// Limpiar la entrada primero
	for i := range db.BContent[index].BName {
		db.BContent[index].BName[i] = 0
	}

	// Copiar el nombre (truncando si es necesario)
	copy(db.BContent[index].BName[:], []byte(name))

	// Establecer el número de inodo
	db.BContent[index].BInodo = inodeNum

	return nil
}

// GetEntry obtiene información de una entrada específica
func (db *DirectoryBlock) GetEntry(index int) (string, int32, error) {
	if index < 0 || index >= B_CONTENT_COUNT {
		return "", -1, fmt.Errorf("índice fuera de rango: %d", index)
	}

	// Obtener el nombre hasta el primer byte nulo
	nameBytes := bytes.Trim(db.BContent[index].BName[:], "\x00")
	name := string(nameBytes)

	// Devolver nombre e inodo
	return name, db.BContent[index].BInodo, nil
}

// FindEntry busca una entrada por nombre y devuelve su índice e inodo
func (db *DirectoryBlock) FindEntry(name string) (int, int32) {
	for i := 0; i < B_CONTENT_COUNT; i++ {
		entryName, inodeNum, err := db.GetEntry(i)
		if err == nil && entryName == name && inodeNum != -1 {
			return i, inodeNum
		}
	}
	return -1, -1 // No encontrado
}

// HasFreeEntry verifica si hay espacio para nuevas entradas
func (db *DirectoryBlock) HasFreeEntry() bool {
	for i := 0; i < B_CONTENT_COUNT; i++ {
		if db.BContent[i].BInodo == -1 {
			return true
		}
	}
	return false
}

// AddEntry añade una nueva entrada al primer espacio disponible
func (db *DirectoryBlock) AddEntry(name string, inodeNum int32) error {
	// Buscar el primer espacio disponible
	for i := 0; i < B_CONTENT_COUNT; i++ {
		if db.BContent[i].BInodo == -1 {
			return db.SetEntry(i, name, inodeNum)
		}
	}
	return fmt.Errorf("no hay espacio disponible en el bloque de carpetas")
}

// RemoveEntry elimina una entrada por nombre
func (db *DirectoryBlock) RemoveEntry(name string) bool {
	idx, _ := db.FindEntry(name)
	if idx != -1 {
		// Limpiar la entrada
		for i := range db.BContent[idx].BName {
			db.BContent[idx].BName[i] = 0
		}
		db.BContent[idx].BInodo = -1
		return true
	}
	return false
}

// GetEntryCount devuelve el número de entradas utilizadas
func (db *DirectoryBlock) GetEntryCount() int {
	count := 0
	for i := 0; i < B_CONTENT_COUNT; i++ {
		if db.BContent[i].BInodo != -1 {
			count++
		}
	}
	return count
}

// ListEntries devuelve una lista de todas las entradas válidas
func (db *DirectoryBlock) ListEntries() []struct {
	Name     string
	InodeNum int32
} {
	var entries []struct {
		Name     string
		InodeNum int32
	}

	for i := 0; i < B_CONTENT_COUNT; i++ {
		name, inodeNum, err := db.GetEntry(i)
		if err == nil && inodeNum != -1 {
			entries = append(entries, struct {
				Name     string
				InodeNum int32
			}{
				Name:     name,
				InodeNum: inodeNum,
			})
		}
	}

	return entries
}
