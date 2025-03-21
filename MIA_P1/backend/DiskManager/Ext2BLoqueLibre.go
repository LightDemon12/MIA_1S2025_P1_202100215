// DiskManager/directoryUtils.go

package DiskManager

import (
	"encoding/binary"
	"fmt"
	"os"
)

// findDirectoryBlockWithSpace busca un bloque de directorio con espacio o asigna uno nuevo
func findDirectoryBlockWithSpace(file *os.File, startByte int64, superblock *SuperBlock, dirInode *Inode) (*DirectoryBlock, int32, error) {
	// 1. Primero revisamos los bloques directos ya asignados (0-11)
	for i := 0; i < 12; i++ {
		if dirInode.IBlock[i] <= 0 {
			// Encontramos un slot vacío, podemos asignar un nuevo bloque aquí
			return assignNewDirectoryBlock(file, startByte, superblock, dirInode, i, 0, 0)
		}

		// Leer el bloque existente
		blockNum := dirInode.IBlock[i]
		dirBlock, err := readDirectoryBlock(file, startByte, superblock, blockNum)
		if err != nil {
			fmt.Printf("WARN: Error leyendo bloque de directorio %d: %v\n", blockNum, err)
			continue
		}

		// Verificar si hay espacio libre en este bloque
		for j := 0; j < B_CONTENT_COUNT; j++ {
			if dirBlock.BContent[j].BInodo <= 0 {
				// ¡Encontramos espacio disponible!
				fmt.Printf("INFO: Espacio encontrado en bloque directo %d, posición %d\n", blockNum, j)
				return dirBlock, blockNum, nil
			}
		}
	}

	// 2. Si no hay espacio en los bloques directos, revisar el bloque indirecto simple (12)
	fmt.Printf("INFO: Bloques directos llenos, revisando bloque indirecto simple\n")

	if dirInode.IBlock[12] <= 0 {
		// No hay bloque indirecto asignado, crearlo
		indirectBlockNum, err := assignIndirectBlock(file, startByte, superblock, dirInode, 12)
		if err != nil {
			return nil, -1, fmt.Errorf("error asignando bloque indirecto: %v", err)
		}
		dirInode.IBlock[12] = int32(indirectBlockNum)

		// Actualizar el inodo con el nuevo bloque indirecto
		inodePos := startByte + int64(superblock.SInodeStart) + int64(dirInode.IUid)*int64(superblock.SInodeSize)
		_, err = file.Seek(inodePos, 0)
		if err != nil {
			return nil, -1, fmt.Errorf("error al posicionarse para actualizar inodo: %v", err)
		}

		err = writeInodeToDisc(file, dirInode)
		if err != nil {
			return nil, -1, fmt.Errorf("error al actualizar inodo con bloque indirecto: %v", err)
		}
	}

	// Leer el bloque indirecto
	indirectBlockPos := startByte + int64(superblock.SBlockStart) + int64(dirInode.IBlock[12])*int64(superblock.SBlockSize)
	_, err := file.Seek(indirectBlockPos, 0)
	if err != nil {
		return nil, -1, fmt.Errorf("error al posicionarse en bloque indirecto: %v", err)
	}

	// Calcular cuántos punteros a bloques caben en un bloque
	blockPointersPerBlock := int(superblock.SBlockSize) / 4 // int32 = 4 bytes

	// Leer los punteros del bloque indirecto
	indirectPointers := make([]int32, blockPointersPerBlock)
	err = binary.Read(file, binary.LittleEndian, &indirectPointers)
	if err != nil {
		return nil, -1, fmt.Errorf("error leyendo punteros del bloque indirecto: %v", err)
	}

	// Revisar los bloques referenciados por el bloque indirecto
	for i := 0; i < blockPointersPerBlock; i++ {
		if indirectPointers[i] <= 0 {
			// Encontramos un slot vacío, asignar nuevo bloque de directorio
			newDirBlock, newBlockNum, err := assignNewDirectoryBlock(file, startByte, superblock, dirInode, 12, i, int(dirInode.IBlock[12]))
			if err != nil {
				return nil, -1, fmt.Errorf("error asignando nuevo bloque desde indirecto: %v", err)
			}

			return newDirBlock, newBlockNum, nil
		}

		// Leer el bloque de directorio existente
		dirBlock, err := readDirectoryBlock(file, startByte, superblock, indirectPointers[i])
		if err != nil {
			fmt.Printf("WARN: Error leyendo bloque de directorio indirecto %d: %v\n", indirectPointers[i], err)
			continue
		}

		// Verificar si hay espacio libre en este bloque
		for j := 0; j < B_CONTENT_COUNT; j++ {
			if dirBlock.BContent[j].BInodo <= 0 {
				// ¡Encontramos espacio disponible!
				fmt.Printf("INFO: Espacio encontrado en bloque indirecto %d, posición %d\n", indirectPointers[i], j)
				return dirBlock, indirectPointers[i], nil
			}
		}
	}

	// 3. Si llegamos aquí podríamos continuar con bloques indirectos dobles y triples
	// pero por simplicidad, reportamos que el directorio está lleno

	return nil, -1, fmt.Errorf("directorio está lleno (todos los bloques directos e indirectos están ocupados)")
}

// assignIndirectBlock asigna un nuevo bloque indirecto y lo inicializa
func assignIndirectBlock(file *os.File, startByte int64, superblock *SuperBlock, dirInode *Inode, indirectIndex int) (int, error) {
	fmt.Printf("INFO: Asignando nuevo bloque indirecto en índice %d\n", indirectIndex)

	// 1. Cargar el bitmap de bloques
	blockBitmap, err := loadBlockBitmap(file, startByte, superblock)
	if err != nil {
		return -1, fmt.Errorf("error cargando bitmap de bloques: %v", err)
	}

	// 2. Encontrar un bloque libre
	freeBlockNum := findFreeBlock(blockBitmap, int(superblock.SBlocksCount))
	if freeBlockNum < 0 {
		return -1, fmt.Errorf("no hay bloques libres disponibles")
	}

	// 3. Calcular cuántos punteros a bloques caben en un bloque
	blockPointersPerBlock := int(superblock.SBlockSize) / 4 // int32 = 4 bytes

	// 4. Crear un bloque indirecto vacío (todos los punteros en -1)
	indirectPointers := make([]int32, blockPointersPerBlock)
	for i := range indirectPointers {
		indirectPointers[i] = -1
	}

	// 5. Escribir el bloque indirecto al disco
	blockPos := startByte + int64(superblock.SBlockStart) + int64(freeBlockNum)*int64(superblock.SBlockSize)
	_, err = file.Seek(blockPos, 0)
	if err != nil {
		return -1, fmt.Errorf("error al posicionarse para escribir bloque indirecto: %v", err)
	}

	err = binary.Write(file, binary.LittleEndian, indirectPointers)
	if err != nil {
		return -1, fmt.Errorf("error al escribir bloque indirecto: %v", err)
	}

	// 6. Actualizar bitmap de bloques
	blockBitmap[freeBlockNum/8] |= (1 << (freeBlockNum % 8))
	_, err = file.Seek(startByte+int64(superblock.SBmBlockStart), 0)
	if err != nil {
		return -1, fmt.Errorf("error al posicionarse para actualizar bitmap: %v", err)
	}
	_, err = file.Write(blockBitmap)
	if err != nil {
		return -1, fmt.Errorf("error al actualizar bitmap de bloques: %v", err)
	}

	// 7. Actualizar superbloque (decrementar bloques libres)
	superblock.SFreeBlocksCount--
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return -1, fmt.Errorf("error al posicionarse para actualizar superbloque: %v", err)
	}

	err = writeSuperBlockToDisc(file, superblock)
	if err != nil {
		return -1, fmt.Errorf("error al actualizar superbloque: %v", err)
	}

	fmt.Printf("INFO: Nuevo bloque indirecto asignado: %d\n", freeBlockNum)
	return freeBlockNum, nil
}

// assignNewDirectoryBlock asigna un nuevo bloque para un directorio
// Actualizada para manejar bloques indirectos
func assignNewDirectoryBlock(file *os.File, startByte int64, superblock *SuperBlock, dirInode *Inode,
	blockIndex int, indirectPos int, indirectBlockNum int) (*DirectoryBlock, int32, error) {
	fmt.Printf("INFO: Asignando nuevo bloque de directorio (índice=%d, indirectPos=%d)\n", blockIndex, indirectPos)

	// 1. Cargar el bitmap de bloques
	blockBitmap, err := loadBlockBitmap(file, startByte, superblock)
	if err != nil {
		return nil, -1, fmt.Errorf("error cargando bitmap de bloques: %v", err)
	}

	// 2. Encontrar un bloque libre
	freeBlockNum := findFreeBlock(blockBitmap, int(superblock.SBlocksCount))
	if freeBlockNum < 0 {
		return nil, -1, fmt.Errorf("no hay bloques libres disponibles")
	}

	// 3. Crear un nuevo bloque de directorio vacío
	newDirBlock := &DirectoryBlock{}
	for i := 0; i < B_CONTENT_COUNT; i++ {
		newDirBlock.BContent[i].BInodo = -1 // -1 indica entrada no utilizada
		for j := 0; j < 12; j++ {
			newDirBlock.BContent[i].BName[j] = 0
		}
	}

	// 4. Inicializar con entradas "." y ".." si es el primer bloque directo
	if blockIndex == 0 && indirectPos == 0 {
		// "." - referencia a este mismo directorio
		newDirBlock.BContent[0].BInodo = int32(dirInode.IUid) // El ID del inodo actual
		copy(newDirBlock.BContent[0].BName[:], []byte("."))

		// ".." - referencia al directorio padre
		newDirBlock.BContent[1].BInodo = int32(2) // Por defecto apunta a root, ajustar según sea necesario
		copy(newDirBlock.BContent[1].BName[:], []byte(".."))
	}

	// 5. Escribir el nuevo bloque al disco
	blockPos := startByte + int64(superblock.SBlockStart) + int64(freeBlockNum)*int64(superblock.SBlockSize)
	_, err = file.Seek(blockPos, 0)
	if err != nil {
		return nil, -1, fmt.Errorf("error al posicionarse para escribir bloque: %v", err)
	}

	err = writeDirectoryBlockToDisc(file, newDirBlock)
	if err != nil {
		return nil, -1, fmt.Errorf("error al escribir bloque de directorio: %v", err)
	}

	// 6. Actualizar el inodo o bloque indirecto para referenciar el nuevo bloque
	if blockIndex < 12 {
		// Actualizar puntero directo en el inodo
		dirInode.IBlock[blockIndex] = int32(freeBlockNum)

		// Escribir el inodo actualizado
		inodePos := startByte + int64(superblock.SInodeStart) + int64(dirInode.IUid)*int64(superblock.SInodeSize)
		_, err = file.Seek(inodePos, 0)
		if err != nil {
			return nil, -1, fmt.Errorf("error al posicionarse para actualizar inodo: %v", err)
		}

		err = writeInodeToDisc(file, dirInode)
		if err != nil {
			return nil, -1, fmt.Errorf("error al actualizar inodo: %v", err)
		}
	} else {
		// Actualizar puntero en el bloque indirecto
		indirectBlockPos := startByte + int64(superblock.SBlockStart) + int64(indirectBlockNum)*int64(superblock.SBlockSize)
		_, err = file.Seek(indirectBlockPos+int64(indirectPos*4), 0)
		if err != nil {
			return nil, -1, fmt.Errorf("error al posicionarse en bloque indirecto: %v", err)
		}

		// Escribir el puntero al nuevo bloque
		err = binary.Write(file, binary.LittleEndian, int32(freeBlockNum))
		if err != nil {
			return nil, -1, fmt.Errorf("error al actualizar puntero en bloque indirecto: %v", err)
		}
	}

	// 7. Actualizar bitmap de bloques
	blockBitmap[freeBlockNum/8] |= (1 << (freeBlockNum % 8))
	_, err = file.Seek(startByte+int64(superblock.SBmBlockStart), 0)
	if err != nil {
		return nil, -1, fmt.Errorf("error al posicionarse para actualizar bitmap: %v", err)
	}
	_, err = file.Write(blockBitmap)
	if err != nil {
		return nil, -1, fmt.Errorf("error al actualizar bitmap de bloques: %v", err)
	}

	// 8. Actualizar el superbloque (decrementar bloques libres)
	superblock.SFreeBlocksCount--
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return nil, -1, fmt.Errorf("error al posicionarse para actualizar superbloque: %v", err)
	}

	err = writeSuperBlockToDisc(file, superblock)
	if err != nil {
		return nil, -1, fmt.Errorf("error al actualizar superbloque: %v", err)
	}

	fmt.Printf("INFO: Nuevo bloque de directorio asignado: %d\n", freeBlockNum)
	return newDirBlock, int32(freeBlockNum), nil
}

// assignNewDirectoryBlock asigna un nuevo bloque para un directorio

// readDirectoryBlock lee un bloque de directorio del disco
func readDirectoryBlock(file *os.File, startByte int64, superblock *SuperBlock, blockNum int32) (*DirectoryBlock, error) {
	if blockNum <= 0 {
		return nil, fmt.Errorf("número de bloque inválido: %d", blockNum)
	}

	blockPos := startByte + int64(superblock.SBlockStart) + int64(blockNum)*int64(superblock.SBlockSize)
	_, err := file.Seek(blockPos, 0)
	if err != nil {
		return nil, err
	}

	dirBlock := &DirectoryBlock{}
	err = binary.Read(file, binary.LittleEndian, dirBlock)
	if err != nil {
		return nil, err
	}

	return dirBlock, nil
}

// findFreeBlock encuentra un bloque libre en el bitmap
func findFreeBlock(bitmap []byte, maxBlocks int) int {
	// Comenzar desde el bloque 50 para evitar bloques del sistema
	for i := 50; i < maxBlocks; i++ {
		bytePos := i / 8
		bitPos := i % 8

		if bytePos < len(bitmap) && (bitmap[bytePos]&(1<<bitPos)) == 0 {
			return i
		}
	}
	return -1
}

// getInodePosition calcula la posición de un inodo en el disco
func getInodePosition(startByte int64, superblock *SuperBlock, inodeNum int) int64 {
	return startByte + int64(superblock.SInodeStart) + int64(inodeNum)*int64(superblock.SInodeSize)
}
