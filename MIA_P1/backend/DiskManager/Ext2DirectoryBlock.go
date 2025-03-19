package DiskManager

import (
	"bytes"
	"fmt"
	"strings"
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
	dir := &DirectoryBlock{}
	// Inicializar todas las entradas con inodo -1 para indicar entrada vacía
	for i := range dir.BContent {
		dir.BContent[i].BInodo = -1
	}
	return dir
}

// InitializeAsDirectory inicializa el bloque como una carpeta con sus entradas "." y ".."
func (dirBlock *DirectoryBlock) InitializeAsDirectory(selfInode, parentInode int32, dirName string) {
	// Entrada "." - referencia a sí mismo
	copy(dirBlock.BContent[0].BName[:], ".")
	dirBlock.BContent[0].BInodo = selfInode

	// Entrada ".." - referencia al padre
	copy(dirBlock.BContent[1].BName[:], "..")
	dirBlock.BContent[1].BInodo = parentInode

	// Nombre del directorio actual (opcional, para referencia)
	if dirName != "" {
		copy(dirBlock.BContent[2].BName[:], dirName)
		dirBlock.BContent[2].BInodo = selfInode
	}
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

// AddEntry añade una nueva entrada al directorio
func (dirBlock *DirectoryBlock) AddEntry(name string, inodeNum int32) bool {
	fmt.Printf("Añadiendo entrada: nombre='%s', inodo=%d\n", name, inodeNum)

	// Buscar una entrada libre (inodo == -1)
	for i := 2; i < B_CONTENT_COUNT; i++ {
		if dirBlock.BContent[i].BInodo == -1 { // Cambio aquí: verificar -1, no 0
			// Limpiar la entrada
			for j := range dirBlock.BContent[i].BName {
				dirBlock.BContent[i].BName[j] = 0
			}

			// Copiar el nombre con seguridad
			copy(dirBlock.BContent[i].BName[:], name)
			dirBlock.BContent[i].BInodo = inodeNum

			// Verificar la entrada
			storedName := strings.TrimRight(string(dirBlock.BContent[i].BName[:]), "\x00")
			fmt.Printf("Entrada añadida en posición %d: '%s' -> inodo %d\n",
				i, storedName, dirBlock.BContent[i].BInodo)
			return true
		}
	}
	return false
}

func (dirBlock *DirectoryBlock) PrintEntries() {
	fmt.Println("\nEntradas del directorio:")
	for i := 0; i < B_CONTENT_COUNT; i++ {
		name := strings.TrimRight(string(dirBlock.BContent[i].BName[:]), "\x00")
		if dirBlock.BContent[i].BInodo != -1 { // Cambio aquí: verificar -1, no 0
			fmt.Printf("[%d] '%s' -> inodo %d\n",
				i, name, dirBlock.BContent[i].BInodo)
		}
	}
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

// GetEntries retorna todas las entradas válidas del directorio
func (dirBlock *DirectoryBlock) GetEntries() []struct {
	Name     string
	InodeNum int32
} {
	var entries []struct {
		Name     string
		InodeNum int32
	}

	for i := 0; i < B_CONTENT_COUNT; i++ {
		if dirBlock.BContent[i].BInodo != 0 {
			name := strings.TrimRight(string(dirBlock.BContent[i].BName[:]), "\x00")
			entries = append(entries, struct {
				Name     string
				InodeNum int32
			}{
				Name:     name,
				InodeNum: dirBlock.BContent[i].BInodo,
			})
		}
	}
	return entries
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
