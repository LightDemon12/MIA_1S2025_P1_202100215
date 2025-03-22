package DiskManager

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CreateEXT2Directory crea un directorio en el sistema de archivos EXT2
// Implementación segura para evitar corrupción de otros archivos
// CreateEXT2Directory crea un directorio en el sistema de archivos EXT2
// Versión mejorada con soporte para bloques indirectos
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

	// 5. SEGURIDAD: Proteger users.txt (como en la versión original)
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

	// 10. Encontrar un inodo libre
	freeInodeNum := findSafeInodeNum(inodeBitmap, int(superblock.SInodesCount))
	if freeInodeNum < 0 {
		return fmt.Errorf("no hay inodos libres disponibles")
	}

	// 11. NUEVO: Encontrar y reservar bloques para el directorio (incluyendo indirectos si necesario)
	initialEntries := 2 // "." y ".."
	dirBlocks, indirectBlockNum, err := findSafeBlocksForDirectory(
		file, startByte, superblock, blockBitmap, initialEntries)
	if err != nil {
		return fmt.Errorf("error al reservar bloques para directorio: %v", err)
	}

	// 12. Preparar el nuevo inodo para el directorio
	ownerID := getUserIdFromName(id, owner)
	groupID := getGroupIdFromName(id, ownerGroup)
	if ownerID <= 0 {
		ownerID = 1 // Default a root si no se encuentra
	}
	if groupID <= 0 {
		groupID = 1 // Default a root si no se encuentra
	}

	// Crear y configurar el inodo
	dirInode := &Inode{}
	// Cada entrada ocupa 16 bytes
	initialEntriesSize := int32(initialEntries * 16)

	// NUEVO: Configurar inodo con bloques directos e indirectos
	setupDirectoryInode(dirInode, dirBlocks, indirectBlockNum, initialEntriesSize,
		ownerID, groupID, perms)

	// 13. NUEVO: Inicializar todos los bloques del directorio
	isRootDir := path == "/"
	err = initializeDirectoryBlocks(file, startByte, superblock, dirBlocks,
		int32(freeInodeNum), int32(parentInodeNum), isRootDir)
	if err != nil {
		return fmt.Errorf("error al inicializar bloques del directorio: %v", err)
	}

	// 14. Escribir el inodo del directorio
	inodePos := startByte + int64(superblock.SInodeStart) + int64(freeInodeNum)*int64(superblock.SInodeSize)
	_, err = file.Seek(inodePos, 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para escribir inodo: %v", err)
	}

	err = writeInodeToDisc(file, dirInode)
	if err != nil {
		return fmt.Errorf("error al escribir inodo: %v", err)
	}

	// 15. Actualizar el directorio padre
	// Buscar espacio en el directorio padre (manejando indirectos)
	parentBlockNum, entryIdx, err := findEmptySpaceInDirectoryBlocks(file, startByte, superblock, parentInode)
	if err != nil {
		// Necesitamos añadir un nuevo bloque al directorio padre
		fmt.Printf("Directorio padre lleno. Añadiendo nuevo bloque...\n")

		// NUEVO: Añadir un bloque al directorio padre con soporte para indirectos
		parentBlockNum, err = addBlockToDirectory(file, startByte, superblock, parentInode, blockBitmap)
		if err != nil {
			return fmt.Errorf("no se pudo añadir bloque al directorio padre: %v", err)
		}

		// El primer espacio en el nuevo bloque
		entryIdx = 0

		// Actualizar superbloque por el bloque adicional
		superblock.SFreeBlocksCount--
	}

	// Leer el bloque del directorio padre
	parentBlockPos := startByte + int64(superblock.SBlockStart) + int64(parentBlockNum)*int64(superblock.SBlockSize)
	_, err = file.Seek(parentBlockPos, 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para leer directorio padre: %v", err)
	}

	parentDirBlock, err := ReadDirectoryBlockFromDisc(file, int64(superblock.SBlockSize))
	if err != nil {
		return fmt.Errorf("error al leer bloque de directorio padre: %v", err)
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

	// Aumentar el tamaño del directorio padre
	parentInode.ISize += 16 // Cada entrada ocupa 16 bytes

	// Escribir el bloque actualizado
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

	// 16. Actualizar bitmaps
	// Marcar inodo como usado
	inodeBitmap[freeInodeNum/8] |= (1 << (freeInodeNum % 8))

	// Los bloques ya fueron marcados en las funciones auxiliares

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

	// 17. Actualizar superbloque
	// Restar inodo y bloques usados (incluyendo indirectos)
	superblock.SFreeInodesCount--
	superblock.SFreeBlocksCount -= int32(len(dirBlocks))
	if indirectBlockNum >= 0 {
		superblock.SFreeBlocksCount-- // Por el bloque indirecto
	}

	superblock.SMtime = time.Now()

	_, err = file.Seek(startByte, 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para actualizar superbloque: %v", err)
	}

	err = writeSuperBlockToDisc(file, superblock)
	if err != nil {
		return fmt.Errorf("error al actualizar superbloque: %v", err)
	}

	// Forzar la sincronización de los cambios al disco
	err = file.Sync()
	if err != nil {
		return fmt.Errorf("error al sincronizar cambios con el disco: %v", err)
	}

	// Construir mensaje de resumen
	indirectMsg := ""
	if indirectBlockNum >= 0 {
		indirectMsg = fmt.Sprintf(", usando bloque indirecto %d", indirectBlockNum)
	}

	fmt.Printf("Directorio '%s' creado exitosamente (inodo %d, %d bloques%s)\n",
		path, freeInodeNum, len(dirBlocks), indirectMsg)

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

// writePointerBlockToDisc escribe un bloque de punteros al disco
func writePointerBlockToDisc(file *os.File, pointerBlock *PointerBlock) error {
	for i := 0; i < POINTERS_PER_BLOCK; i++ {
		err := binary.Write(file, binary.LittleEndian, &pointerBlock.BPointers[i])
		if err != nil {
			return fmt.Errorf("error al escribir puntero %d: %v", i, err)
		}
	}
	return nil
}

// readPointerBlockFromDisc lee un bloque de punteros del disco
func readPointerBlockFromDisc(file *os.File, blockSize int64) (*PointerBlock, error) {
	pointerBlock := &PointerBlock{}

	// Leer POINTERS_PER_BLOCK punteros (int32)
	for i := 0; i < POINTERS_PER_BLOCK; i++ {
		err := binary.Read(file, binary.LittleEndian, &pointerBlock.BPointers[i])
		if err != nil {
			return nil, fmt.Errorf("error al leer puntero %d: %v", i, err)
		}
	}

	return pointerBlock, nil
}

// findSafeBlocksForDirectory encuentra y reserva bloques para un directorio,
// incluyendo la posibilidad de usar bloques indirectos si es necesario
func findSafeBlocksForDirectory(file *os.File, startByte int64, superblock *SuperBlock,
	blockBitmap []byte, initialEntriesCount int) ([]int32, int32, error) {

	// Número de entradas que caben en un bloque
	entriesPerBlock := B_CONTENT_COUNT

	// Calcular cuántos bloques necesitamos para estas entradas
	neededBlocks := (initialEntriesCount + entriesPerBlock - 1) / entriesPerBlock
	if neededBlocks <= 0 {
		neededBlocks = 1 // Mínimo un bloque
	}

	// Lista de bloques críticos a evitar
	criticalBlocks := identifyCriticalBlocks(file, startByte, superblock)

	// Resultado: lista de bloques asignados
	blocks := make([]int32, 0, neededBlocks)

	// Encontrar bloques libres
	for i := 0; i < neededBlocks; i++ {
		blockNum := findSafeBlockNum(blockBitmap, int(superblock.SBlocksCount), criticalBlocks)
		if blockNum < 0 {
			return nil, -1, fmt.Errorf("no hay suficientes bloques libres")
		}

		// Marcar el bloque como usado en el bitmap para que no se reutilice
		blockBitmap[blockNum/8] |= (1 << (blockNum % 8))

		// Añadir a nuestra lista
		blocks = append(blocks, int32(blockNum))
	}

	var indirectBlockNum int32 = -1

	// Si necesitamos más de 12 bloques, crear un bloque indirecto simple
	if neededBlocks > 12 {
		// Encontrar un bloque para el indirecto
		indirectBlockIdx := findSafeBlockNum(blockBitmap, int(superblock.SBlocksCount), criticalBlocks)
		if indirectBlockIdx < 0 {
			return nil, -1, fmt.Errorf("no hay bloques libres para indirecto")
		}

		// Marcar el bloque indirecto como usado
		blockBitmap[indirectBlockIdx/8] |= (1 << (indirectBlockIdx % 8))
		indirectBlockNum = int32(indirectBlockIdx)

		// Crear e inicializar el bloque de punteros
		pointerBlock := NewPointerBlock()

		// Añadir punteros para los bloques después del índice 11
		for i := 12; i < len(blocks); i++ {
			pointerBlock.BPointers[i-12] = blocks[i]
		}

		// Escribir el bloque de punteros
		blockPos := startByte + int64(superblock.SBlockStart) +
			int64(indirectBlockNum)*int64(superblock.SBlockSize)
		_, err := file.Seek(blockPos, 0)
		if err != nil {
			return nil, -1, fmt.Errorf("error al posicionarse para escribir bloque indirecto: %v", err)
		}

		err = writePointerBlockToDisc(file, pointerBlock)
		if err != nil {
			return nil, -1, fmt.Errorf("error al escribir bloque indirecto: %v", err)
		}
	}

	return blocks, indirectBlockNum, nil
}

// setupDirectoryInode configura un inodo para un directorio nuevo,
// incluyendo la asignación de bloques directos e indirectos
func setupDirectoryInode(inode *Inode, blocks []int32, indirectBlockNum int32,
	initialEntriesSize int32, owner, group int32, perms []byte) {

	// Configurar propietario y grupo
	inode.IUid = owner
	inode.IGid = group

	// Configurar tamaño inicial basado en las entradas
	inode.ISize = initialEntriesSize

	// Establecer fechas
	now := time.Now()
	inode.IAtime = now
	inode.ICtime = now
	inode.IMtime = now

	// Establecer tipo de inodo (directorio)
	inode.IType = INODE_FOLDER

	// Configurar permisos
	if perms == nil || len(perms) < 3 {
		// Permisos por defecto para directorios: rwxr-xr-x
		inode.IPerm[0] = 7
		inode.IPerm[1] = 5
		inode.IPerm[2] = 5
	} else {
		copy(inode.IPerm[:], perms[:3])
	}

	// Inicializar punteros a bloques (todos a -1 inicialmente)
	for i := 0; i < 15; i++ {
		inode.IBlock[i] = -1
	}

	// Asignar bloques directos
	directBlockCount := min(len(blocks), 12)
	for i := 0; i < directBlockCount; i++ {
		inode.IBlock[i] = blocks[i]
	}

	// Si hay bloque indirecto, asignarlo
	if indirectBlockNum >= 0 {
		inode.IBlock[INDIRECT_BLOCK_INDEX] = indirectBlockNum
	}
}

// initializeDirectoryBlock inicializa un bloque de directorio con entradas por defecto
func initializeDirectoryBlock(dirBlock *DirectoryBlock, selfInodeNum, parentInodeNum int32, isRootDir bool) {
	// Inicializar todas las entradas
	for i := 0; i < B_CONTENT_COUNT; i++ {
		dirBlock.BContent[i].BInodo = -1
		for j := range dirBlock.BContent[i].BName {
			dirBlock.BContent[i].BName[j] = 0
		}
	}

	// Configurar entrada "." (apunta al propio directorio)
	dirBlock.BContent[0].BInodo = selfInodeNum
	copy(dirBlock.BContent[0].BName[:], []byte("."))

	// Configurar entrada ".." (apunta al directorio padre o a sí mismo si es raíz)
	if isRootDir {
		dirBlock.BContent[1].BInodo = selfInodeNum // La raíz es su propio padre
	} else {
		dirBlock.BContent[1].BInodo = parentInodeNum
	}
	copy(dirBlock.BContent[1].BName[:], []byte(".."))
}

// initializeDirectoryBlocks inicializa todos los bloques de un directorio
func initializeDirectoryBlocks(file *os.File, startByte int64, superblock *SuperBlock,
	blocks []int32, selfInodeNum, parentInodeNum int32, isRootDir bool) error {

	for i, blockNum := range blocks {
		dirBlock := &DirectoryBlock{}

		// Solo el primer bloque tiene entradas especiales "." y ".."
		if i == 0 {
			initializeDirectoryBlock(dirBlock, selfInodeNum, parentInodeNum, isRootDir)
		} else {
			// Los bloques adicionales se inicializan vacíos
			for j := 0; j < B_CONTENT_COUNT; j++ {
				dirBlock.BContent[j].BInodo = -1
				for k := range dirBlock.BContent[j].BName {
					dirBlock.BContent[j].BName[k] = 0
				}
			}
		}

		// Escribir el bloque
		blockPos := startByte + int64(superblock.SBlockStart) +
			int64(blockNum)*int64(superblock.SBlockSize)
		_, err := file.Seek(blockPos, 0)
		if err != nil {
			return fmt.Errorf("error al posicionarse para escribir bloque de directorio %d: %v",
				i, err)
		}

		err = writeDirectoryBlockToDisc(file, dirBlock)
		if err != nil {
			return fmt.Errorf("error al escribir bloque de directorio %d: %v", i, err)
		}
	}

	return nil
}

// findEmptySpaceInDirectoryBlocks busca un espacio vacío en los bloques de un directorio
func findEmptySpaceInDirectoryBlocks(file *os.File, startByte int64, superblock *SuperBlock,
	inode *Inode) (blockNum int32, entryIdx int, err error) {
	blocksStart := startByte + int64(superblock.SBlockStart)

	// Primero revisar bloques directos
	for i := 0; i < 12; i++ {
		blockNum := inode.IBlock[i]
		if blockNum <= 0 {
			continue
		}

		// Leer el bloque
		blockPos := blocksStart + int64(blockNum)*int64(superblock.SBlockSize)
		_, err := file.Seek(blockPos, 0)
		if err != nil {
			continue
		}

		dirBlock, err := ReadDirectoryBlockFromDisc(file, int64(superblock.SBlockSize))
		if err != nil {
			continue
		}

		// Buscar una entrada vacía
		for j := 0; j < B_CONTENT_COUNT; j++ {
			if dirBlock.BContent[j].BInodo <= 0 {
				return blockNum, j, nil
			}
		}
	}

	// Si no hay en bloques directos, revisar indirecto simple
	if inode.IBlock[INDIRECT_BLOCK_INDEX] > 0 {
		indirectBlockPos := blocksStart + int64(inode.IBlock[INDIRECT_BLOCK_INDEX])*
			int64(superblock.SBlockSize)
		_, err := file.Seek(indirectBlockPos, 0)
		if err != nil {
			return -1, -1, fmt.Errorf("error al leer bloque indirecto: %v", err)
		}

		pointerBlock, err := readPointerBlockFromDisc(file, int64(superblock.SBlockSize))
		if err != nil {
			return -1, -1, fmt.Errorf("error al leer punteros: %v", err)
		}

		// Revisar cada bloque referenciado
		for i := 0; i < POINTERS_PER_BLOCK; i++ {
			blockNum := pointerBlock.BPointers[i]
			if blockNum <= 0 || blockNum == POINTER_UNUSED_VALUE {
				continue
			}

			// Leer el bloque
			blockPos := blocksStart + int64(blockNum)*int64(superblock.SBlockSize)
			_, err := file.Seek(blockPos, 0)
			if err != nil {
				continue
			}

			dirBlock, err := ReadDirectoryBlockFromDisc(file, int64(superblock.SBlockSize))
			if err != nil {
				continue
			}

			// Buscar una entrada vacía
			for j := 0; j < B_CONTENT_COUNT; j++ {
				if dirBlock.BContent[j].BInodo <= 0 {
					return blockNum, j, nil
				}
			}
		}
	}

	// No se encontró espacio
	return -1, -1, fmt.Errorf("directorio lleno")
}

// addBlockToDirectory añade un nuevo bloque a un directorio, manejando indirectos si es necesario
func addBlockToDirectory(file *os.File, startByte int64, superblock *SuperBlock,
	inode *Inode, blockBitmap []byte) (int32, error) {

	// 1. Encontrar un bloque libre
	criticalBlocks := identifyCriticalBlocks(file, startByte, superblock)
	newBlockNum := findSafeBlockNum(blockBitmap, int(superblock.SBlocksCount), criticalBlocks)
	if newBlockNum < 0 {
		return -1, fmt.Errorf("no hay bloques libres disponibles")
	}

	// Marcar el bloque como usado
	blockBitmap[newBlockNum/8] |= (1 << (newBlockNum % 8))

	// 2. Inicializar el nuevo bloque de directorio
	dirBlock := &DirectoryBlock{}
	for i := 0; i < B_CONTENT_COUNT; i++ {
		dirBlock.BContent[i].BInodo = -1
		for j := range dirBlock.BContent[i].BName {
			dirBlock.BContent[i].BName[j] = 0
		}
	}

	// 3. Escribir el bloque
	blockPos := startByte + int64(superblock.SBlockStart) +
		int64(newBlockNum)*int64(superblock.SBlockSize)
	_, err := file.Seek(blockPos, 0)
	if err != nil {
		return -1, fmt.Errorf("error al posicionarse para escribir nuevo bloque: %v", err)
	}

	err = writeDirectoryBlockToDisc(file, dirBlock)
	if err != nil {
		return -1, fmt.Errorf("error al escribir nuevo bloque: %v", err)
	}

	// 4. Intentar añadir a un bloque directo primero
	for i := 0; i < 12; i++ {
		if inode.IBlock[i] <= 0 {
			inode.IBlock[i] = int32(newBlockNum)
			return int32(newBlockNum), nil
		}
	}

	// 5. Si no hay espacio en directos, usar indirecto simple
	if inode.IBlock[INDIRECT_BLOCK_INDEX] <= 0 {
		// No hay bloque indirecto, crear uno
		indirectBlockNum := findSafeBlockNum(blockBitmap, int(superblock.SBlocksCount), criticalBlocks)
		if indirectBlockNum < 0 {
			return -1, fmt.Errorf("no hay bloques libres para indirecto")
		}

		// Marcar el bloque indirecto como usado
		blockBitmap[indirectBlockNum/8] |= (1 << (indirectBlockNum % 8))

		// Crear e inicializar el bloque de punteros
		pointerBlock := NewPointerBlock()
		pointerBlock.BPointers[0] = int32(newBlockNum)

		// Escribir el bloque de punteros
		indirectBlockPos := startByte + int64(superblock.SBlockStart) +
			int64(indirectBlockNum)*int64(superblock.SBlockSize)
		_, err := file.Seek(indirectBlockPos, 0)
		if err != nil {
			return -1, fmt.Errorf("error al posicionarse para escribir bloque indirecto: %v", err)
		}

		err = writePointerBlockToDisc(file, pointerBlock)
		if err != nil {
			return -1, fmt.Errorf("error al escribir bloque indirecto: %v", err)
		}

		// Actualizar el inodo
		inode.IBlock[INDIRECT_BLOCK_INDEX] = int32(indirectBlockNum)

	} else {
		// Ya hay bloque indirecto, añadir ahí
		indirectBlockPos := startByte + int64(superblock.SBlockStart) +
			int64(inode.IBlock[INDIRECT_BLOCK_INDEX])*int64(superblock.SBlockSize)
		_, err := file.Seek(indirectBlockPos, 0)
		if err != nil {
			return -1, fmt.Errorf("error al leer bloque indirecto: %v", err)
		}

		pointerBlock, err := readPointerBlockFromDisc(file, int64(superblock.SBlockSize))
		if err != nil {
			return -1, fmt.Errorf("error al leer punteros: %v", err)
		}

		// Encontrar un espacio libre en el bloque indirecto
		freeIndex := -1
		for i := 0; i < POINTERS_PER_BLOCK; i++ {
			if pointerBlock.BPointers[i] <= 0 || pointerBlock.BPointers[i] == POINTER_UNUSED_VALUE {
				freeIndex = i
				break
			}
		}

		if freeIndex < 0 {
			return -1, fmt.Errorf("bloque indirecto lleno")
		}

		// Actualizar el puntero
		pointerBlock.BPointers[freeIndex] = int32(newBlockNum)

		// Escribir el bloque actualizado
		_, err = file.Seek(indirectBlockPos, 0)
		if err != nil {
			return -1, fmt.Errorf("error al posicionarse para actualizar bloque indirecto: %v", err)
		}

		err = writePointerBlockToDisc(file, pointerBlock)
		if err != nil {
			return -1, fmt.Errorf("error al actualizar bloque indirecto: %v", err)
		}
	}

	return int32(newBlockNum), nil
}

// loadInodeBitmap carga el bitmap de inodos
func loadInodeBitmap(file *os.File, startByte int64, superblock *SuperBlock) ([]byte, error) {
	// Tamaño del bitmap en bytes (redondeado hacia arriba)
	bitmapSize := (superblock.SInodesCount + 7) / 8

	// Posicionarse al inicio del bitmap
	_, err := file.Seek(startByte+int64(superblock.SBmInodeStart), 0)
	if err != nil {
		return nil, fmt.Errorf("error al posicionarse en bitmap de inodos: %v", err)
	}

	// Leer el bitmap
	bitmap := make([]byte, bitmapSize)
	_, err = file.Read(bitmap)
	if err != nil {
		return nil, fmt.Errorf("error al leer bitmap de inodos: %v", err)
	}

	return bitmap, nil
}

// loadBlockBitmap carga el bitmap de bloques
func loadBlockBitmap(file *os.File, startByte int64, superblock *SuperBlock) ([]byte, error) {
	// Tamaño del bitmap en bytes (redondeado hacia arriba)
	bitmapSize := (superblock.SBlocksCount + 7) / 8

	// Posicionarse al inicio del bitmap
	_, err := file.Seek(startByte+int64(superblock.SBmBlockStart), 0)
	if err != nil {
		return nil, fmt.Errorf("error al posicionarse en bitmap de bloques: %v", err)
	}

	// Leer el bitmap
	bitmap := make([]byte, bitmapSize)
	_, err = file.Read(bitmap)
	if err != nil {
		return nil, fmt.Errorf("error al leer bitmap de bloques: %v", err)
	}

	return bitmap, nil
}
