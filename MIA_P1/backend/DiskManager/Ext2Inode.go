package DiskManager

import (
	"time"
)

// Constantes para tipos de inodo
const (
	INODE_FOLDER = 0 // Tipo carpeta
	INODE_FILE   = 1 // Tipo archivo
)

// Constantes para permisos UGO (User, Group, Other)
const (
	// Combinaciones comunes
	PERM_DEFAULT_FILE   = 0644 // rw-r--r--
	PERM_DEFAULT_FOLDER = 0755 // rwxr-xr-x
)

// Constantes para índices de bloques indirectos
const (
	INDIRECT_BLOCK_INDEX        = 12 // Índice del bloque indirecto simple
	DOUBLE_INDIRECT_BLOCK_INDEX = 13 // Índice del bloque indirecto doble
	TRIPLE_INDIRECT_BLOCK_INDEX = 14 // Índice del bloque indirecto triple
)

// Inode representa la estructura de un inodo en el sistema de archivos EXT2
type Inode struct {
	IUid   int32     // UID del usuario propietario
	IGid   int32     // GID del grupo al que pertenece
	ISize  int32     // Tamaño del archivo en bytes
	IPerm  [3]byte   // Permisos UGO en formato octal [0-7][0-7][0-7]
	IAtime time.Time // Última fecha de acceso sin modificación
	ICtime time.Time // Fecha de creación
	IMtime time.Time // Última fecha de modificación
	IBlock [15]int32 // Punteros a bloques (12 directos, 3 indirectos)
	IType  byte      // Tipo: 0=Carpeta, 1=Archivo
	// Padding para ajustar el tamaño si es necesario
	IPadding [4]byte // Ajustado por la adición de IPerm
}

// NewInode crea un nuevo inodo inicializado
func NewInode(uid, gid int32, inodeType byte) *Inode {
	now := time.Now()

	// Determinar permisos predeterminados según el tipo
	var defaultPerm int
	if inodeType == INODE_FOLDER {
		defaultPerm = PERM_DEFAULT_FOLDER
	} else {
		defaultPerm = PERM_DEFAULT_FILE
	}

	// Convertir permisos a formato octal de 3 bytes
	permBytes := [3]byte{
		byte((defaultPerm >> 6) & 0x7), // User (bits 8-6)
		byte((defaultPerm >> 3) & 0x7), // Group (bits 5-3)
		byte(defaultPerm & 0x7),        // Other (bits 2-0)
	}

	inode := &Inode{
		IUid:   uid,
		IGid:   gid,
		ISize:  0,
		IPerm:  permBytes,
		IAtime: now,
		ICtime: now,
		IMtime: now,
		IType:  inodeType,
	}

	// Inicializar todos los punteros a bloques con -1 (no utilizados)
	for i := range inode.IBlock {
		inode.IBlock[i] = -1
	}

	// Inicializar padding con ceros
	for i := range inode.IPadding {
		inode.IPadding[i] = 0
	}

	return inode
}

// GetPermission devuelve el valor octal de los permisos completos
func (i *Inode) GetPermission() int {
	return (int(i.IPerm[0]) << 6) | (int(i.IPerm[1]) << 3) | int(i.IPerm[2])
}

// SetPermission establece los permisos desde un valor octal
func (i *Inode) SetPermission(perm int) {
	i.IPerm[0] = byte((perm >> 6) & 0x7) // User
	i.IPerm[1] = byte((perm >> 3) & 0x7) // Group
	i.IPerm[2] = byte(perm & 0x7)        // Other
	i.UpdateModificationTime()
}

// HasUserPermission verifica si el usuario tiene un permiso específico
func (i *Inode) HasUserPermission(perm int) bool {
	userPerm := int(i.IPerm[0])
	return (userPerm & (perm & 0x7)) == (perm & 0x7)
}

// HasGroupPermission verifica si el grupo tiene un permiso específico
func (i *Inode) HasGroupPermission(perm int) bool {
	groupPerm := int(i.IPerm[1])
	return (groupPerm & (perm & 0x7)) == (perm & 0x7)
}

// HasOtherPermission verifica si otros tienen un permiso específico
func (i *Inode) HasOtherPermission(perm int) bool {
	otherPerm := int(i.IPerm[2])
	return (otherPerm & (perm & 0x7)) == (perm & 0x7)
}

// GetPermissionString devuelve los permisos en formato de cadena (rwxrwxrwx)
func (i *Inode) GetPermissionString() string {
	result := ""

	// Función auxiliar para convertir un valor de permiso a rwx
	permToStr := func(p byte) string {
		str := ""
		str += (map[bool]string{true: "r", false: "-"}[(p&0x4) != 0])
		str += (map[bool]string{true: "w", false: "-"}[(p&0x2) != 0])
		str += (map[bool]string{true: "x", false: "-"}[(p&0x1) != 0])
		return str
	}

	// Concatenar permisos de usuario, grupo y otros
	result += permToStr(i.IPerm[0])
	result += permToStr(i.IPerm[1])
	result += permToStr(i.IPerm[2])

	return result
}

// IsFolder verifica si el inodo representa una carpeta
func (i *Inode) IsFolder() bool {
	return i.IType == INODE_FOLDER
}

// IsFile verifica si el inodo representa un archivo
func (i *Inode) IsFile() bool {
	return i.IType == INODE_FILE
}

// GetDirectBlocks devuelve los bloques directos
func (i *Inode) GetDirectBlocks() []int32 {
	result := make([]int32, 0, INDIRECT_BLOCK_INDEX)
	for idx := 0; idx < INDIRECT_BLOCK_INDEX; idx++ {
		if i.IBlock[idx] != -1 {
			result = append(result, i.IBlock[idx])
		}
	}
	return result
}

// AddDirectBlock añade un bloque directo si hay espacio
// Retorna true si se añadió el bloque, false si no hay espacio
func (i *Inode) AddDirectBlock(blockNum int32) bool {
	for idx := 0; idx < INDIRECT_BLOCK_INDEX; idx++ {
		if i.IBlock[idx] == -1 {
			i.IBlock[idx] = blockNum
			return true
		}
	}
	return false // No hay espacio en bloques directos
}

// UpdateAccessTime actualiza el tiempo de último acceso
func (i *Inode) UpdateAccessTime() {
	i.IAtime = time.Now()
}

// UpdateModificationTime actualiza el tiempo de última modificación
func (i *Inode) UpdateModificationTime() {
	i.IMtime = time.Now()
}

// HasIndirectBlocks verifica si el inodo usa bloques indirectos
func (i *Inode) HasIndirectBlocks() bool {
	return i.IBlock[INDIRECT_BLOCK_INDEX] != -1 ||
		i.IBlock[DOUBLE_INDIRECT_BLOCK_INDEX] != -1 ||
		i.IBlock[TRIPLE_INDIRECT_BLOCK_INDEX] != -1
}

// IncreaseSize incrementa el tamaño del archivo en la cantidad especificada
func (i *Inode) IncreaseSize(additionalBytes int32) {
	i.ISize += additionalBytes
	i.UpdateModificationTime()
}

// DecreaseSize reduce el tamaño del archivo en la cantidad especificada
func (i *Inode) DecreaseSize(bytesToRemove int32) {
	if bytesToRemove > i.ISize {
		i.ISize = 0
	} else {
		i.ISize -= bytesToRemove
	}
	i.UpdateModificationTime()
}

// ClearBlocks elimina todos los punteros a bloques
func (i *Inode) ClearBlocks() {
	for idx := range i.IBlock {
		i.IBlock[idx] = -1
	}
	i.UpdateModificationTime()
}

// GetDirectBlockCount devuelve el número de bloques directos utilizados
func (i *Inode) GetDirectBlockCount() int {
	count := 0
	for idx := 0; idx < INDIRECT_BLOCK_INDEX; idx++ {
		if i.IBlock[idx] != -1 {
			count++
		}
	}
	return count
}
