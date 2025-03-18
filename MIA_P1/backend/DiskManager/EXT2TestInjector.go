package DiskManager

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// EXT2AutoInjector - versión que crea múltiples inodos (archivos y directorios)
func EXT2AutoInjector(id string) (bool, string) {
	// Localizar partición montada
	mountedPartition, err := findMountedPartitionById(id)
	if err != nil {
		return false, fmt.Sprintf("Error: %s", err)
	}

	// Abrir el disco
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return false, fmt.Sprintf("Error al abrir el disco: %s", err)
	}
	defer file.Close()

	// Obtener posición de inicio
	startByte, _, err := getPartitionDetails(file, mountedPartition)
	if err != nil {
		return false, fmt.Sprintf("Error al obtener detalles de la partición: %s", err)
	}

	// Leer superbloque
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para leer el superbloque: %s", err)
	}

	superblock, err := readSuperBlockFromDisc(file)
	if err != nil {
		return false, fmt.Sprintf("Error al leer el superbloque: %s", err)
	}

	fmt.Printf("=== Creando múltiples inodos de prueba ===\n")
	fmt.Printf("Tamaño de bloque: %d bytes\n", superblock.SBlockSize)

	// Posiciones importantes
	inodeTablePos := startByte + int64(superblock.SInodeStart)
	bmInodePos := startByte + int64(superblock.SBmInodeStart)
	bmBlockPos := startByte + int64(superblock.SBmBlockStart)

	// Leer bitmaps actuales
	_, err = file.Seek(bmInodePos, 0)
	if err != nil {
		return false, "Error al posicionarse en bitmap de inodos"
	}

	bmInodes := make([]byte, superblock.SInodesCount/8+1)
	_, err = file.Read(bmInodes)
	if err != nil {
		return false, "Error al leer bitmap de inodos"
	}

	_, err = file.Seek(bmBlockPos, 0)
	if err != nil {
		return false, "Error al posicionarse en bitmap de bloques"
	}

	bmBlocks := make([]byte, superblock.SBlocksCount/8+1)
	_, err = file.Read(bmBlocks)
	if err != nil {
		return false, "Error al leer bitmap de bloques"
	}

	// Leer inodo raíz (inodo 2)
	rootInodePos := inodeTablePos + 2*int64(superblock.SInodeSize)
	_, err = file.Seek(rootInodePos, 0)
	if err != nil {
		return false, "Error al posicionarse en inodo raíz"
	}

	rootInode, err := readInodeFromDisc(file)
	if err != nil {
		return false, "Error al leer inodo raíz"
	}

	// Leer bloque de directorio raíz
	rootBlockPos := startByte + int64(superblock.SBlockStart) + int64(rootInode.IBlock[0])*int64(superblock.SBlockSize)
	_, err = file.Seek(rootBlockPos, 0)
	if err != nil {
		return false, "Error al posicionarse en bloque raíz"
	}

	rootDirBlock := &DirectoryBlock{}
	err = binary.Read(file, binary.LittleEndian, rootDirBlock)
	if err != nil {
		return false, "Error al leer bloque raíz"
	}

	// Funciones auxiliares
	// 1. Encontrar inodo libre
	findFreeInode := func() int {
		for i := EXT2_RESERVED_INODES; i < int(superblock.SInodesCount); i++ {
			byteIdx := i / 8
			bitIdx := i % 8
			if (bmInodes[byteIdx] & (1 << bitIdx)) == 0 {
				// Marcar como usado
				bmInodes[byteIdx] |= (1 << bitIdx)
				return i
			}
		}
		return -1
	}

	// 2. Encontrar bloque libre
	findFreeBlock := func() int {
		for i := 0; i < int(superblock.SBlocksCount); i++ {
			byteIdx := i / 8
			bitIdx := i % 8
			if (bmBlocks[byteIdx] & (1 << bitIdx)) == 0 {
				// Marcar como usado
				bmBlocks[byteIdx] |= (1 << bitIdx)
				return i
			}
		}
		return -1
	}

	// 3. Buscar entrada libre en un directorio
	findFreeEntry := func(dirBlock *DirectoryBlock) int {
		for i := 0; i < B_CONTENT_COUNT; i++ {
			if dirBlock.BContent[i].BInodo == -1 {
				return i
			}
		}
		return -1
	}

	// 4. Crear un nuevo directorio
	createDirectory := func(name string, parentDirBlock *DirectoryBlock, parentInodeNum int) (int, int, error) {
		// Buscar un inodo libre
		dirInodeNum := findFreeInode()
		if dirInodeNum == -1 {
			return -1, -1, fmt.Errorf("no hay inodos libres para el directorio")
		}

		// Buscar un bloque libre para el contenido del directorio
		dirBlockNum := findFreeBlock()
		if dirBlockNum == -1 {
			return -1, -1, fmt.Errorf("no hay bloques libres para el directorio")
		}

		// Buscar una entrada libre en el directorio padre
		entryIdx := findFreeEntry(parentDirBlock)
		if entryIdx == -1 {
			return -1, -1, fmt.Errorf("no hay entradas libres en el directorio padre")
		}

		// Crear inodo del directorio
		dirInode := NewInode(0, 0, INODE_FOLDER)
		dirInode.IPerm[0] = 7 // rwx
		dirInode.IPerm[1] = 5 // r-x
		dirInode.IPerm[2] = 5 // r-x

		// Inicializar todos los bloques del inodo a -1
		for i := 0; i < 15; i++ {
			dirInode.IBlock[i] = -1
		}

		// Asignar el bloque al inodo
		dirInode.IBlock[0] = int32(dirBlockNum)
		dirInode.ISize = 64 // Tamaño estándar para directorio (2 entradas mínimo)

		// Timestamp actual

		// Crear el bloque de directorio
		dirBlock := &DirectoryBlock{}
		for i := 0; i < B_CONTENT_COUNT; i++ {
			dirBlock.BContent[i].BInodo = -1
		}

		// Añadir entradas "." y ".."
		copy(dirBlock.BContent[0].BName[:], []byte("."))
		dirBlock.BContent[0].BInodo = int32(dirInodeNum)

		copy(dirBlock.BContent[1].BName[:], []byte(".."))
		dirBlock.BContent[1].BInodo = int32(parentInodeNum)

		// Escribir el inodo del directorio
		dirInodePos := inodeTablePos + int64(dirInodeNum)*int64(superblock.SInodeSize)
		_, err = file.Seek(dirInodePos, 0)
		if err != nil {
			return -1, -1, err
		}

		err = writeInodeToDisc(file, dirInode)
		if err != nil {
			return -1, -1, err
		}

		// Escribir el bloque del directorio
		dirBlockPos := startByte + int64(superblock.SBlockStart) + int64(dirBlockNum)*int64(superblock.SBlockSize)
		_, err = file.Seek(dirBlockPos, 0)
		if err != nil {
			return -1, -1, err
		}

		err = writeDirectoryBlockToDisc(file, dirBlock)
		if err != nil {
			return -1, -1, err
		}

		// Añadir entrada en el directorio padre
		copy(parentDirBlock.BContent[entryIdx].BName[:], []byte(name))
		parentDirBlock.BContent[entryIdx].BInodo = int32(dirInodeNum)

		return dirInodeNum, dirBlockNum, nil
	}

	// 5. Crear archivo con tamaño específico
	createFile := func(name string, parentDirBlock *DirectoryBlock, size int) (int, []int, error) {
		// Calculamos cuántos bloques necesitamos
		blockSize := int(superblock.SBlockSize)
		blocksNeeded := (size + blockSize - 1) / blockSize // Redondeo hacia arriba

		// Verificar si es demasiado grande
		if blocksNeeded > 12+256 { // 12 bloques directos + 256 indirectos
			return -1, nil, fmt.Errorf("tamaño de archivo demasiado grande")
		}

		// Buscar inodo libre
		fileInodeNum := findFreeInode()
		if fileInodeNum == -1 {
			return -1, nil, fmt.Errorf("no hay inodos libres para el archivo")
		}

		// Buscar entrada libre en directorio padre
		entryIdx := findFreeEntry(parentDirBlock)
		if entryIdx == -1 {
			return -1, nil, fmt.Errorf("no hay entradas libres en el directorio padre")
		}

		// Crear inodo para el archivo
		fileInode := NewInode(0, 0, INODE_FILE)
		fileInode.IPerm[0] = 6 // rw-
		fileInode.IPerm[1] = 4 // r--
		fileInode.IPerm[2] = 4 // r--
		fileInode.ISize = int32(size)

		// Inicializar todos los bloques a -1
		for i := 0; i < 15; i++ {
			fileInode.IBlock[i] = -1
		}

		// Asignar bloques
		usedBlocks := []int{}

		// 1. Asignar bloques directos
		directBlocks := blocksNeeded
		if directBlocks > 12 {
			directBlocks = 12 // Máximo 12 bloques directos
		}

		for i := 0; i < directBlocks; i++ {
			blockNum := findFreeBlock()
			if blockNum == -1 {
				return -1, usedBlocks, fmt.Errorf("no hay suficientes bloques libres")
			}

			fileInode.IBlock[i] = int32(blockNum)
			usedBlocks = append(usedBlocks, blockNum)

			// Escribir datos de prueba en el bloque
			blockPos := startByte + int64(superblock.SBlockStart) + int64(blockNum)*int64(blockSize)
			_, err = file.Seek(blockPos, 0)
			if err != nil {
				return -1, usedBlocks, err
			}

			// Contenido de prueba (personalizado por posición)
			blockData := make([]byte, blockSize)
			for j := 0; j < blockSize; j++ {
				blockData[j] = byte('A' + ((i + j) % 26))
			}

			_, err = file.Write(blockData)
			if err != nil {
				return -1, usedBlocks, err
			}
		}

		// 2. Si necesitamos bloques indirectos
		remainingBlocks := blocksNeeded - directBlocks
		if remainingBlocks > 0 {
			// Crear el bloque de indirección
			indirectBlockNum := findFreeBlock()
			if indirectBlockNum == -1 {
				return -1, usedBlocks, fmt.Errorf("no hay bloques libres para indirección")
			}

			fileInode.IBlock[12] = int32(indirectBlockNum) // Indirecto simple
			usedBlocks = append(usedBlocks, indirectBlockNum)

			// Preparar tabla de punteros
			pointersPerBlock := blockSize / 4
			indirectPointers := make([]int32, pointersPerBlock)
			for i := range indirectPointers {
				indirectPointers[i] = -1
			}

			// Asignar bloques indirectos
			for i := 0; i < remainingBlocks; i++ {
				blockNum := findFreeBlock()
				if blockNum == -1 {
					return -1, usedBlocks, fmt.Errorf("no hay suficientes bloques libres para indirectos")
				}

				indirectPointers[i] = int32(blockNum)
				usedBlocks = append(usedBlocks, blockNum)

				// Escribir contenido de prueba
				blockPos := startByte + int64(superblock.SBlockStart) + int64(blockNum)*int64(blockSize)
				_, err = file.Seek(blockPos, 0)
				if err != nil {
					return -1, usedBlocks, err
				}

				// Para bloques indirectos usamos letras minúsculas
				blockData := make([]byte, blockSize)
				for j := 0; j < blockSize; j++ {
					blockData[j] = byte('a' + ((i + j) % 26))
				}

				_, err = file.Write(blockData)
				if err != nil {
					return -1, usedBlocks, err
				}
			}

			// Escribir la tabla de indirección
			indirectBlockPos := startByte + int64(superblock.SBlockStart) + int64(indirectBlockNum)*int64(blockSize)
			_, err = file.Seek(indirectBlockPos, 0)
			if err != nil {
				return -1, usedBlocks, err
			}

			err = binary.Write(file, binary.LittleEndian, indirectPointers)
			if err != nil {
				return -1, usedBlocks, err
			}
		}

		// Actualizar tiempos del inodo

		// Escribir el inodo
		fileInodePos := inodeTablePos + int64(fileInodeNum)*int64(superblock.SInodeSize)
		_, err = file.Seek(fileInodePos, 0)
		if err != nil {
			return -1, usedBlocks, err
		}

		err = writeInodeToDisc(file, fileInode)
		if err != nil {
			return -1, usedBlocks, err
		}

		// Añadir entrada al directorio padre
		copy(parentDirBlock.BContent[entryIdx].BName[:], []byte(name))
		parentDirBlock.BContent[entryIdx].BInodo = int32(fileInodeNum)

		return fileInodeNum, usedBlocks, nil
	}

	// Crear múltiples inodos
	createdItems := []string{}
	blockSize := int(superblock.SBlockSize)

	// 1. Crear archivos en la raíz
	// Archivo pequeño (3 bloques)
	smallFileInodeNum, smallFileBlocks, err := createFile("small.txt", rootDirBlock, blockSize*3)
	if err == nil {
		createdItems = append(createdItems, fmt.Sprintf("Archivo 'small.txt': inodo %d, %d bytes, %d bloques",
			smallFileInodeNum, blockSize*3, len(smallFileBlocks)))
	}

	// Archivo mediano (8 bloques)
	mediumFileInodeNum, mediumFileBlocks, err := createFile("medium.txt", rootDirBlock, blockSize*8)
	if err == nil {
		createdItems = append(createdItems, fmt.Sprintf("Archivo 'medium.txt': inodo %d, %d bytes, %d bloques",
			mediumFileInodeNum, blockSize*8, len(mediumFileBlocks)))
	}

	// Archivo grande (15 bloques - usa indirectos)
	largeFileInodeNum, largeFileBlocks, err := createFile("large.txt", rootDirBlock, blockSize*15)
	if err == nil {
		createdItems = append(createdItems, fmt.Sprintf("Archivo 'large.txt': inodo %d, %d bytes, %d bloques (indirecto)",
			largeFileInodeNum, blockSize*15, len(largeFileBlocks)))
	}

	// 2. Crear un directorio en la raíz
	docsDirInodeNum, docsDirBlockNum, err := createDirectory("docs", rootDirBlock, 2) // 2 es el inodo raíz
	if err == nil {
		createdItems = append(createdItems, fmt.Sprintf("Directorio 'docs': inodo %d, bloque %d",
			docsDirInodeNum, docsDirBlockNum))

		// Leer el bloque del directorio docs
		docsDirBlockPos := startByte + int64(superblock.SBlockStart) + int64(docsDirBlockNum)*int64(blockSize)
		_, err = file.Seek(docsDirBlockPos, 0)
		if err == nil {
			docsDirBlock := &DirectoryBlock{}
			err = binary.Read(file, binary.LittleEndian, docsDirBlock)
			if err == nil {
				// Crear archivos dentro del directorio docs
				docFileInodeNum, docFileBlocks, err := createFile("readme.txt", docsDirBlock, blockSize*2)
				if err == nil {
					createdItems = append(createdItems, fmt.Sprintf("Archivo 'docs/readme.txt': inodo %d, %d bytes, %d bloques",
						docFileInodeNum, blockSize*2, len(docFileBlocks)))
				}

				// Guardar el bloque del directorio actualizado
				_, err = file.Seek(docsDirBlockPos, 0)
				if err == nil {
					err = writeDirectoryBlockToDisc(file, docsDirBlock)
				}
			}
		}
	}

	// 3. Crear otro directorio
	mediaInodeNum, mediaBlockNum, err := createDirectory("media", rootDirBlock, 2)
	if err == nil {
		createdItems = append(createdItems, fmt.Sprintf("Directorio 'media': inodo %d, bloque %d",
			mediaInodeNum, mediaBlockNum))

		// Leer bloque del directorio media
		mediaBlockPos := startByte + int64(superblock.SBlockStart) + int64(mediaBlockNum)*int64(blockSize)
		_, err = file.Seek(mediaBlockPos, 0)
		if err == nil {
			mediaBlock := &DirectoryBlock{}
			err = binary.Read(file, binary.LittleEndian, mediaBlock)
			if err == nil {
				// Crear un archivo grande en media
				mediaFileInodeNum, mediaFileBlocks, err := createFile("video.mp4", mediaBlock, blockSize*18)
				if err == nil {
					createdItems = append(createdItems, fmt.Sprintf("Archivo 'media/video.mp4': inodo %d, %d bytes, %d bloques (indirecto)",
						mediaFileInodeNum, blockSize*18, len(mediaFileBlocks)))
				}

				// Guardar el bloque actualizado
				_, err = file.Seek(mediaBlockPos, 0)
				if err == nil {
					err = writeDirectoryBlockToDisc(file, mediaBlock)
				}
			}
		}
	}

	// Actualizar el directorio raíz en disco
	_, err = file.Seek(rootBlockPos, 0)
	if err != nil {
		return false, "Error al posicionarse para actualizar directorio raíz"
	}

	err = writeDirectoryBlockToDisc(file, rootDirBlock)
	if err != nil {
		return false, "Error al actualizar directorio raíz"
	}

	// Contar inodos y bloques usados
	inodesUsed := len(createdItems)
	blocksUsed := 0

	for _, item := range createdItems {
		if strings.Contains(item, "Directorio") {
			blocksUsed++ // Un directorio usa un bloque
		} else if strings.Contains(item, "bloques (indirecto)") {
			var blockCount int
			fmt.Sscanf(strings.Split(item, ", ")[2], "%d bloques", &blockCount)
			blocksUsed += blockCount
		} else if strings.Contains(item, "bloques") {
			var blockCount int
			fmt.Sscanf(strings.Split(item, ", ")[2], "%d bloques", &blockCount)
			blocksUsed += blockCount
		}
	}

	// Actualizar superbloque
	superblock.SFreeBlocksCount -= int32(blocksUsed)
	superblock.SFreeInodesCount -= int32(inodesUsed)

	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, "Error al posicionarse para actualizar superbloque"
	}

	err = writeSuperBlockToDisc(file, superblock)
	if err != nil {
		return false, "Error al actualizar superbloque"
	}

	// Actualizar bitmaps
	_, err = file.Seek(bmInodePos, 0)
	if err != nil {
		return false, "Error al posicionarse para actualizar bitmap de inodos"
	}

	_, err = file.Write(bmInodes)
	if err != nil {
		return false, "Error al actualizar bitmap de inodos"
	}

	_, err = file.Seek(bmBlockPos, 0)
	if err != nil {
		return false, "Error al posicionarse para actualizar bitmap de bloques"
	}

	_, err = file.Write(bmBlocks)
	if err != nil {
		return false, "Error al actualizar bitmap de bloques"
	}

	// Mensaje de éxito
	var message strings.Builder
	message.WriteString(fmt.Sprintf("=== INYECCIÓN EXITOSA: %d INODOS CREADOS ===\n\n", inodesUsed))

	for _, item := range createdItems {
		message.WriteString("• " + item + "\n")
	}

	message.WriteString(fmt.Sprintf("\nTotal: %d inodos y %d bloques utilizados\n", inodesUsed, blocksUsed))
	message.WriteString("\nPara visualizar la estructura:\n")
	message.WriteString("rep -id=151A -path=/home/light/inodos.jpg -name=inode\n")
	message.WriteString("rep -id=151A -path=/home/light/tree.jpg -name=tree\n")

	return true, message.String()
}
