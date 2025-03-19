package DiskManager

import (
	"fmt"
	"os"
	"time"
)

// Constantes para modos de operación de archivo
const (
	FILE_READ   = iota // Leer contenido
	FILE_WRITE         // Sobrescribir contenido
	FILE_APPEND        // Añadir contenido al final
)

// EXT2FileOperation realiza operaciones de lectura/escritura/anexo en archivos EXT2
func EXT2FileOperation(id string, path string, operation int, content string) (string, error) {
	// 1. Validar que la ruta existe y es un archivo
	exists, pathType, err := ValidateEXT2Path(id, path)
	if err != nil {
		return "", err
	}

	if !exists {
		return "", fmt.Errorf("El archivo no existe: %s", path)
	}

	if pathType != "archivo" {
		return "", fmt.Errorf("La ruta %s no es un archivo", path)
	}

	// 2. Obtener la partición montada
	mountedPartition, err := findMountedPartitionById(id)
	if err != nil {
		return "", fmt.Errorf("Error: %s", err)
	}

	// 3. Abrir el disco
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return "", fmt.Errorf("Error al abrir el disco: %s", err)
	}
	defer file.Close()

	// 4. Obtener detalles de la partición y superbloque
	startByte, _, err := getPartitionDetails(file, mountedPartition)
	if err != nil {
		return "", fmt.Errorf("Error al obtener detalles de la partición: %s", err)
	}

	_, err = file.Seek(startByte, 0)
	if err != nil {
		return "", fmt.Errorf("Error al posicionarse para leer superbloque: %s", err)
	}

	superblock, err := readSuperBlockFromDisc(file)
	if err != nil {
		return "", fmt.Errorf("Error al leer el superbloque: %s", err)
	}

	// 5. Encontrar el inodo del archivo siguiendo la ruta
	inodeNum, inodeData, err := findInodeByPath(file, startByte, superblock, path)
	if err != nil {
		return "", fmt.Errorf("Error al buscar el inodo del archivo: %s", err)
	}

	// 6. Ejecutar la operación según el modo
	switch operation {
	case FILE_READ:
		// Leer el contenido del archivo
		return readFileContent(file, startByte, superblock, inodeData)

	case FILE_WRITE:
		// Sobrescribir completamente el archivo
		return writeFileContent(file, startByte, superblock, inodeNum, inodeData, content, false)

	case FILE_APPEND:
		// Añadir al final del archivo
		return writeFileContent(file, startByte, superblock, inodeNum, inodeData, content, true)

	default:
		return "", fmt.Errorf("Operación no reconocida: %d", operation)
	}
}

// readFileContent lee todo el contenido de un archivo
func readFileContent(file *os.File, startByte int64, sb *SuperBlock, inode *Inode) (string, error) {
	// Determinar cuánto contenido necesitamos leer
	contentSize := inode.ISize
	if contentSize <= 0 {
		return "", nil // Archivo vacío
	}

	// Buffer para almacenar el contenido
	contentBuffer := make([]byte, contentSize)
	bytesRead := int32(0)

	// Leer los bloques directos primero
	for i := 0; i < 12 && bytesRead < contentSize; i++ {
		blockNum := inode.IBlock[i]
		if blockNum <= 0 {
			break
		}

		// Posicionarse al inicio del bloque
		blockPos := startByte + int64(sb.SBlockStart) + int64(blockNum)*int64(sb.SBlockSize)
		_, err := file.Seek(blockPos, 0)
		if err != nil {
			return "", fmt.Errorf("Error al posicionarse para leer bloque: %s", err)
		}

		// Determinar cuántos bytes leer de este bloque
		bytesToRead := min(int(sb.SBlockSize), int(contentSize-bytesRead))

		// Leer el bloque - CORREGIDO: Convertir a int para los índices de slice
		bufferPosInt := int(bytesRead)
		n, err := file.Read(contentBuffer[bufferPosInt : bufferPosInt+bytesToRead])
		if err != nil {
			return "", fmt.Errorf("Error al leer bloque: %s", err)
		}

		bytesRead += int32(n)
	}

	// Leer el indirecto simple si es necesario
	if bytesRead < contentSize && inode.IBlock[12] > 0 {
		// Implementar lectura desde indirecto simple si lo necesitas
		// Por ahora lo dejamos como extensión
	}

	// Convertir a string
	return string(contentBuffer[:bytesRead]), nil
}

// writeFileContent escribe o añade contenido a un archivo
func writeFileContent(file *os.File, startByte int64, sb *SuperBlock, inodeNum int, inode *Inode,
	newContent string, isAppend bool) (string, error) {

	// Calcular el tamaño total después de la operación
	var totalSize int32
	if isAppend {
		totalSize = inode.ISize + int32(len(newContent))
	} else {
		totalSize = int32(len(newContent))
	}

	// Verificar si necesitamos más bloques
	neededBlocks := (totalSize + sb.SBlockSize - 1) / sb.SBlockSize
	currentBlocks := 0

	// Contar bloques actuales
	for i := 0; i < 12 && inode.IBlock[i] > 0; i++ {
		currentBlocks++
	}

	// Si necesitamos más bloques, asignarlos
	newBlocks := []int32{}
	if neededBlocks > int32(currentBlocks) {
		// Leer el bitmap de bloques
		bmBlockPos := startByte + int64(sb.SBmBlockStart)
		_, err := file.Seek(bmBlockPos, 0)
		if err != nil {
			return "", fmt.Errorf("Error al posicionarse en bitmap de bloques: %s", err)
		}

		bmBlocks := make([]byte, sb.SBlocksCount/8+1)
		_, err = file.Read(bmBlocks)
		if err != nil {
			return "", fmt.Errorf("Error al leer bitmap de bloques: %s", err)
		}

		// Encontrar bloques libres
		blocksNeeded := int(neededBlocks) - currentBlocks
		for i := 0; i < int(sb.SBlocksCount) && len(newBlocks) < blocksNeeded; i++ {
			bytePos := i / 8
			bitPos := i % 8

			if bytePos < len(bmBlocks) && (bmBlocks[bytePos]&(1<<bitPos)) == 0 {
				// Marcar bloque como usado
				bmBlocks[bytePos] |= (1 << bitPos)
				newBlocks = append(newBlocks, int32(i))
			}
		}

		if len(newBlocks) < blocksNeeded {
			return "", fmt.Errorf("No hay suficientes bloques libres para esta operación")
		}

		// Escribir bitmap actualizado
		_, err = file.Seek(bmBlockPos, 0)
		if err == nil {
			file.Write(bmBlocks)
		}

		// Actualizar el superbloque
		sb.SFreeBlocksCount -= int32(len(newBlocks))
		_, err = file.Seek(startByte, 0)
		if err == nil {
			writeSuperBlockToDisc(file, sb)
		}
	}

	// Preparar buffer para escribir
	var contentToWrite []byte
	if isAppend {
		// Leer el contenido actual primero
		currentContent, err := readFileContent(file, startByte, sb, inode)
		if err != nil {
			return "", fmt.Errorf("Error al leer contenido actual: %s", err)
		}

		contentToWrite = []byte(currentContent + newContent)
	} else {
		contentToWrite = []byte(newContent)
	}

	// Asignar los nuevos bloques al inodo si es necesario
	newBlockIndex := 0
	for i := currentBlocks; i < int(neededBlocks); i++ {
		if i < 12 { // Solo usamos bloques directos por ahora
			inode.IBlock[i] = newBlocks[newBlockIndex]
			newBlockIndex++
		}
	}

	// Escribir el contenido a los bloques
	contentPos := 0
	for i := 0; i < int(neededBlocks); i++ {
		if i >= 12 {
			break // No manejamos bloques indirectos por ahora
		}

		blockNum := inode.IBlock[i]
		if blockNum <= 0 {
			break
		}

		// Posicionarse al inicio del bloque
		blockPos := startByte + int64(sb.SBlockStart) + int64(blockNum)*int64(sb.SBlockSize)
		_, err := file.Seek(blockPos, 0)
		if err != nil {
			return "", fmt.Errorf("Error al posicionarse para escribir bloque: %s", err)
		}

		// Determinar cuántos bytes escribir en este bloque
		bytesToWrite := min(int(sb.SBlockSize), len(contentToWrite)-contentPos)
		if bytesToWrite <= 0 {
			break
		}

		// Crear buffer del tamaño exacto del bloque (inicializado a ceros)
		blockBuffer := make([]byte, sb.SBlockSize)

		// Copiar el contenido al inicio del buffer
		copy(blockBuffer, contentToWrite[contentPos:contentPos+bytesToWrite])

		// Escribir el buffer completo
		_, err = file.Write(blockBuffer)
		if err != nil {
			return "", fmt.Errorf("Error al escribir bloque: %s", err)
		}

		contentPos += bytesToWrite
	}

	// Actualizar tamaño y timestamp en el inodo
	inode.ISize = int32(len(contentToWrite))
	inode.IMtime = time.Now()

	// Escribir el inodo actualizado
	inodePos := startByte + int64(sb.SInodeStart) + int64(inodeNum)*int64(sb.SInodeSize)
	_, err := file.Seek(inodePos, 0)
	if err != nil {
		return "", fmt.Errorf("Error al posicionarse para actualizar inodo: %s", err)
	}

	err = writeInodeToDisc(file, inode)
	if err != nil {
		return "", fmt.Errorf("Error al actualizar inodo: %s", err)
	}

	// Mensaje para retornar según la operación
	if isAppend {
		return fmt.Sprintf("Contenido añadido exitosamente. Nuevo tamaño: %d bytes", len(contentToWrite)), nil
	} else {
		return fmt.Sprintf("Archivo sobrescrito exitosamente. Tamaño: %d bytes", len(contentToWrite)), nil
	}
}
