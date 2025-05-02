package DiskManager

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"time"
)

// FSNode representa un elemento del sistema de archivos (archivo o directorio)
type FSNode struct {
	Name        string    `json:"name"`
	Type        string    `json:"type"` // "directory" o "file"
	Path        string    `json:"path"`
	Size        int32     `json:"size"`
	InodeNum    int       `json:"inodeNum"`
	Permissions string    `json:"permissions"`
	Owner       string    `json:"owner"`
	Group       string    `json:"group"`
	CreatedAt   time.Time `json:"createdAt"`
	ModifiedAt  time.Time `json:"modifiedAt"`
	AccessedAt  time.Time `json:"accessedAt"`
	Content     string    `json:"content,omitempty"`  // Solo para archivos
	Children    []*FSNode `json:"children,omitempty"` // Solo para directorios
}

// FileSystemInfo representa la información general del sistema de archivos
type FSInfo struct {
	PartitionID   string    `json:"partitionId"`
	PartitionPath string    `json:"partitionPath"`
	TotalInodes   int32     `json:"totalInodes"`
	UsedInodes    int32     `json:"usedInodes"`
	FreeInodes    int32     `json:"freeInodes"`
	TotalBlocks   int32     `json:"totalBlocks"`
	UsedBlocks    int32     `json:"usedBlocks"`
	FreeBlocks    int32     `json:"freeBlocks"`
	BlockSize     int32     `json:"blockSize"`
	CreatedAt     time.Time `json:"createdAt"`
	RootNode      *FSNode   `json:"rootNode"`
}

// GetFileSystemStructure extrae la estructura del sistema de archivos de una partición montada
func GetFileSystemStructure(id string) (*FSInfo, error) {
	// 1. Encontrar la partición montada
	mountedPartition, err := FindMountedPartitionById(id)
	if err != nil {
		return nil, fmt.Errorf("error: %s", err)
	}

	// 2. Abrir el archivo de disco
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("error al abrir el disco: %s", err)
	}
	defer file.Close()

	// 3. Obtener inicio de la partición y leer superbloque
	startByte, _, err := GetPartitionDetails(file, mountedPartition)
	if err != nil {
		return nil, fmt.Errorf("error al obtener detalles de la partición: %s", err)
	}

	_, err = file.Seek(startByte, 0)
	if err != nil {
		return nil, fmt.Errorf("error al posicionarse en el superbloque: %s", err)
	}

	superblock, err := ReadSuperBlockFromDisc(file)
	if err != nil {
		return nil, fmt.Errorf("error al leer el superbloque: %s", err)
	}

	// 4. Crear estructura básica de información del sistema de archivos
	fsInfo := &FSInfo{
		PartitionID:   id,
		PartitionPath: mountedPartition.DiskPath,
		TotalInodes:   superblock.SInodesCount,
		FreeInodes:    superblock.SFreeInodesCount,
		UsedInodes:    superblock.SInodesCount - superblock.SFreeInodesCount,
		TotalBlocks:   superblock.SBlocksCount,
		FreeBlocks:    superblock.SFreeBlocksCount,
		UsedBlocks:    superblock.SBlocksCount - superblock.SFreeBlocksCount,
		BlockSize:     superblock.SBlockSize,
		CreatedAt:     superblock.SMtime,
	}

	// 5. Leer el bitmap de inodos
	inodeBitmapPos := startByte + int64(superblock.SBmInodeStart)
	_, err = file.Seek(inodeBitmapPos, 0)
	if err != nil {
		return nil, fmt.Errorf("error al posicionarse en el bitmap de inodos: %s", err)
	}

	inodeBitmap := make([]byte, superblock.SInodesCount/8+1)
	_, err = file.Read(inodeBitmap)
	if err != nil {
		return nil, fmt.Errorf("error al leer el bitmap de inodos: %s", err)
	}

	// 6. Leer todos los inodos en uso
	inodeTableStart := startByte + int64(superblock.SInodeStart)
	inodes := make(map[int]*Inode)

	for i := 0; i < int(superblock.SInodesCount); i++ {
		bytePos := i / 8
		bitPos := i % 8

		if bytePos < len(inodeBitmap) && (inodeBitmap[bytePos]&(1<<bitPos)) != 0 {
			inodePos := inodeTableStart + int64(i)*int64(superblock.SInodeSize)
			_, err = file.Seek(inodePos, 0)
			if err != nil {
				continue
			}

			inode, err := readInodeFromDisc(file)
			if err != nil {
				continue
			}

			inodes[i] = inode
		}
	}

	// 7. Mapeo de nombres de archivos y directorios
	// Usamos un enfoque diferente al anterior: procesamos recursivamente empezando por el inodo raíz

	// Mapa para evitar recursión infinita
	processedInodes := make(map[int]bool)

	// Mapa para guardar las rutas completas de los inodos
	inodePaths := make(map[int]string)
	inodePaths[2] = "/" // El inodo 2 es el directorio raíz

	// Calculamos la posición inicial de los bloques - MOVIDO AQUÍ PARA CORREGIR EL ERROR
	blocksStart := startByte + int64(superblock.SBlockStart)

	// Función para construir el árbol del sistema de archivos recursivamente
	var buildFSTree func(inodeNum int, path string) *FSNode
	buildFSTree = func(inodeNum int, path string) *FSNode {
		if processedInodes[inodeNum] {
			return nil // Evitar ciclos
		}
		processedInodes[inodeNum] = true

		inode, exists := inodes[inodeNum]
		if !exists {
			return nil
		}

		// Determinar tipo (directorio o archivo)
		nodeType := "file"
		if inode.IType == 0 {
			nodeType = "directory"
		}

		// Crear el nodo básico
		node := &FSNode{
			Name:        getBaseNameFromPath(path),
			Type:        nodeType,
			Path:        path,
			Size:        inode.ISize,
			InodeNum:    inodeNum,
			Permissions: fmt.Sprintf("%o%o%o", inode.IPerm[0], inode.IPerm[1], inode.IPerm[2]),
			Owner:       fmt.Sprintf("uid:%d", inode.IUid),
			Group:       fmt.Sprintf("gid:%d", inode.IGid),
			CreatedAt:   inode.ICtime,
			ModifiedAt:  inode.IMtime,
			AccessedAt:  inode.IAtime,
		}

		// Si es un directorio, leer sus entradas
		if nodeType == "directory" {
			directoryEntries := readDirectoryEntries(file, inode, startByte, superblock.SBlockSize, blocksStart)

			for _, entry := range directoryEntries {
				// Ignorar entradas . y ..
				if entry.Name == "." || entry.Name == ".." {
					continue
				}

				// Construir ruta para el hijo
				childPath := path
				if path == "/" {
					childPath += entry.Name
				} else {
					childPath += "/" + entry.Name
				}

				// Guardar la ruta para este inodo
				inodePaths[entry.InodeNum] = childPath

				// Construir recursivamente el hijo
				childNode := buildFSTree(entry.InodeNum, childPath)
				if childNode != nil {
					node.Children = append(node.Children, childNode)
				}
			}
		} else if nodeType == "file" {
			// Si es un archivo, leer su contenido
			node.Content = readNodeFileContent(file, inode, startByte, superblock.SBlockSize, blocksStart)
		}

		return node
	}

	// Iniciar la construcción del árbol desde el directorio raíz (inodo 2)
	rootNode := buildFSTree(2, "/")
	fsInfo.RootNode = rootNode

	return fsInfo, nil
}

// Función auxiliar para leer las entradas de un directorio
func readDirectoryEntries(file *os.File, inode *Inode, startByte int64, blockSize int32, blocksStart int64) []struct {
	Name     string
	InodeNum int
} {
	var entries []struct {
		Name     string
		InodeNum int
	}

	// Leer los bloques directos que contienen entradas de directorio
	for i := 0; i < 12; i++ {
		blockNum := inode.IBlock[i]
		if blockNum <= 0 {
			continue
		}

		blockPos := blocksStart + int64(blockNum)*int64(blockSize)
		_, err := file.Seek(blockPos, 0)
		if err != nil {
			continue
		}

		// Leer el bloque
		blockData := make([]byte, blockSize)
		bytesRead, err := file.Read(blockData)
		if err != nil || bytesRead == 0 {
			continue
		}

		// Procesar entradas del directorio
		reader := bytes.NewReader(blockData)

		for j := 0; j < B_CONTENT_COUNT; j++ {
			// Leer nombre
			nameBytes := make([]byte, B_NAME_SIZE)
			_, err := reader.Read(nameBytes)
			if err != nil {
				break
			}

			// Leer inodo
			var entryInodeNum int32
			err = binary.Read(reader, binary.LittleEndian, &entryInodeNum)
			if err != nil {
				break
			}

			// Extraer nombre terminado en null
			name := ""
			for k := 0; k < B_NAME_SIZE && nameBytes[k] != 0; k++ {
				name += string(nameBytes[k])
			}

			// Solo incluir entradas válidas
			if name != "" && entryInodeNum > 0 {
				entries = append(entries, struct {
					Name     string
					InodeNum int
				}{
					Name:     name,
					InodeNum: int(entryInodeNum),
				})
			}
		}
	}

	return entries
}

// NOMBRE CAMBIADO para evitar conflicto con otra función existente
// Función para leer el contenido de un archivo
func readNodeFileContent(file *os.File, inode *Inode, startByte int64, blockSize int32, blocksStart int64) string {
	var content strings.Builder

	// Leer bloques directos
	for i := 0; i < 12; i++ {
		blockNum := inode.IBlock[i]
		if blockNum <= 0 {
			continue
		}

		blockPos := blocksStart + int64(blockNum)*int64(blockSize)
		_, err := file.Seek(blockPos, 0)
		if err != nil {
			continue
		}

		// Leer el bloque
		buffer := make([]byte, blockSize)
		bytesRead, err := file.Read(buffer)
		if err != nil {
			continue
		}

		// Determinar cuánto del bloque pertenece al archivo
		contentSize := min(int(inode.ISize), bytesRead)

		// Filtrar caracteres no imprimibles
		for _, b := range buffer[:contentSize] {
			if b >= 32 && b <= 126 || b == 10 || b == 13 || b == 9 {
				content.WriteByte(b)
			}
		}
	}

	// Restricción de contenido (para archivos grandes)
	if content.Len() > 1000 {
		return content.String()[:1000] + "... (contenido truncado)"
	}

	return content.String()
}

// Obtiene el nombre base de una ruta
func getBaseNameFromPath(path string) string {
	if path == "/" {
		return "/"
	}

	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}
