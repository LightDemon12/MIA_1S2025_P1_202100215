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
func CreateEXT2Directory(id, path string) (bool, string) {
	// 1. Verificar la partición montada
	mountedPartition, err := findMountedPartitionById(id)
	if err != nil {
		return false, fmt.Sprintf("Error: %s", err)
	}

	// 2. Abrir el disco
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return false, fmt.Sprintf("Error al abrir el disco: %s", err)
	}
	defer file.Close()

	// 3. Obtener detalles de la partición y leer el superbloque
	startByte, _, err := getPartitionDetails(file, mountedPartition)
	if err != nil {
		return false, fmt.Sprintf("Error al obtener detalles de la partición: %s", err)
	}

	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para leer superbloque: %s", err)
	}

	superblock, err := readSuperBlockFromDisc(file)
	if err != nil {
		return false, fmt.Sprintf("Error al leer el superbloque: %s", err)
	}

	// 4. Leer los bitmaps
	// Bitmap de inodos
	bmInodePos := startByte + int64(superblock.SBmInodeStart)
	_, err = file.Seek(bmInodePos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en bitmap de inodos: %s", err)
	}

	bmInodes := make([]byte, superblock.SInodesCount/8+1)
	_, err = file.Read(bmInodes)
	if err != nil {
		return false, fmt.Sprintf("Error al leer bitmap de inodos: %s", err)
	}

	// Bitmap de bloques
	bmBlockPos := startByte + int64(superblock.SBmBlockStart)
	_, err = file.Seek(bmBlockPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en bitmap de bloques: %s", err)
	}

	bmBlocks := make([]byte, superblock.SBlocksCount/8+1)
	_, err = file.Read(bmBlocks)
	if err != nil {
		return false, fmt.Sprintf("Error al leer bitmap de bloques: %s", err)
	}

	// 5. Parsear la ruta para obtener directorio padre y nombre
	dirPath := filepath.Dir(path)
	dirName := filepath.Base(path)

	// No permitir nombres muy largos (máximo 12 caracteres para ser compatible con BName)
	if len(dirName) > 12 {
		return false, "Error: El nombre del directorio no puede exceder 12 caracteres"
	}

	// 6. Encontrar el directorio padre
	parentInodeNum, parentInode, err := findInodeByPath(file, startByte, superblock, dirPath)
	if err != nil {
		return false, fmt.Sprintf("Error al buscar directorio padre: %s", err)
	}

	// 7. Verificar que el directorio padre sea efectivamente un directorio
	if parentInode.IType != INODE_FOLDER {
		return false, fmt.Sprintf("Error: %s no es un directorio", dirPath)
	}

	// 8. Verificar que el directorio no exista ya
	_, _, err = findInodeByPath(file, startByte, superblock, path)
	if err == nil {
		return false, fmt.Sprintf("Error: Ya existe un archivo o directorio en la ruta %s", path)
	}

	// 9. Encontrar un inodo libre
	freeInodeNum := -1
	for i := EXT2_RESERVED_INODES; i < int(superblock.SInodesCount); i++ {
		bytePos := i / 8
		bitPos := i % 8
		if bytePos >= len(bmInodes) {
			continue
		}
		if (bmInodes[bytePos] & (1 << bitPos)) == 0 {
			freeInodeNum = i
			break
		}
	}

	if freeInodeNum == -1 {
		return false, "Error: No hay inodos libres disponibles"
	}

	// 10. Encontrar un bloque libre
	freeBlockNum := -1
	for i := 0; i < int(superblock.SBlocksCount); i++ {
		bytePos := i / 8
		bitPos := i % 8
		if bytePos >= len(bmBlocks) {
			continue
		}
		if (bmBlocks[bytePos] & (1 << bitPos)) == 0 {
			freeBlockNum = i
			break
		}
	}

	if freeBlockNum == -1 {
		return false, "Error: No hay bloques libres disponibles"
	}

	// 11. Crear el nuevo inodo para el directorio
	dirInode := NewInode(0, 0, INODE_FOLDER)
	dirInode.IPerm[0] = 7 // rwx para propietario
	dirInode.IPerm[1] = 5 // r-x para grupo
	dirInode.IPerm[2] = 5 // r-x para otros
	dirInode.ISize = 64   // Tamaño estándar de directorio

	// Inicializar bloques con -1
	for i := 0; i < 15; i++ {
		dirInode.IBlock[i] = -1
	}

	// Asignar el bloque al inodo
	dirInode.IBlock[0] = int32(freeBlockNum)

	// 12. Crear el bloque de directorio
	dirBlock := &DirectoryBlock{}
	for i := 0; i < B_CONTENT_COUNT; i++ {
		dirBlock.BContent[i].BInodo = -1 // Inicializar con -1
	}

	// Añadir entradas "." y ".."
	copy(dirBlock.BContent[0].BName[:], []byte("."))
	dirBlock.BContent[0].BInodo = int32(freeInodeNum)

	copy(dirBlock.BContent[1].BName[:], []byte(".."))
	dirBlock.BContent[1].BInodo = int32(parentInodeNum)

	// 13. Leer el bloque del directorio padre
	parentBlockNum := parentInode.IBlock[0] // Asumimos que usa el primer bloque directo
	if parentBlockNum == -1 {
		return false, "Error: El directorio padre no tiene un bloque asignado"
	}

	parentBlockPos := startByte + int64(superblock.SBlockStart) + int64(parentBlockNum)*int64(superblock.SBlockSize)
	_, err = file.Seek(parentBlockPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en el bloque del directorio padre: %s", err)
	}

	parentDirBlock := &DirectoryBlock{}
	err = binary.Read(file, binary.LittleEndian, parentDirBlock)
	if err != nil {
		return false, fmt.Sprintf("Error al leer el bloque del directorio padre: %s", err)
	}

	// 14. Buscar una entrada libre en el directorio padre
	entryIdx := -1
	for i := 0; i < B_CONTENT_COUNT; i++ {
		if parentDirBlock.BContent[i].BInodo <= 0 {
			entryIdx = i
			break
		}
	}

	if entryIdx == -1 {
		return false, "Error: No hay espacio en el directorio padre para una nueva entrada"
	}

	// 15. Añadir entrada en el directorio padre
	for j := range parentDirBlock.BContent[entryIdx].BName {
		parentDirBlock.BContent[entryIdx].BName[j] = 0
	}
	copy(parentDirBlock.BContent[entryIdx].BName[:], []byte(dirName))
	parentDirBlock.BContent[entryIdx].BInodo = int32(freeInodeNum)

	// 16. Actualizar bitmaps
	// Marcar inodo como usado
	bmInodes[freeInodeNum/8] |= (1 << (freeInodeNum % 8))

	// Marcar bloque como usado
	bmBlocks[freeBlockNum/8] |= (1 << (freeBlockNum % 8))

	// 17. Actualizar superbloque
	superblock.SFreeInodesCount--
	superblock.SFreeBlocksCount--
	superblock.SMtime = time.Now()

	// 18. Escribir todo al disco
	// Escribir superbloque
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para escribir superbloque: %s", err)
	}
	err = writeSuperBlockToDisc(file, superblock)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir superbloque: %s", err)
	}

	// Escribir bitmap de inodos
	_, err = file.Seek(bmInodePos, 0)
	if err != nil {
		return false, "Error al posicionarse para escribir bitmap de inodos"
	}
	_, err = file.Write(bmInodes)
	if err != nil {
		return false, "Error al escribir bitmap de inodos"
	}

	// Escribir bitmap de bloques
	_, err = file.Seek(bmBlockPos, 0)
	if err != nil {
		return false, "Error al posicionarse para escribir bitmap de bloques"
	}
	_, err = file.Write(bmBlocks)
	if err != nil {
		return false, "Error al escribir bitmap de bloques"
	}

	// Escribir el nuevo inodo
	inodePos := startByte + int64(superblock.SInodeStart) + int64(freeInodeNum)*int64(superblock.SInodeSize)
	_, err = file.Seek(inodePos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para escribir inodo: %s", err)
	}
	err = writeInodeToDisc(file, dirInode)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir inodo: %s", err)
	}

	// Escribir el nuevo bloque de directorio
	blockPos := startByte + int64(superblock.SBlockStart) + int64(freeBlockNum)*int64(superblock.SBlockSize)
	_, err = file.Seek(blockPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para escribir bloque: %s", err)
	}
	err = writeDirectoryBlockToDisc(file, dirBlock)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir bloque de directorio: %s", err)
	}

	// Escribir el directorio padre actualizado
	_, err = file.Seek(parentBlockPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para actualizar directorio padre: %s", err)
	}
	err = writeDirectoryBlockToDisc(file, parentDirBlock)
	if err != nil {
		return false, fmt.Sprintf("Error al actualizar directorio padre: %s", err)
	}

	return true, fmt.Sprintf("Directorio %s creado exitosamente (inodo %d, bloque %d)",
		path, freeInodeNum, freeBlockNum)
}

// CreateEXT2File crea un archivo con contenido en el sistema de archivos EXT2
// CreateEXT2File crea un archivo con contenido en el sistema de archivos EXT2
func CreateEXT2File(id, path, content string) (bool, string) {
	fmt.Printf("Creando archivo EXT2: %s con contenido: %s\n", path, content)

	// 1. Verificar la partición montada
	mountedPartition, err := findMountedPartitionById(id)
	if err != nil {
		return false, fmt.Sprintf("Error: %s", err)
	}

	// 2. Abrir el disco
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return false, fmt.Sprintf("Error al abrir el disco: %s", err)
	}
	defer file.Close()

	// 3. Obtener detalles de la partición y leer el superbloque
	startByte, _, err := getPartitionDetails(file, mountedPartition)
	if err != nil {
		return false, fmt.Sprintf("Error al obtener detalles de la partición: %s", err)
	}

	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para leer superbloque: %s", err)
	}

	superblock, err := readSuperBlockFromDisc(file)
	if err != nil {
		return false, fmt.Sprintf("Error al leer el superbloque: %s", err)
	}

	// 4. Leer los bitmaps
	// Bitmap de inodos
	bmInodePos := startByte + int64(superblock.SBmInodeStart)
	_, err = file.Seek(bmInodePos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en bitmap de inodos: %s", err)
	}

	bmInodes := make([]byte, superblock.SInodesCount/8+1)
	_, err = file.Read(bmInodes)
	if err != nil {
		return false, fmt.Sprintf("Error al leer bitmap de inodos: %s", err)
	}

	// Bitmap de bloques
	bmBlockPos := startByte + int64(superblock.SBmBlockStart)
	_, err = file.Seek(bmBlockPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en bitmap de bloques: %s", err)
	}

	bmBlocks := make([]byte, superblock.SBlocksCount/8+1)
	_, err = file.Read(bmBlocks)
	if err != nil {
		return false, fmt.Sprintf("Error al leer bitmap de bloques: %s", err)
	}

	// 5. Parsear la ruta para obtener directorio padre y nombre
	dirPath := filepath.Dir(path)
	if dirPath == "." {
		dirPath = "/"
	}
	fileName := filepath.Base(path)

	// No permitir nombres muy largos (máximo 12 caracteres para ser compatible con BName)
	if len(fileName) > 12 {
		return false, "Error: El nombre del archivo no puede exceder 12 caracteres"
	}

	// 6. Encontrar el directorio padre
	_, parentInode, err := findInodeByPath(file, startByte, superblock, dirPath)
	if err != nil {
		return false, fmt.Sprintf("Error al buscar directorio padre: %s", err)
	}

	// 7. Verificar que el directorio padre sea efectivamente un directorio
	if parentInode.IType != INODE_FOLDER {
		return false, fmt.Sprintf("Error: %s no es un directorio", dirPath)
	}

	// 8. Verificar que el archivo no exista ya
	existingInodeNum, _, err := findInodeByPath(file, startByte, superblock, path)
	if err == nil && existingInodeNum > 0 {
		return false, fmt.Sprintf("Error: Ya existe un archivo o directorio en la ruta %s", path)
	}

	// 9. Encontrar un inodo libre
	freeInodeNum := -1
	for i := EXT2_RESERVED_INODES; i < int(superblock.SInodesCount); i++ {
		bytePos := i / 8
		bitPos := i % 8
		if bytePos >= len(bmInodes) {
			continue
		}
		if (bmInodes[bytePos] & (1 << bitPos)) == 0 {
			freeInodeNum = i
			break
		}
	}

	if freeInodeNum == -1 {
		return false, "Error: No hay inodos libres disponibles"
	}

	// 10. Encontrar un bloque libre
	freeBlockNum := -1
	for i := 0; i < int(superblock.SBlocksCount); i++ {
		bytePos := i / 8
		bitPos := i % 8
		if bytePos >= len(bmBlocks) {
			continue
		}
		if (bmBlocks[bytePos] & (1 << bitPos)) == 0 {
			freeBlockNum = i
			break
		}
	}

	if freeBlockNum == -1 {
		return false, "Error: No hay bloques libres disponibles"
	}

	// 11. Leer el bloque del directorio padre
	parentBlockNum := int(parentInode.IBlock[0]) // Asumimos que usa el primer bloque directo
	if parentBlockNum < 0 {
		return false, "Error: El directorio padre no tiene un bloque asignado"
	}

	parentBlockPos := startByte + int64(superblock.SBlockStart) + int64(parentBlockNum)*int64(superblock.SBlockSize)
	_, err = file.Seek(parentBlockPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en el bloque del directorio padre: %s", err)
	}

	parentDirBlock := &DirectoryBlock{}
	err = binary.Read(file, binary.LittleEndian, parentDirBlock)
	if err != nil {
		return false, fmt.Sprintf("Error al leer el bloque del directorio padre: %s", err)
	}

	// 12. Buscar una entrada libre en el directorio padre
	entryIdx := -1
	for i := 0; i < B_CONTENT_COUNT; i++ {
		if parentDirBlock.BContent[i].BInodo <= 0 {
			entryIdx = i
			break
		}
	}

	if entryIdx == -1 {
		return false, "Error: No hay espacio en el directorio padre para una nueva entrada"
	}

	// 13. ENFOQUE RADICALMENTE DIFERENTE: Escribir el contenido al bloque
	blockPos := startByte + int64(superblock.SBlockStart) + int64(freeBlockNum)*int64(superblock.SBlockSize)

	// Información de depuración
	fmt.Printf("Escribiendo contenido al bloque %d (pos %d)\n", freeBlockNum, blockPos)
	fmt.Printf("  - Contenido: '%s'\n", content)
	fmt.Printf("  - Longitud: %d bytes\n", len(content))

	// Preparar el buffer del tamaño exacto del bloque
	blockBuffer := make([]byte, superblock.SBlockSize)

	// En lugar de poner ceros o bytes de relleno, vamos a llenar TODO el buffer con el contenido
	// tantas veces como sea necesario para que el patrón sea reconocible
	contentBytes := []byte(content)

	// Copiamos el contenido real
	copy(blockBuffer, contentBytes)

	// Si el contenido es más corto que el bloque, repetimos el contenido
	if len(contentBytes) < int(superblock.SBlockSize) {
		for i := len(contentBytes); i < int(superblock.SBlockSize); i++ {
			// Si el bloque es más grande que el contenido, llenamos con un patrón reconocible
			// pero NO con ceros o FF FF FF FF que parecen causar problemas
			blockBuffer[i] = byte('A' + (i % 26))
		}
	}

	// Posicionarnos exactamente al inicio del bloque
	_, err = file.Seek(blockPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para escribir: %s", err)
	}

	// Escribir el buffer completo
	bytesWritten, err := file.Write(blockBuffer)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir contenido: %s", err)
	}

	if bytesWritten != int(superblock.SBlockSize) {
		return false, fmt.Sprintf("Error: Se escribieron %d bytes, pero el tamaño del bloque es %d bytes",
			bytesWritten, superblock.SBlockSize)
	}

	fmt.Printf("  - Escrito exitoso: %d bytes\n", bytesWritten)
	fmt.Printf("  - Primeros 16 bytes escritos (hex): ")
	for i := 0; i < 16 && i < len(blockBuffer); i++ {
		fmt.Printf("%02X ", blockBuffer[i])
	}
	fmt.Println()

	// 14. Crear el nuevo inodo para el archivo
	fileInode := NewInode(0, 0, INODE_FILE)
	fileInode.IPerm[0] = 6 // rw- para propietario
	fileInode.IPerm[1] = 4 // r-- para grupo
	fileInode.IPerm[2] = 4 // r-- para otros
	fileInode.ISize = int32(len(content))

	// Inicializar bloques con -1
	for i := 0; i < 15; i++ {
		fileInode.IBlock[i] = -1
	}

	// Asignar el bloque al inodo
	fileInode.IBlock[0] = int32(freeBlockNum)

	// 15. Añadir entrada en el directorio padre
	// Limpiar el nombre primero
	for j := range parentDirBlock.BContent[entryIdx].BName {
		parentDirBlock.BContent[entryIdx].BName[j] = 0
	}
	// Copiar el nuevo nombre
	copy(parentDirBlock.BContent[entryIdx].BName[:], []byte(fileName))
	parentDirBlock.BContent[entryIdx].BInodo = int32(freeInodeNum)

	// 16. Actualizar bitmaps
	// Marcar inodo como usado
	bmInodes[freeInodeNum/8] |= (1 << (freeInodeNum % 8))

	// Marcar bloque como usado
	bmBlocks[freeBlockNum/8] |= (1 << (freeBlockNum % 8))

	// 17. Actualizar superbloque
	superblock.SFreeInodesCount--
	superblock.SFreeBlocksCount--
	superblock.SMtime = time.Now()

	// 18. Escribir todo al disco
	// Escribir superbloque
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para escribir superbloque: %s", err)
	}
	err = writeSuperBlockToDisc(file, superblock)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir superbloque: %s", err)
	}

	// Escribir bitmap de inodos
	_, err = file.Seek(bmInodePos, 0)
	if err != nil {
		return false, "Error al posicionarse para escribir bitmap de inodos"
	}
	_, err = file.Write(bmInodes)
	if err != nil {
		return false, "Error al escribir bitmap de inodos"
	}

	// Escribir bitmap de bloques
	_, err = file.Seek(bmBlockPos, 0)
	if err != nil {
		return false, "Error al posicionarse para escribir bitmap de bloques"
	}
	_, err = file.Write(bmBlocks)
	if err != nil {
		return false, "Error al escribir bitmap de bloques"
	}

	// Escribir el nuevo inodo
	inodePos := startByte + int64(superblock.SInodeStart) + int64(freeInodeNum)*int64(superblock.SInodeSize)
	_, err = file.Seek(inodePos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para escribir inodo: %s", err)
	}
	err = writeInodeToDisc(file, fileInode)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir inodo: %s", err)
	}

	// Verificar que se haya escrito correctamente el inodo
	_, err = file.Seek(inodePos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para verificar inodo: %s", err)
	}
	verificacionInode, err := readInodeFromDisc(file)
	if err != nil {
		return false, fmt.Sprintf("Error al verificar inodo: %s", err)
	}
	fmt.Printf("Inodo verificado - Tamaño: %d, Tipo: %d, Block[0]: %d\n",
		verificacionInode.ISize, verificacionInode.IType, verificacionInode.IBlock[0])

	// Escribir el directorio padre actualizado
	_, err = file.Seek(parentBlockPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para actualizar directorio padre: %s", err)
	}
	err = writeDirectoryBlockToDisc(file, parentDirBlock)
	if err != nil {
		return false, fmt.Sprintf("Error al actualizar directorio padre: %s", err)
	}

	// PASO ADICIONAL: Verificar que se haya escrito correctamente el contenido
	_, err = file.Seek(blockPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para verificar contenido: %s", err)
	}

	verifyBuffer := make([]byte, superblock.SBlockSize)
	bytesRead, err := file.Read(verifyBuffer)
	if err != nil {
		return false, fmt.Sprintf("Error al leer contenido para verificación: %s", err)
	}

	if bytesRead != int(superblock.SBlockSize) {
		return false, fmt.Sprintf("Error de verificación: Se leyeron %d bytes, pero el tamaño del bloque es %d bytes",
			bytesRead, superblock.SBlockSize)
	}

	// Verificar solo la parte del contenido
	verifyContent := string(verifyBuffer[:len(content)])
	if verifyContent != content {
		fmt.Printf("ADVERTENCIA: El contenido verificado difiere del original:\n")
		fmt.Printf("  - Original (primeros 32 bytes): '%s'\n", truncateString(content, 32))
		fmt.Printf("  - Verificado (primeros 32 bytes): '%s'\n", truncateString(verifyContent, 32))
		fmt.Printf("  - Bytes verificados (hex): ")
		for i := 0; i < min(16, bytesRead); i++ {
			fmt.Printf("%02X ", verifyBuffer[i])
		}
		fmt.Println()
	} else {
		fmt.Printf("Verificación exitosa: Contenido escrito correctamente\n")
	}

	return true, fmt.Sprintf("Archivo %s creado exitosamente (inodo %d, bloque %d, contenido: %d bytes)",
		path, freeInodeNum, freeBlockNum, len(content))
}

// Función auxiliar para truncar strings
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
}

// findInodeByPath busca un inodo por su ruta
func findInodeByPath(file *os.File, startByte int64, superblock *SuperBlock, path string) (int, *Inode, error) {
	if path == "" || path == "/" {
		// Directorio raíz (inodo 2)
		inodePos := startByte + int64(superblock.SInodeStart) + 2*int64(superblock.SInodeSize)
		_, err := file.Seek(inodePos, 0)
		if err != nil {
			return -1, nil, err
		}

		inode, err := readInodeFromDisc(file)
		if err != nil {
			return -1, nil, err
		}

		return 2, inode, nil
	}

	// Separar la ruta en componentes
	parts := strings.Split(strings.Trim(path, "/"), "/")
	currentInodeNum := 2 // Empezar desde el directorio raíz

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Leer el inodo actual
		inodePos := startByte + int64(superblock.SInodeStart) + int64(currentInodeNum)*int64(superblock.SInodeSize)
		_, err := file.Seek(inodePos, 0)
		if err != nil {
			return -1, nil, err
		}

		currentInode, err := readInodeFromDisc(file)
		if err != nil {
			return -1, nil, fmt.Errorf("no se pudo leer el inodo %d: %v", currentInodeNum, err)
		}

		// Verificar que sea un directorio
		if currentInode.IType != INODE_FOLDER {
			return -1, nil, fmt.Errorf("el inodo %d no es un directorio", currentInodeNum)
		}

		// Buscar el hijo correspondiente
		found := false
		for i := 0; i < 12; i++ { // Solo bloques directos por ahora
			if currentInode.IBlock[i] <= 0 {
				continue
			}

			// Leer el bloque de directorio
			blockPos := startByte + int64(superblock.SBlockStart) + int64(currentInode.IBlock[i])*int64(superblock.SBlockSize)
			_, err := file.Seek(blockPos, 0)
			if err != nil {
				continue
			}

			dirBlock := &DirectoryBlock{}
			err = binary.Read(file, binary.LittleEndian, dirBlock)
			if err != nil {
				continue
			}

			// Buscar la entrada en el directorio
			for j := 0; j < B_CONTENT_COUNT; j++ {
				if dirBlock.BContent[j].BInodo <= 0 {
					continue
				}

				entryName := strings.TrimRight(string(dirBlock.BContent[j].BName[:]), "\x00")
				if entryName == part {
					currentInodeNum = int(dirBlock.BContent[j].BInodo)
					found = true
					break
				}
			}

			if found {
				break
			}
		}

		if !found {
			return -1, nil, fmt.Errorf("no se encontró la entrada %s en la ruta", part)
		}
	}

	// Leer el inodo final
	inodePos := startByte + int64(superblock.SInodeStart) + int64(currentInodeNum)*int64(superblock.SInodeSize)
	_, err := file.Seek(inodePos, 0)
	if err != nil {
		return -1, nil, err
	}

	inode, err := readInodeFromDisc(file)
	if err != nil {
		return -1, nil, err
	}

	return currentInodeNum, inode, nil
}

func GetUserIdFromName(partitionID, username string) int32 {
	return getUserIdFromName(partitionID, username)
}

// GetGroupIdFromName versión exportable para obtener el ID de grupo
func GetGroupIdFromName(partitionID, groupname string) int32 {
	return getGroupIdFromName(partitionID, groupname)
}

func OverwriteEXT2File(id, path, content string) (bool, string) {
	fmt.Printf("Sobrescribiendo archivo EXT2: %s\n", path)

	// 1. Verificar la partición montada
	mountedPartition, err := findMountedPartitionById(id)
	if err != nil {
		return false, fmt.Sprintf("Error: %s", err)
	}

	// 2. Abrir el disco
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return false, fmt.Sprintf("Error al abrir el disco: %s", err)
	}
	defer file.Close()

	// 3. Obtener detalles de la partición y leer el superbloque
	startByte, _, err := getPartitionDetails(file, mountedPartition)
	if err != nil {
		return false, fmt.Sprintf("Error al obtener detalles de la partición: %s", err)
	}

	superblock, err := readSuperBlockFromDisc(file)
	if err != nil {
		return false, fmt.Sprintf("Error al leer el superbloque: %s", err)
	}

	// 4. Encontrar el inodo del archivo a sobrescribir
	inodeNum, inode, err := findInodeByPath(file, startByte, superblock, path)
	if err != nil {
		return false, fmt.Sprintf("Error al buscar archivo: %s", err)
	}

	// 5. Verificar que sea un archivo y no un directorio
	if inode.IType != INODE_FILE {
		return false, "Error: La ruta especificada no es un archivo"
	}

	// 6. Preparar el buffer con el nuevo contenido
	blockBuffer := make([]byte, superblock.SBlockSize)
	contentBytes := []byte(content)

	// Copiar el contenido real
	copy(blockBuffer, contentBytes)

	// Rellenar el resto del bloque si es necesario
	if len(contentBytes) < int(superblock.SBlockSize) {
		for i := len(contentBytes); i < int(superblock.SBlockSize); i++ {
			blockBuffer[i] = byte('A' + (i % 26))
		}
	}

	// 7. Sobrescribir el bloque existente
	blockNum := inode.IBlock[0]
	if blockNum < 0 {
		return false, "Error: El archivo no tiene un bloque asignado"
	}

	blockPos := startByte + int64(superblock.SBlockStart) + int64(blockNum)*int64(superblock.SBlockSize)
	_, err = file.Seek(blockPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para escribir: %s", err)
	}

	bytesWritten, err := file.Write(blockBuffer)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir contenido: %s", err)
	}

	if bytesWritten != int(superblock.SBlockSize) {
		return false, fmt.Sprintf("Error: Se escribieron %d bytes, pero el tamaño del bloque es %d",
			bytesWritten, superblock.SBlockSize)
	}

	// 8. Actualizar el tamaño en el inodo
	inode.ISize = int32(len(content))

	// 9. Escribir el inodo actualizado
	inodePos := startByte + int64(superblock.SInodeStart) + int64(inodeNum)*int64(superblock.SInodeSize)
	_, err = file.Seek(inodePos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para actualizar inodo: %s", err)
	}

	err = writeInodeToDisc(file, inode)
	if err != nil {
		return false, fmt.Sprintf("Error al actualizar inodo: %s", err)
	}

	// 10. Actualizar el superbloque (tiempo de modificación)
	superblock.SMtime = time.Now()
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para actualizar superbloque: %s", err)
	}

	err = writeSuperBlockToDisc(file, superblock)
	if err != nil {
		return false, fmt.Sprintf("Error al actualizar superbloque: %s", err)
	}

	return true, fmt.Sprintf("Archivo %s sobrescrito exitosamente con %d bytes", path, len(content))
}
