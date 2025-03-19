package DiskManager

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"time"
)

// EXT2AutoInjector crea una estructura de directorios y archivos con contenido simple
func EXT2AutoInjector(id string) (bool, string) {
	fmt.Println("Iniciando inyección de archivos en partición:", id)

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

	fmt.Printf("Superbloque leído: inodos=%d, bloques=%d\n",
		superblock.SInodesCount, superblock.SBlocksCount)

	// Posiciones importantes
	inodeTablePos := startByte + int64(superblock.SInodeStart)
	bmInodePos := startByte + int64(superblock.SBmInodeStart)
	bmBlockPos := startByte + int64(superblock.SBmBlockStart)

	fmt.Printf("Posiciones: inodeTable=%d, bmInode=%d, bmBlock=%d\n",
		inodeTablePos, bmInodePos, bmBlockPos)

	// Leer bitmaps actuales
	_, err = file.Seek(bmInodePos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en bitmap de inodos: %s", err)
	}

	bmInodes := make([]byte, superblock.SInodesCount/8+1)
	_, err = file.Read(bmInodes)
	if err != nil {
		return false, fmt.Sprintf("Error al leer bitmap de inodos: %s", err)
	}

	_, err = file.Seek(bmBlockPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en bitmap de bloques: %s", err)
	}

	bmBlocks := make([]byte, superblock.SBlocksCount/8+1)
	_, err = file.Read(bmBlocks)
	if err != nil {
		return false, fmt.Sprintf("Error al leer bitmap de bloques: %s", err)
	}

	// Leer inodo raíz (inodo 2)
	rootInodePos := inodeTablePos + 2*int64(superblock.SInodeSize)
	_, err = file.Seek(rootInodePos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en inodo raíz: %s", err)
	}

	rootInode, err := readInodeFromDisc(file)
	if err != nil {
		return false, fmt.Sprintf("Error al leer inodo raíz: %s", err)
	}

	fmt.Printf("Inodo raíz: tipo=%d, bloques[0]=%d\n", rootInode.IType, rootInode.IBlock[0])

	// Leer bloque de directorio raíz
	rootBlockPos := startByte + int64(superblock.SBlockStart) + int64(rootInode.IBlock[0])*int64(superblock.SBlockSize)
	_, err = file.Seek(rootBlockPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en bloque raíz: %s", err)
	}

	rootDirBlock := &DirectoryBlock{}
	err = binary.Read(file, binary.LittleEndian, rootDirBlock)
	if err != nil {
		return false, fmt.Sprintf("Error al leer bloque raíz: %s", err)
	}

	// Imprimir entradas actuales del directorio raíz
	fmt.Println("Entradas actuales del directorio raíz:")
	for i := 0; i < B_CONTENT_COUNT; i++ {
		name := strings.TrimRight(string(rootDirBlock.BContent[i].BName[:]), "\x00")
		fmt.Printf("[%d] '%s' -> inodo %d\n", i, name, rootDirBlock.BContent[i].BInodo)
	}

	// Funciones auxiliares
	// 1. Encontrar inodo libre
	findFreeInode := func() int {
		for i := EXT2_RESERVED_INODES; i < int(superblock.SInodesCount); i++ {
			byteIdx := i / 8
			bitIdx := i % 8
			if byteIdx >= len(bmInodes) {
				continue // Índice fuera de rango
			}
			if (bmInodes[byteIdx] & (1 << bitIdx)) == 0 {
				// Marcar como usado
				bmInodes[byteIdx] |= (1 << bitIdx)
				fmt.Printf("Encontrado inodo libre: %d\n", i)
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
			if byteIdx >= len(bmBlocks) {
				continue // Índice fuera de rango
			}
			if (bmBlocks[byteIdx] & (1 << bitIdx)) == 0 {
				// Marcar como usado
				bmBlocks[byteIdx] |= (1 << bitIdx)
				fmt.Printf("Encontrado bloque libre: %d\n", i)
				return i
			}
		}
		return -1
	}

	// 3. Buscar entrada libre en un directorio
	findFreeEntry := func(dirBlock *DirectoryBlock) int {
		fmt.Println("Buscando entrada libre en directorio...")
		for i := 0; i < B_CONTENT_COUNT; i++ {
			// Verificar si es una entrada libre (valor -1 o 0)
			if dirBlock.BContent[i].BInodo <= 0 {
				fmt.Printf("Entrada libre encontrada en posición %d\n", i)
				return i
			}
		}
		fmt.Println("No se encontraron entradas libres")
		return -1
	}

	// 4. Crear un nuevo directorio
	createDirectory := func(name string, parentDirBlock *DirectoryBlock, parentInodeNum int) (int, int, error) {
		fmt.Printf("Creando directorio: '%s'\n", name)

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

		fmt.Printf("Usando entrada %d del directorio padre para '%s'\n", entryIdx, name)

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
		dirInode.ISize = 64 // Tamaño estándar para directorio

		// Crear el bloque de directorio
		dirBlock := &DirectoryBlock{}
		for i := 0; i < B_CONTENT_COUNT; i++ {
			// Inicializar con -1 para indicar entradas libres
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
			return -1, -1, fmt.Errorf("error al posicionarse para escribir inodo: %s", err)
		}

		err = writeInodeToDisc(file, dirInode)
		if err != nil {
			return -1, -1, fmt.Errorf("error al escribir inodo: %s", err)
		}

		// Escribir el bloque del directorio
		dirBlockPos := startByte + int64(superblock.SBlockStart) + int64(dirBlockNum)*int64(superblock.SBlockSize)
		_, err = file.Seek(dirBlockPos, 0)
		if err != nil {
			return -1, -1, fmt.Errorf("error al posicionarse para escribir bloque: %s", err)
		}

		err = writeDirectoryBlockToDisc(file, dirBlock)
		if err != nil {
			return -1, -1, fmt.Errorf("error al escribir bloque: %s", err)
		}

		// Añadir entrada en el directorio padre
		for j := range parentDirBlock.BContent[entryIdx].BName {
			parentDirBlock.BContent[entryIdx].BName[j] = 0 // Limpiar nombre
		}
		copy(parentDirBlock.BContent[entryIdx].BName[:], []byte(name))
		parentDirBlock.BContent[entryIdx].BInodo = int32(dirInodeNum)

		fmt.Printf("Directorio '%s' creado: inodo=%d, bloque=%d\n",
			name, dirInodeNum, dirBlockNum)

		return dirInodeNum, dirBlockNum, nil
	}

	// 5. Crear archivo con contenido específico
	createTextFile := func(name string, parentDirBlock *DirectoryBlock, content string) (int, int, error) {
		fmt.Printf("Creando archivo: '%s' con contenido: '%s'\n", name, content)

		// Buscar inodo libre
		fileInodeNum := findFreeInode()
		if fileInodeNum == -1 {
			return -1, -1, fmt.Errorf("no hay inodos libres para el archivo")
		}

		// Buscar bloque libre para el contenido
		fileBlockNum := findFreeBlock()
		if fileBlockNum == -1 {
			return -1, -1, fmt.Errorf("no hay bloques libres para el archivo")
		}

		// Buscar entrada libre en directorio padre
		entryIdx := findFreeEntry(parentDirBlock)
		if entryIdx == -1 {
			return -1, -1, fmt.Errorf("no hay entradas libres en el directorio padre")
		}

		fmt.Printf("Usando entrada %d del directorio padre para '%s'\n", entryIdx, name)

		// Crear inodo para el archivo
		fileInode := NewInode(0, 0, INODE_FILE)
		fileInode.IPerm[0] = 6 // rw-
		fileInode.IPerm[1] = 4 // r--
		fileInode.IPerm[2] = 4 // r--
		fileInode.ISize = int32(len(content))

		// Inicializar todos los bloques a -1
		for i := 0; i < 15; i++ {
			fileInode.IBlock[i] = -1
		}

		// Asignar el bloque al inodo
		fileInode.IBlock[0] = int32(fileBlockNum)

		// Escribir el contenido al bloque
		blockPos := startByte + int64(superblock.SBlockStart) + int64(fileBlockNum)*int64(superblock.SBlockSize)
		_, err = file.Seek(blockPos, 0)
		if err != nil {
			return -1, -1, fmt.Errorf("error al posicionarse para escribir contenido: %s", err)
		}

		// Crear y escribir el bloque de archivo
		fileBlock := NewFileBlock()
		fileBlock.WriteContent([]byte(content))

		err = binary.Write(file, binary.LittleEndian, fileBlock.BContent)
		if err != nil {
			return -1, -1, fmt.Errorf("error al escribir contenido: %s", err)
		}

		// Escribir el inodo
		fileInodePos := inodeTablePos + int64(fileInodeNum)*int64(superblock.SInodeSize)
		_, err = file.Seek(fileInodePos, 0)
		if err != nil {
			return -1, -1, fmt.Errorf("error al posicionarse para escribir inodo: %s", err)
		}

		err = writeInodeToDisc(file, fileInode)
		if err != nil {
			return -1, -1, fmt.Errorf("error al escribir inodo: %s", err)
		}

		// Añadir entrada al directorio padre
		for j := range parentDirBlock.BContent[entryIdx].BName {
			parentDirBlock.BContent[entryIdx].BName[j] = 0 // Limpiar nombre
		}
		copy(parentDirBlock.BContent[entryIdx].BName[:], []byte(name))
		parentDirBlock.BContent[entryIdx].BInodo = int32(fileInodeNum)

		fmt.Printf("Archivo '%s' creado: inodo=%d, bloque=%d\n",
			name, fileInodeNum, fileBlockNum)

		return fileInodeNum, fileBlockNum, nil
	}

	// Crear una estructura de directorios y archivos con contenido simple
	createdItems := []string{}

	// 1. Crear archivos en la raíz
	helloInodeNum, helloBlockNum, err := createTextFile("hola.txt", rootDirBlock, "¡Hola, mundo!")
	if err != nil {
		fmt.Printf("Error al crear hola.txt: %s\n", err)
	} else {
		createdItems = append(createdItems, fmt.Sprintf("Archivo 'hola.txt': inodo %d, bloque %d, contenido: '¡Hola, mundo!'",
			helloInodeNum, helloBlockNum))
	}

	// Actualizar el directorio raíz después de cada modificación importante
	_, err = file.Seek(rootBlockPos, 0)
	if err == nil {
		err = writeDirectoryBlockToDisc(file, rootDirBlock)
		if err != nil {
			fmt.Printf("Error al actualizar directorio raíz: %s\n", err)
		}
	}

	notesInodeNum, notesBlockNum, err := createTextFile("notas.txt", rootDirBlock, "Lista de tareas:\n1. Estudiar EXT2\n2. Completar proyecto")
	if err != nil {
		fmt.Printf("Error al crear notas.txt: %s\n", err)
	} else {
		createdItems = append(createdItems, fmt.Sprintf("Archivo 'notas.txt': inodo %d, bloque %d, contenido: lista de tareas",
			notesInodeNum, notesBlockNum))
	}

	// Actualizar el directorio raíz después de cada modificación importante
	_, err = file.Seek(rootBlockPos, 0)
	if err == nil {
		err = writeDirectoryBlockToDisc(file, rootDirBlock)
		if err != nil {
			fmt.Printf("Error al actualizar directorio raíz: %s\n", err)
		}
	}

	// 2. Crear directorios
	// Crear directorio documentos
	docsInodeNum, docsBlockNum, err := createDirectory("docs", rootDirBlock, 2)
	if err != nil {
		fmt.Printf("Error al crear directorio docs: %s\n", err)
	} else {
		createdItems = append(createdItems, fmt.Sprintf("Directorio 'docs': inodo %d, bloque %d",
			docsInodeNum, docsBlockNum))

		// Actualizar el directorio raíz
		_, err = file.Seek(rootBlockPos, 0)
		if err == nil {
			err = writeDirectoryBlockToDisc(file, rootDirBlock)
		}

		// Leer el bloque del directorio documentos
		docsBlockPos := startByte + int64(superblock.SBlockStart) + int64(docsBlockNum)*int64(superblock.SBlockSize)
		_, err = file.Seek(docsBlockPos, 0)
		if err == nil {
			docsBlock := &DirectoryBlock{}
			err = binary.Read(file, binary.LittleEndian, docsBlock)
			if err == nil {
				// Crear archivo dentro de documentos
				readmeInodeNum, readmeBlockNum, err := createTextFile("leeme.txt", docsBlock, "Este es el directorio de documentos.")
				if err != nil {
					fmt.Printf("Error al crear leeme.txt: %s\n", err)
				} else {
					createdItems = append(createdItems, fmt.Sprintf("Archivo 'docs/leeme.txt': inodo %d, bloque %d",
						readmeInodeNum, readmeBlockNum))
				}

				// Guardar el bloque actualizado
				_, err = file.Seek(docsBlockPos, 0)
				if err == nil {
					err = writeDirectoryBlockToDisc(file, docsBlock)
					if err != nil {
						fmt.Printf("Error al actualizar directorio docs: %s\n", err)
					}
				}
			}
		}
	}

	// Contar inodos y bloques usados
	inodesUsed := len(createdItems)
	blocksUsed := len(createdItems) // Simplificación: cada item usa un bloque

	// Actualizar superbloque
	superblock.SFreeBlocksCount -= int32(blocksUsed)
	superblock.SFreeInodesCount -= int32(inodesUsed)
	superblock.SMtime = time.Now()

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
	message.WriteString(fmt.Sprintf("=== INYECCIÓN EXITOSA: %d ARCHIVOS Y DIRECTORIOS CREADOS ===\n\n", len(createdItems)))

	for _, item := range createdItems {
		message.WriteString("• " + item + "\n")
	}

	message.WriteString(fmt.Sprintf("\nTotal: %d inodos y %d bloques utilizados\n", inodesUsed, blocksUsed))
	message.WriteString("\nPara visualizar la estructura:\n")
	message.WriteString("rep -id=" + id + " -path=/home/reportes/inodos.jpg -name=inode\n")
	message.WriteString("rep -id=" + id + " -path=/home/reportes/tree.jpg -name=tree\n")

	return true, message.String()
}
