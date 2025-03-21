package DiskManager

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CreateEXT2File crea un archivo con contenido en el sistema de archivos EXT2
// Implementación segura para evitar corrupción de otros archivos
func CreateEXT2File(id, path, content string, owner, ownerGroup string, perms []byte) error {
	fmt.Printf("CreateEXT2File: Creando archivo '%s'\n", path)

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

	// 5. SEGURIDAD: Hacer un snapshot del inodo de users.txt para garantizar su integridad
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

	// 6. Analizar la ruta del archivo
	dirPath := filepath.Dir(path)
	if dirPath == "." {
		dirPath = "/"
	}
	fileName := filepath.Base(path)

	if len(fileName) > 12 {
		return fmt.Errorf("nombre de archivo demasiado largo (máximo 12 caracteres)")
	}

	// 7. Buscar el directorio padre
	_, parentInode, err := FindInodeByPath(file, startByte, superblock, dirPath)
	if err != nil {
		return fmt.Errorf("directorio padre no encontrado: %v", err)
	}

	if parentInode.IType != INODE_FOLDER {
		return fmt.Errorf("la ruta '%s' no es un directorio", dirPath)
	}

	// 8. Verificar si el archivo ya existe
	_, _, err = FindInodeByPath(file, startByte, superblock, path)
	if err == nil {
		return fmt.Errorf("ya existe un archivo o directorio en '%s'", path)
	}

	// 9. Cargar bitmaps de inodos y bloques
	// Bitmap de inodos
	inodeBitmap, err := loadInodeBitmap(file, startByte, superblock)
	if err != nil {
		return fmt.Errorf("error cargando bitmap de inodos: %v", err)
	}

	// Bitmap de bloques
	blockBitmap, err := loadBlockBitmap(file, startByte, superblock)
	if err != nil {
		return fmt.Errorf("error cargando bitmap de bloques: %v", err)
	}

	// 10. SEGURIDAD: Identificar bloques usados por archivos críticos
	criticalBlocks := identifyCriticalBlocks(file, startByte, superblock)

	// 11. Encontrar un inodo libre lejos de los inodos críticos
	freeInodeNum := findSafeInodeNum(inodeBitmap, int(superblock.SInodesCount))
	if freeInodeNum < 0 {
		return fmt.Errorf("no hay inodos libres disponibles")
	}

	// 12. Encontrar un bloque libre lejos de los bloques críticos
	// CORREGIDO: Convertir int32 a int
	freeBlockNum := findSafeBlockNum(blockBitmap, int(superblock.SBlocksCount), criticalBlocks)
	if freeBlockNum < 0 {
		return fmt.Errorf("no hay bloques libres disponibles")
	}

	// 13. Preparar buffer para escritura del contenido
	contentBytes := []byte(content)
	blockSize := int(superblock.SBlockSize)
	dataBuffer := make([]byte, blockSize)

	// Copiar contenido al buffer con cuidado para evitar desbordamientos
	contentLen := len(contentBytes)
	if contentLen > blockSize {
		contentLen = blockSize
	}
	copy(dataBuffer[:contentLen], contentBytes[:contentLen])

	// Rellenar el resto del buffer con un patrón seguro
	for i := contentLen; i < blockSize; i++ {
		dataBuffer[i] = byte('-') // Usar un caracter seguro
	}

	// 14. Escribir el contenido al bloque
	blockPos := startByte + int64(superblock.SBlockStart) + int64(freeBlockNum)*int64(blockSize)
	_, err = file.Seek(blockPos, 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para escribir bloque: %v", err)
	}

	_, err = file.Write(dataBuffer)
	if err != nil {
		return fmt.Errorf("error al escribir bloque de datos: %v", err)
	}

	// 15. Obtener IDs de usuario y grupo
	ownerID := getUserIdFromName(id, owner)
	groupID := getGroupIdFromName(id, ownerGroup)
	if ownerID <= 0 {
		ownerID = 1 // Default a root si no se encuentra
	}
	if groupID <= 0 {
		groupID = 1 // Default a root si no se encuentra
	}

	// 16. Crear el inodo para el archivo
	fileInode := &Inode{}
	fileInode.IUid = ownerID
	fileInode.IGid = groupID
	fileInode.ISize = int32(len(content))
	fileInode.IAtime = time.Now()
	fileInode.ICtime = time.Now()
	fileInode.IMtime = time.Now()
	fileInode.IType = INODE_FILE

	// Configurar permisos
	if perms == nil || len(perms) < 3 {
		// Permisos por defecto: rw-r--r--
		fileInode.IPerm[0] = 6
		fileInode.IPerm[1] = 4
		fileInode.IPerm[2] = 4
	} else {
		copy(fileInode.IPerm[:], perms[:3])
	}

	// Inicializar punteros a bloques
	for i := 0; i < 15; i++ {
		fileInode.IBlock[i] = -1
	}
	fileInode.IBlock[0] = int32(freeBlockNum)

	// 17. Escribir el inodo
	inodePos := startByte + int64(superblock.SInodeStart) + int64(freeInodeNum)*int64(superblock.SInodeSize)
	_, err = file.Seek(inodePos, 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para escribir inodo: %v", err)
	}

	err = writeInodeToDisc(file, fileInode)
	if err != nil {
		return fmt.Errorf("error al escribir inodo: %v", err)
	}

	// 18. Actualizar el directorio padre
	parentDirBlock, blockNum, err := findDirectoryBlockWithSpace(file, startByte, superblock, parentInode)
	if err != nil {
		return fmt.Errorf("error al leer directorio padre: %v", err)
	}

	// Buscar una entrada libre en el directorio
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

	// Limpiar la entrada y asignar el nuevo archivo
	for i := range parentDirBlock.BContent[entryIdx].BName {
		parentDirBlock.BContent[entryIdx].BName[i] = 0
	}

	// Copiar el nombre con límite seguro
	nameLen := len(fileName)
	if nameLen > 12 {
		nameLen = 12
	}
	copy(parentDirBlock.BContent[entryIdx].BName[:nameLen], []byte(fileName[:nameLen]))
	parentDirBlock.BContent[entryIdx].BInodo = int32(freeInodeNum)

	// Escribir el bloque de directorio actualizado
	dirBlockPos := startByte + int64(superblock.SBlockStart) + int64(blockNum)*int64(superblock.SBlockSize)
	_, err = file.Seek(dirBlockPos, 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para actualizar directorio: %v", err)
	}

	err = writeDirectoryBlockToDisc(file, parentDirBlock)
	if err != nil {
		return fmt.Errorf("error al actualizar directorio padre: %v", err)
	}

	// 19. Actualizar bitmaps
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

	// 20. Actualizar superbloque
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

	fmt.Printf("Archivo '%s' creado exitosamente (inodo %d, bloque %d)\n", path, freeInodeNum, freeBlockNum)
	return nil
}

// Funciones auxiliares seguras

// loadInodeBitmap carga el bitmap de inodos completo
func loadInodeBitmap(file *os.File, startByte int64, sb *SuperBlock) ([]byte, error) {
	bitmapSize := sb.SInodesCount/8 + 1
	bitmap := make([]byte, bitmapSize)

	_, err := file.Seek(startByte+int64(sb.SBmInodeStart), 0)
	if err != nil {
		return nil, err
	}

	_, err = file.Read(bitmap)
	if err != nil {
		return nil, err
	}

	return bitmap, nil
}

// loadBlockBitmap carga el bitmap de bloques completo
func loadBlockBitmap(file *os.File, startByte int64, sb *SuperBlock) ([]byte, error) {
	bitmapSize := sb.SBlocksCount/8 + 1
	bitmap := make([]byte, bitmapSize)

	_, err := file.Seek(startByte+int64(sb.SBmBlockStart), 0)
	if err != nil {
		return nil, err
	}

	_, err = file.Read(bitmap)
	if err != nil {
		return nil, err
	}

	return bitmap, nil
}

// identifyCriticalBlocks identifica bloques usados por archivos críticos del sistema
func identifyCriticalBlocks(file *os.File, startByte int64, sb *SuperBlock) map[int32]bool {
	criticalBlocks := make(map[int32]bool)

	// Proteger especialmente los bloques de users.txt (inodo 3)
	usersInodePos := startByte + int64(sb.SInodeStart) + 3*int64(sb.SInodeSize)
	_, err := file.Seek(usersInodePos, 0)
	if err == nil {
		usersInode, readErr := readInodeFromDisc(file)
		if readErr == nil {
			// Marcar todos los bloques de users.txt como críticos
			for i := 0; i < 12; i++ {
				if usersInode.IBlock[i] > 0 {
					blockNum := usersInode.IBlock[i]
					criticalBlocks[blockNum] = true

					// También marcar una zona de seguridad alrededor
					for j := int32(1); j <= 20; j++ {
						if blockNum-j >= 0 {
							criticalBlocks[blockNum-j] = true
						}
						criticalBlocks[blockNum+j] = true
					}
				}
			}
		}
	}

	// También proteger los primeros 50 bloques donde suelen estar los archivos críticos
	for i := int32(0); i < 50; i++ {
		criticalBlocks[i] = true
	}

	return criticalBlocks
}

// findSafeInodeNum encuentra un inodo libre que esté a una distancia segura de inodos críticos
// CORREGIDO: Cambiado el tipo de maxInodes a int
func findSafeInodeNum(bitmap []byte, maxInodes int) int {
	// Comenzar desde el inodo 15 para evitar los inodos del sistema
	for i := 15; i < maxInodes; i++ {
		bytePos := i / 8
		bitPos := i % 8

		if bytePos < len(bitmap) && (bitmap[bytePos]&(1<<bitPos)) == 0 {
			return i
		}
	}
	return -1
}

// findSafeBlockNum encuentra un bloque libre que no esté en la lista de bloques críticos
// CORREGIDO: Cambiado el tipo de maxBlocks a int
func findSafeBlockNum(bitmap []byte, maxBlocks int, criticalBlocks map[int32]bool) int {
	// Primero intentar bloques a partir del 100 para mayor seguridad
	for i := 100; i < maxBlocks; i++ {
		bytePos := i / 8
		bitPos := i % 8

		if bytePos < len(bitmap) && (bitmap[bytePos]&(1<<bitPos)) == 0 {
			if !criticalBlocks[int32(i)] {
				return i
			}
		}
	}

	// Si no encontramos en esa zona, buscar en cualquier bloque no crítico
	for i := 50; i < maxBlocks; i++ {
		bytePos := i / 8
		bitPos := i % 8

		if bytePos < len(bitmap) && (bitmap[bytePos]&(1<<bitPos)) == 0 {
			if !criticalBlocks[int32(i)] {
				return i
			}
		}
	}

	return -1
}

// findFirstDirectoryBlock encuentra el primer bloque de un directorio
func findFirstDirectoryBlock(file *os.File, startByte int64, sb *SuperBlock, inode *Inode) (*DirectoryBlock, int32, error) {
	// Solo consideramos el primer bloque directo por ahora
	if inode.IBlock[0] <= 0 {
		return nil, -1, fmt.Errorf("directorio no tiene bloques asignados")
	}

	blockNum := inode.IBlock[0]
	blockPos := startByte + int64(sb.SBlockStart) + int64(blockNum)*int64(sb.SBlockSize)

	_, err := file.Seek(blockPos, 0)
	if err != nil {
		return nil, -1, err
	}

	dirBlock := &DirectoryBlock{}
	err = binary.Read(file, binary.LittleEndian, dirBlock)
	if err != nil {
		return nil, -1, err
	}

	return dirBlock, blockNum, nil
}

// compareInodes compara dos inodos para determinar si son iguales
func compareInodes(a, b *Inode) bool {
	if a == nil || b == nil {
		return false
	}

	// Comparar los punteros a bloques (lo más importante)
	for i := 0; i < 15; i++ {
		if a.IBlock[i] != b.IBlock[i] {
			return false
		}
	}

	// No comparamos fechas porque pueden cambiar legítimamente
	return a.IType == b.IType &&
		a.IUid == b.IUid &&
		a.IGid == b.IGid &&
		a.ISize == b.ISize
}

// findInodeByPath implementación optimizada para encontrar un inodo por su ruta
func FindInodeByPath(file *os.File, startByte int64, superblock *SuperBlock, path string) (int, *Inode, error) {
	fmt.Printf("Buscando inodo para ruta: %s\n", path)

	if path == "" || path == "/" {
		// Caso especial: directorio raíz (inodo 2)
		inodePos := startByte + int64(superblock.SInodeStart) + 2*int64(superblock.SInodeSize)
		_, err := file.Seek(inodePos, 0)
		if err != nil {
			return -1, nil, fmt.Errorf("error al posicionarse en inodo raíz: %v", err)
		}

		inode, err := readInodeFromDisc(file)
		if err != nil {
			return -1, nil, fmt.Errorf("error al leer inodo raíz: %v", err)
		}

		fmt.Printf("Inodo raíz encontrado (2)\n")
		return 2, inode, nil
	}

	// Normalizar la ruta y dividir en componentes
	cleanPath := filepath.Clean(path)
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	components := strings.Split(strings.TrimPrefix(cleanPath, "/"), "/")
	fmt.Printf("Validando ruta: %s con %d componentes\n", cleanPath, len(components))

	// Comenzar la búsqueda desde el directorio raíz (inodo 2)
	currentInodeNum := 2

	// Recorrer cada componente de la ruta
	for i, component := range components {
		if component == "" {
			continue
		}

		fmt.Printf("Verificando componente: '%s'\n", component)

		// Leer el inodo actual
		inodePos := startByte + int64(superblock.SInodeStart) + int64(currentInodeNum)*int64(superblock.SInodeSize)
		_, err := file.Seek(inodePos, 0)
		if err != nil {
			return -1, nil, fmt.Errorf("error al posicionarse en inodo %d: %v", currentInodeNum, err)
		}

		currentInode, err := readInodeFromDisc(file)
		if err != nil {
			return -1, nil, fmt.Errorf("error al leer inodo %d: %v", currentInodeNum, err)
		}

		fmt.Printf("Leyendo inodo desde posición: %d\n", inodePos)
		fmt.Printf("Tipo de inodo leído: %d\n", currentInode.IType)

		// Verificar que sea un directorio (excepto para el último componente)
		if i < len(components)-1 && currentInode.IType != INODE_FOLDER {
			return -1, nil, fmt.Errorf("el componente '%s' no es un directorio", component)
		}

		// Buscar la entrada en el directorio
		found := false
		nextInodeNum := -1

		// Recorrer los bloques directos del inodo actual
		for j := 0; j < 12; j++ {
			if currentInode.IBlock[j] <= 0 {
				continue
			}

			// Leer el bloque de directorio
			blockPos := startByte + int64(superblock.SBlockStart) + int64(currentInode.IBlock[j])*int64(superblock.SBlockSize)
			_, err := file.Seek(blockPos, 0)
			if err != nil {
				continue
			}

			dirBlock := &DirectoryBlock{}
			err = binary.Read(file, binary.LittleEndian, dirBlock)
			if err != nil {
				continue
			}

			// Buscar la entrada que coincide con el componente actual
			for k := 0; k < B_CONTENT_COUNT; k++ {
				if dirBlock.BContent[k].BInodo <= 0 {
					continue
				}

				// Extraer el nombre eliminando caracteres nulos
				entryName := ""
				for l := 0; l < len(dirBlock.BContent[k].BName); l++ {
					if dirBlock.BContent[k].BName[l] == 0 {
						break
					}
					entryName += string(dirBlock.BContent[k].BName[l])
				}

				fmt.Printf("Entrada encontrada: '%s' -> inodo %d\n", entryName, dirBlock.BContent[k].BInodo)

				if entryName == component {
					nextInodeNum = int(dirBlock.BContent[k].BInodo)
					found = true
					break
				}
			}

			if found {
				break
			}
		}

		if !found {
			return -1, nil, fmt.Errorf("no se encontró el componente '%s' en el directorio", component)
		}

		currentInodeNum = nextInodeNum
	}

	// Leer el inodo final
	inodePos := startByte + int64(superblock.SInodeStart) + int64(currentInodeNum)*int64(superblock.SInodeSize)
	_, err := file.Seek(inodePos, 0)
	if err != nil {
		return -1, nil, fmt.Errorf("error al posicionarse en inodo final %d: %v", currentInodeNum, err)
	}

	finalInode, err := readInodeFromDisc(file)
	if err != nil {
		return -1, nil, fmt.Errorf("error al leer inodo final %d: %v", currentInodeNum, err)
	}

	return currentInodeNum, finalInode, nil
}
