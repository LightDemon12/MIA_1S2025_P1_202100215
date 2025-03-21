package DiskManager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CreateEXT2Directory crea un directorio en el sistema de archivos EXT2
// Implementación segura para evitar corrupción de otros archivos
func CreateEXT2Directory(id, path string, owner, ownerGroup string, perms []byte) error {
	fmt.Printf("CreateEXT2Directory: Creando directorio '%s'\n", path)
	// 1. Verificar la partición montada
	mountedPartition, err := FindMountedPartitionById(id)
	if err != nil {
		return fmt.Errorf("partición no encontrada: %v", err)
	}

	// 2. Abrir el disco en modo exclusivo para evitar interferencias
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("error al abrir disco: %v", err)
	}
	defer file.Close()

	// 3. Obtener la posición de inicio de la partición
	startByte, _, err := GetPartitionDetails(file, mountedPartition)
	if err != nil {
		return fmt.Errorf("error obteniendo detalles de partición: %v", err)
	}

	// 4. Leer el superbloque
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para leer superbloque: %v", err)
	}

	superblock, err := ReadSuperBlockFromDisc(file)
	if err != nil {
		return fmt.Errorf("error al leer superbloque: %v", err)
	}

	// 5. SEGURIDAD: Proteger users.txt
	usersInodeNum := 3 // Sabemos que es el inodo 3
	usersInodePos := startByte + int64(superblock.SInodeStart) + int64(usersInodeNum)*int64(superblock.SInodeSize)
	_, err = file.Seek(usersInodePos, 0)
	if err == nil {
		usersInode, readErr := readInodeFromDisc(file)
		if readErr == nil {
			// Guardar copia del inodo para restaurarlo después si es necesario
			usersInodeCopy := *usersInode

			// SEGURIDAD: Restaurar users.txt al final, pase lo que pase
			defer func() {
				fmt.Printf("SEGURIDAD: Verificando integridad del inodo %d (users.txt)\n", usersInodeNum)
				_, err := file.Seek(usersInodePos, 0)
				if err == nil {
					currentInode, readErr := readInodeFromDisc(file)
					if readErr == nil {
						// Comparar inodos para ver si cambió
						if !compareInodes(currentInode, &usersInodeCopy) {
							fmt.Printf("ALERTA: Inodo %d (users.txt) fue modificado. Restaurando...\n", usersInodeNum)
							_, err = file.Seek(usersInodePos, 0)
							if err == nil {
								writeErr := writeInodeToDisc(file, &usersInodeCopy)
								if writeErr != nil {
									fmt.Printf("ERROR: No se pudo restaurar inodo %d: %v\n", usersInodeNum, writeErr)
								} else {
									fmt.Printf("REPARACIÓN: Inodo %d restaurado exitosamente\n", usersInodeNum)
								}
							}
						}
					}
				}
			}()
		}
	}

	// 6. Normalizar y analizar la ruta del directorio
	if path != "/" && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}

	dirPath := filepath.Dir(path)
	if dirPath == "." {
		dirPath = "/"
	}
	dirName := filepath.Base(path)

	// Verificar si necesitamos crear el directorio padre primero
	parentExists, _ := FileExists(id, dirPath)
	if !parentExists && dirPath != "/" {
		fmt.Printf("Directorio padre '%s' no existe, creándolo primero...\n", dirPath)

		// Crear el directorio padre recursivamente
		err = CreateEXT2Directory(id, dirPath, owner, ownerGroup, perms)
		if err != nil {
			return fmt.Errorf("error al crear directorio padre '%s': %v", dirPath, err)
		}
	}

	// 7. Buscar el directorio padre (ahora debería existir)
	parentInodeNum, parentInode, err := FindInodeByPath(file, startByte, superblock, dirPath)
	if err != nil {
		// Si el directorio padre no existe, intentar crearlo
		if dirPath != "/" {
			fmt.Printf("Creando directorio padre '%s'...\n", dirPath)

			// Llamada recursiva para crear el directorio padre
			err = CreateEXT2Directory(id, dirPath, owner, ownerGroup, perms)
			if err != nil {
				return fmt.Errorf("error creando directorio padre '%s': %v", dirPath, err)
			}

			// Intentar obtener el inodo del directorio padre recién creado
			parentInodeNum, parentInode, err = FindInodeByPath(file, startByte, superblock, dirPath)
			if err != nil {
				return fmt.Errorf("error crítico: directorio padre no encontrado después de crearlo: %v", err)
			}
		}
	}

	// 8. Verificar si el directorio ya existe
	_, _, err = FindInodeByPath(file, startByte, superblock, path)
	if err == nil {
		return fmt.Errorf("ya existe un archivo o directorio en '%s'", path)
	}

	// 9. Cargar bitmaps de inodos y bloques
	inodeBitmap, err := loadInodeBitmap(file, startByte, superblock)
	if err != nil {
		return fmt.Errorf("error cargando bitmap de inodos: %v", err)
	}

	blockBitmap, err := loadBlockBitmap(file, startByte, superblock)
	if err != nil {
		return fmt.Errorf("error cargando bitmap de bloques: %v", err)
	}

	// 10. Identificar bloques críticos
	criticalBlocks := identifyCriticalBlocks(file, startByte, superblock)

	// 11. Encontrar un inodo libre
	freeInodeNum := findSafeInodeNum(inodeBitmap, int(superblock.SInodesCount))
	if freeInodeNum < 0 {
		return fmt.Errorf("no hay inodos libres disponibles")
	}

	// 12. Encontrar un bloque libre para el directorio
	freeBlockNum := findSafeBlockNum(blockBitmap, int(superblock.SBlocksCount), criticalBlocks)
	if freeBlockNum < 0 {
		return fmt.Errorf("no hay bloques libres disponibles")
	}

	// 13. Preparar el nuevo inodo para el directorio
	ownerID := getUserIdFromName(id, owner)
	groupID := getGroupIdFromName(id, ownerGroup)
	if ownerID <= 0 {
		ownerID = 1 // Default a root si no se encuentra
	}
	if groupID <= 0 {
		groupID = 1 // Default a root si no se encuentra
	}

	dirInode := &Inode{}
	dirInode.IUid = ownerID
	dirInode.IGid = groupID
	dirInode.ISize = 2 * 16 // Tamaño inicial para entradas "." y ".."
	dirInode.IAtime = time.Now()
	dirInode.ICtime = time.Now()
	dirInode.IMtime = time.Now()
	dirInode.IType = INODE_FOLDER

	// Configurar permisos
	if perms == nil || len(perms) < 3 {
		// Permisos por defecto para directorios: rwxr-xr-x
		dirInode.IPerm[0] = 7
		dirInode.IPerm[1] = 5
		dirInode.IPerm[2] = 5
	} else {
		copy(dirInode.IPerm[:], perms[:3])
	}

	// Inicializar punteros a bloques
	for i := 0; i < 15; i++ {
		dirInode.IBlock[i] = -1
	}
	dirInode.IBlock[0] = int32(freeBlockNum)

	// 14. Crear y escribir el bloque de directorio
	dirBlock := &DirectoryBlock{}

	// Inicializar todas las entradas
	for i := 0; i < B_CONTENT_COUNT; i++ {
		dirBlock.BContent[i].BInodo = -1
		for j := range dirBlock.BContent[i].BName {
			dirBlock.BContent[i].BName[j] = 0
		}
	}

	// Configurar entrada "." (apunta al propio directorio)
	dirBlock.BContent[0].BInodo = int32(freeInodeNum)
	copy(dirBlock.BContent[0].BName[:], []byte("."))

	// Configurar entrada ".." (apunta al directorio padre)
	dirBlock.BContent[1].BInodo = int32(parentInodeNum)
	copy(dirBlock.BContent[1].BName[:], []byte(".."))

	// Escribir el bloque de directorio
	blockPos := startByte + int64(superblock.SBlockStart) + int64(freeBlockNum)*int64(superblock.SBlockSize)
	_, err = file.Seek(blockPos, 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para escribir bloque de directorio: %v", err)
	}

	err = writeDirectoryBlockToDisc(file, dirBlock)
	if err != nil {
		return fmt.Errorf("error al escribir bloque de directorio: %v", err)
	}

	// 15. Escribir el inodo del directorio
	inodePos := startByte + int64(superblock.SInodeStart) + int64(freeInodeNum)*int64(superblock.SInodeSize)
	_, err = file.Seek(inodePos, 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para escribir inodo: %v", err)
	}

	err = writeInodeToDisc(file, dirInode)
	if err != nil {
		return fmt.Errorf("error al escribir inodo: %v", err)
	}

	// 16. Actualizar el directorio padre
	parentDirBlock, parentBlockNum, err := findDirectoryBlockWithSpace(file, startByte, superblock, parentInode)
	if err != nil {
		return fmt.Errorf("error al buscar espacio en directorio padre: %v", err)
	}

	// Buscar una entrada libre en el directorio padre
	entryIdx := -1
	for i := 0; i < B_CONTENT_COUNT; i++ {
		if parentDirBlock.BContent[i].BInodo <= 0 {
			entryIdx = i
			break
		}
	}

	if entryIdx < 0 {
		return fmt.Errorf("directorio padre está lleno")
	}

	// Obtener el nombre del directorio a crear
	dirName = filepath.Base(path)
	fmt.Printf("DEBUG: Añadiendo entrada '%s' (inodo %d) al directorio padre en bloque %d, posición %d\n",
		dirName, freeInodeNum, parentBlockNum, entryIdx)

	// Limpiar y preparar la nueva entrada
	for i := range parentDirBlock.BContent[entryIdx].BName {
		parentDirBlock.BContent[entryIdx].BName[i] = 0
	}

	// Copiar el nombre con límite seguro
	nameLen := len(dirName)
	if nameLen > 12 {
		nameLen = 12
	}
	copy(parentDirBlock.BContent[entryIdx].BName[:], []byte(dirName))
	parentDirBlock.BContent[entryIdx].BInodo = int32(freeInodeNum)

	// Asegurarnos de que el tamaño del directorio padre se actualice
	parentInode.ISize += 16 // Cada entrada ocupa 16 bytes

	// Escribir el bloque del directorio padre actualizado
	parentBlockPos := startByte + int64(superblock.SBlockStart) + int64(parentBlockNum)*int64(superblock.SBlockSize)
	_, err = file.Seek(parentBlockPos, 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para actualizar directorio padre: %v", err)
	}

	err = writeDirectoryBlockToDisc(file, parentDirBlock)
	if err != nil {
		return fmt.Errorf("error al actualizar directorio padre: %v", err)
	}

	// Actualizar el inodo del directorio padre
	parentInodePos := startByte + int64(superblock.SInodeStart) + int64(parentInodeNum)*int64(superblock.SInodeSize)
	_, err = file.Seek(parentInodePos, 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para actualizar inodo padre: %v", err)
	}

	err = writeInodeToDisc(file, parentInode)
	if err != nil {
		return fmt.Errorf("error al actualizar inodo padre: %v", err)
	}

	// Forzar la sincronización de los cambios al disco
	err = file.Sync()
	if err != nil {
		return fmt.Errorf("error al sincronizar cambios con el disco: %v", err)
	}

	// Verificar que la entrada se creó correctamente
	_, err = file.Seek(parentBlockPos, 0)
	if err != nil {
		return fmt.Errorf("error al verificar entrada creada: %v", err)
	}

	verifyBlock, err := ReadDirectoryBlockFromDisc(file, int64(superblock.SBlockSize))
	if err != nil {
		return fmt.Errorf("error al leer bloque para verificación: %v", err)
	}

	// Verificar la entrada
	found := false
	for i := 0; i < B_CONTENT_COUNT; i++ {
		if verifyBlock.BContent[i].BInodo == int32(freeInodeNum) {
			entryName := strings.TrimRight(string(verifyBlock.BContent[i].BName[:]), "\x00")
			if entryName == dirName {
				found = true
				fmt.Printf("DEBUG: Verificación exitosa - Entrada '%s' encontrada en directorio padre\n",
					entryName)
				break
			}
		}
	}

	if !found {
		return fmt.Errorf("error crítico: la entrada del directorio no se guardó correctamente")
	}

	// 17. Actualizar bitmaps
	// Marcar inodo como usado
	inodeBitmap[freeInodeNum/8] |= (1 << (freeInodeNum % 8))

	// Marcar bloque como usado
	blockBitmap[freeBlockNum/8] |= (1 << (freeBlockNum % 8))

	// Escribir bitmap de inodos actualizado
	_, err = file.Seek(startByte+int64(superblock.SBmInodeStart), 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para actualizar bitmap de inodos: %v", err)
	}

	_, err = file.Write(inodeBitmap)
	if err != nil {
		return fmt.Errorf("error al actualizar bitmap de inodos: %v", err)
	}

	// Escribir bitmap de bloques actualizado
	_, err = file.Seek(startByte+int64(superblock.SBmBlockStart), 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para actualizar bitmap de bloques: %v", err)
	}

	_, err = file.Write(blockBitmap)
	if err != nil {
		return fmt.Errorf("error al actualizar bitmap de bloques: %v", err)
	}

	// 18. Actualizar superbloque
	superblock.SFreeInodesCount--
	superblock.SFreeBlocksCount--
	superblock.SMtime = time.Now()

	_, err = file.Seek(startByte, 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para actualizar superbloque: %v", err)
	}

	err = writeSuperBlockToDisc(file, superblock)
	if err != nil {
		return fmt.Errorf("error al actualizar superbloque: %v", err)
	}

	fmt.Printf("Directorio '%s' creado exitosamente (inodo %d, bloque %d)\n", path, freeInodeNum, freeBlockNum)
	return nil
}

// CreateEXT2DirectoryRecursive crea un directorio y todos los directorios padres necesarios
func CreateEXT2DirectoryRecursive(id, path string, owner, ownerGroup string, perms []byte) error {
	fmt.Printf("CreateEXT2DirectoryRecursive: Creando ruta '%s'\n", path)

	// Normalizar la ruta
	if path == "" || path == "/" {
		return nil // La raíz ya existe, nada que hacer
	}

	// Si ya termina en /, eliminar ese slash final
	if path != "/" && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}

	// Dividir la ruta en componentes
	components := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(components) == 0 {
		return nil // La raíz ya existe
	}

	// Crear cada directorio del camino si no existe
	currentPath := "/"
	for i, component := range components {
		if component == "" {
			continue
		}

		// Construir la ruta para este nivel
		if currentPath == "/" {
			currentPath = "/" + component
		} else {
			currentPath = currentPath + "/" + component
		}

		fmt.Printf("Verificando si existe: %s\n", currentPath)

		// Verificar si este directorio ya existe
		exists, isDir, err := fileExistsAndType(id, currentPath)

		if err != nil {
			// Error al verificar, probablemente no existe
			exists = false
		}

		if exists {
			// Si existe pero no es un directorio (excepto el último componente)
			if !isDir && i < len(components)-1 {
				return fmt.Errorf("'%s' existe pero no es un directorio", currentPath)
			}

			// Si existe y es del tipo correcto, continuamos con el siguiente
			continue
		}

		// No existe, hay que crearlo
		fmt.Printf("Creando directorio intermedio: %s\n", currentPath)

		err = CreateEXT2Directory(id, currentPath, owner, ownerGroup, perms)
		if err != nil {
			return fmt.Errorf("error creando directorio '%s': %v", currentPath, err)
		}
	}

	return nil
}

// fileExistsAndType verifica si un archivo existe y si es un directorio
func fileExistsAndType(id, path string) (exists bool, isDir bool, err error) {
	// 1. Verificar la partición montada
	mountedPartition, err := FindMountedPartitionById(id)
	if err != nil {
		return false, false, err
	}

	// 2. Abrir el disco
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return false, false, err
	}
	defer file.Close()

	// 3. Obtener la posición de inicio de la partición
	startByte, _, err := GetPartitionDetails(file, mountedPartition)
	if err != nil {
		return false, false, err
	}

	// 4. Leer el superbloque
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, false, err
	}

	superblock, err := ReadSuperBlockFromDisc(file)
	if err != nil {
		return false, false, err
	}

	// 5. Buscar el inodo
	_, inode, err := FindInodeByPath(file, startByte, superblock, path)
	if err != nil {
		return false, false, err
	}

	// Si llegamos aquí, el archivo/directorio existe
	return true, inode.IType == INODE_FOLDER, nil
}
