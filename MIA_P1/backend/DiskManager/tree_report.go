package DiskManager

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// InodeDetailedInfo almacena información detallada de un inodo
type InodeDetailedInfo struct {
	Number         int
	Type           byte
	Name           string
	Uid            int32
	Gid            int32
	Size           int32
	Permissions    string
	AccessTime     time.Time
	CreationTime   time.Time
	ModifiedTime   time.Time
	Children       []int
	DirectBlocks   []int32
	ContentPreview string
	// Nuevos campos
	HasContent  bool
	IsVirtual   bool
	Description string
}

// BlockInfo almacena información sobre un bloque
type BlockInfo struct {
	Number     int32   // Número de bloque
	Type       string  // Tipo: "directory", "file", "pointer"
	Content    string  // Descripción del contenido
	References []int   // Inodos que lo referencian
	Parent     int32   // Bloque padre (para indirectos)
	Children   []int32 // Bloques hijos (para bloques indirectos)
}

type DirEntry struct {
	InodeNum int
	Name     string
}

type ContentEntry struct {
	Inode uint32
	Name  string
}

// TreeReporter genera el reporte del sistema de archivos EXT2
func TreeReporter(id, path string) (bool, string) {
	fmt.Println("Iniciando generación de reporte para partición:", id)

	// 1. Montar partición y abrir archivo
	mountedPartition, err := findMountedPartitionById(id)
	if err != nil {
		return false, fmt.Sprintf("Error: %s", err)
	}

	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return false, fmt.Sprintf("Error al abrir el disco: %s", err)
	}
	defer file.Close()

	// 2. Obtener inicio de la partición y leer superbloque
	startByte, _, err := getPartitionDetails(file, mountedPartition)
	if err != nil {
		return false, fmt.Sprintf("Error al obtener detalles de la partición: %s", err)
	}

	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en el superbloque: %s", err)
	}

	superblock, err := readSuperBlockFromDisc(file)
	if err != nil {
		return false, fmt.Sprintf("Error al leer el superbloque: %s", err)
	}

	// 3. Leer el bitmap de inodos
	inodeBitmapPos := startByte + int64(superblock.SBmInodeStart)
	_, err = file.Seek(inodeBitmapPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en el bitmap de inodos: %s", err)
	}

	inodeBitmap := make([]byte, superblock.SInodesCount/8+1)
	_, err = file.Read(inodeBitmap)
	if err != nil {
		return false, fmt.Sprintf("Error al leer el bitmap de inodos: %s", err)
	}

	// 4. Leer todos los inodos en uso
	inodeTableStart := startByte + int64(superblock.SInodeStart)
	inodes := make(map[int]*Inode)
	inodeInfo := make(map[int]struct {
		Name     string
		Type     byte
		Size     int32
		Children []struct {
			Name    string
			InodeID int
		}
	})

	for i := 0; i < int(superblock.SInodesCount); i++ {
		// Omitir los inodos 0 y 1 explícitamente
		if i == 0 || i == 1 {
			continue
		}

		bytePos := i / 8
		bitPos := i % 8

		// Verificar si el inodo está en uso
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
			inodeInfo[i] = struct {
				Name     string
				Type     byte
				Size     int32
				Children []struct {
					Name    string
					InodeID int
				}
			}{
				Name: fmt.Sprintf("inode%d", i),
				Type: inode.IType,
				Size: inode.ISize,
				Children: []struct {
					Name    string
					InodeID int
				}{},
			}
		}
	}
	// 5. Procesar los directorios y leer sus entradas
	blocksStart := startByte + int64(superblock.SBlockStart)
	relationships := make(map[int]map[int]string) // parent -> {child: name}

	// Implementar función para procesar entradas de directorio
	processDirectoryEntries := func(inodeNum int, inode *Inode) []struct {
		Name     string
		InodeNum int
	} {
		result := []struct {
			Name     string
			InodeNum int
		}{}

		// Procesar bloques directos
		for j := 0; j < 12; j++ {
			blockNum := inode.IBlock[j]
			if blockNum <= 0 {
				continue
			}

			// Leer el bloque de directorio
			blockPos := blocksStart + int64(blockNum)*int64(superblock.SBlockSize)
			_, err := file.Seek(blockPos, 0)
			if err != nil {
				continue
			}

			// Leer el bloque completo
			blockData := make([]byte, superblock.SBlockSize)
			bytesRead, err := file.Read(blockData)
			if err != nil {
				continue
			}

			fmt.Printf("Leyendo bloque de directorio %d, tamaño leído: %d bytes\n", blockNum, bytesRead)

			// Procesar cada entrada del directorio usando un reader para evitar problemas de índice
			reader := bytes.NewReader(blockData)
			reader.Seek(0, 0) // Reiniciar posición

			for k := 0; k < B_CONTENT_COUNT; k++ {
				// Leer el nombre (array fijo)
				nameBytes := make([]byte, B_NAME_SIZE)
				_, err := reader.Read(nameBytes)
				if err != nil {
					fmt.Printf("Error leyendo nombre de entrada %d: %s\n", k, err)
					break
				}

				// Leer el número de inodo
				var entryInodeNum int32
				err = binary.Read(reader, binary.LittleEndian, &entryInodeNum)
				if err != nil {
					fmt.Printf("Error leyendo inodo de entrada %d: %s\n", k, err)
					break
				}

				// Extraer el nombre (terminado en null)
				name := ""
				for l := 0; l < B_NAME_SIZE && nameBytes[l] != 0; l++ {
					name += string(nameBytes[l])
				}

				// Solo considerar entradas válidas
				if name != "" && entryInodeNum > 0 && entryInodeNum < 100000 {
					fmt.Printf("Entrada válida en directorio %d: %s -> %d\n", inodeNum, name, entryInodeNum)
					result = append(result, struct {
						Name     string
						InodeNum int
					}{
						Name:     name,
						InodeNum: int(entryInodeNum),
					})
				}
			}
		}

		return result
	}

	// Al inicio del procesamiento de relaciones
	fmt.Println("Procesando relaciones entre inodos:")
	directoryNames := make(map[int]string) // Mapa de inodos a nombres conocidos

	// Establecer nombre del inodo raíz
	directoryNames[2] = "/"

	// Procesar primero el directorio raíz para establecer nombres
	rootDirEntries := processDirectoryEntries(2, inodes[2])
	for _, entry := range rootDirEntries {
		if entry.Name != "." && entry.Name != ".." {
			fmt.Printf("Entrada en directorio raíz: %s -> inodo %d\n", entry.Name, entry.InodeNum)
			directoryNames[entry.InodeNum] = "/" + entry.Name

			// Actualizar relaciones
			if relationships[2] == nil {
				relationships[2] = make(map[int]string)
			}
			relationships[2][entry.InodeNum] = entry.Name

			// Actualizar información del inodo hijo
			if _, exists := inodeInfo[entry.InodeNum]; exists {
				temp := inodeInfo[entry.InodeNum]
				temp.Name = entry.Name
				inodeInfo[entry.InodeNum] = temp
			}
		}
	}

	// Luego procesar el resto de directorios
	for i, inode := range inodes {
		// Omitir el directorio raíz (ya procesado) y no-directorios
		if i == 2 || inode.IType != 0 {
			continue
		}

		entries := processDirectoryEntries(i, inode)
		for _, entry := range entries {
			if entry.Name != "." && entry.Name != ".." {
				parentName := directoryNames[i]
				fmt.Printf("Entrada en directorio %d (%s): %s -> inodo %d\n",
					i, parentName, entry.Name, entry.InodeNum)

				// Construir nombres completos para inodos
				if parentName != "" {
					if parentName == "/" {
						directoryNames[entry.InodeNum] = "/" + entry.Name
					} else {
						directoryNames[entry.InodeNum] = parentName + "/" + entry.Name
					}
				} else {
					directoryNames[entry.InodeNum] = entry.Name
				}

				// Actualizar relaciones
				if relationships[i] == nil {
					relationships[i] = make(map[int]string)
				}
				relationships[i][entry.InodeNum] = entry.Name

				// Actualizar información del inodo hijo
				if _, exists := inodeInfo[entry.InodeNum]; exists {
					temp := inodeInfo[entry.InodeNum]
					temp.Name = entry.Name
					inodeInfo[entry.InodeNum] = temp
				}

				// Actualizar los hijos del directorio
				temp := inodeInfo[i]
				temp.Children = append(temp.Children, struct {
					Name    string
					InodeID int
				}{
					Name:    entry.Name,
					InodeID: entry.InodeNum,
				})
				inodeInfo[i] = temp
			}
		}
	}

	// Actualizar las etiquetas de los inodos con sus nombres completos
	for i := range inodeInfo {
		if fullName, exists := directoryNames[i]; exists {
			temp := inodeInfo[i]
			temp.Name = fullName
			inodeInfo[i] = temp
		}
	}
	// 6. Leer contenido de archivos (para visualización)
	// 6. Leer contenido de archivos (para visualización)
	fileContents := make(map[int]string)

	for i, inode := range inodes {
		// Solo procesar inodos de tipo archivo (1)
		if inode.IType != 1 || inode.ISize <= 0 {
			continue
		}

		// Buscar el primer bloque válido
		var blockNum int32 = -1
		for j := 0; j < 12; j++ {
			if inode.IBlock[j] > 0 {
				blockNum = inode.IBlock[j]
				fmt.Printf("Archivo %d (%s) usa el bloque %d para su contenido\n",
					i, inodeInfo[i].Name, blockNum)
				break
			}
		}

		if blockNum <= 0 {
			continue
		}

		// Calcular la posición del bloque
		blockPos := blocksStart + int64(blockNum)*int64(superblock.SBlockSize)
		fmt.Printf("Leyendo contenido desde la posición %d (bloque %d)\n", blockPos, blockNum)
		fmt.Printf("Preparando para leer %d bytes (tamaño del archivo: %d, tamaño del bloque: %d)\n",
			inode.ISize, inode.ISize, superblock.SBlockSize)

		_, err = file.Seek(blockPos, 0)
		if err != nil {
			fmt.Printf("Error al posicionarse en el bloque: %s\n", err)
			continue
		}

		// Leer el bloque completo
		buffer := make([]byte, superblock.SBlockSize)
		bytesRead, err := file.Read(buffer)
		if err != nil {
			fmt.Printf("Error leyendo el bloque: %s\n", err)
			continue
		}

		fmt.Printf("Bytes leídos: %d\n", bytesRead)

		// Mostrar los primeros bytes para depuración
		fmt.Printf("Primeros bytes (hex): ")
		for i := 0; i < min(16, bytesRead); i++ {
			fmt.Printf("%02X ", buffer[i])
		}
		fmt.Println()

		// Determinar el tamaño real del contenido (el menor entre tamaño del archivo y bytes leídos)
		contentSize := min(int(inode.ISize), bytesRead)

		// Convertir a texto, solo los bytes que nos interesan
		content := ""

		// Si el tamaño es 1, probablemente sea un archivo especial o vacío
		if contentSize == 1 {
			content = "[Archivo de 1 byte]"
		} else {
			// Tomar solo los bytes significativos y convertir a texto
			textBytes := buffer[:contentSize]

			// Filtrar caracteres no imprimibles
			var contentBuilder strings.Builder
			for _, b := range textBytes {
				if b >= 32 && b <= 126 || b == 10 || b == 13 || b == 9 {
					contentBuilder.WriteByte(b)
				}
			}
			content = contentBuilder.String()

			// Si sigue vacío después del filtrado, es un archivo binario o corrupto
			if content == "" {
				content = "[No se pudo leer contenido legible]"
			}
		}

		// Mostrar contenido
		if len(content) > 32 {
			fmt.Printf("Contenido como texto (primeros 32 caracteres): '%s...'\n", content[:32])
			fmt.Printf("Tamaño total del contenido: %d bytes\n", len(content))
		} else {
			fmt.Printf("Contenido como texto: '%s'\n", content)
		}

		// Guardar contenido
		fileContents[i] = content
	}

	// 7. Generar el DOT
	var dotBuilder strings.Builder
	dotBuilder.WriteString("digraph FileSystemTree {\n")
	dotBuilder.WriteString("  rankdir=TB;\n")
	dotBuilder.WriteString("  compound=true;\n")
	dotBuilder.WriteString("  node [fontname=\"Arial\", style=filled];\n")
	dotBuilder.WriteString(fmt.Sprintf("  label=\"Reporte de Árbol de Archivos - EXT2 (Partición %s)\";\n\n", mountedPartition.ID))

	// Subgrafo: Información del Sistema
	dotBuilder.WriteString("  subgraph cluster_info {\n")
	dotBuilder.WriteString("    label=\"Información del Sistema\";\n")
	dotBuilder.WriteString("    style=filled;\n")
	dotBuilder.WriteString("    fillcolor=\"#E1BEE7\";\n")
	dotBuilder.WriteString(fmt.Sprintf("    info [shape=record, label=\"{Información|{Partición: %s}|{Inodos Total: %d}|{Bloques Total: %d}}\"];\n",
		mountedPartition.ID, superblock.SInodesCount, superblock.SBlocksCount))
	dotBuilder.WriteString("  }\n\n")

	// Subgrafo: Estructura de Archivos
	dotBuilder.WriteString("  subgraph cluster_tree {\n")
	dotBuilder.WriteString("    label=\"Estructura de Archivos\";\n")
	dotBuilder.WriteString("    style=filled;\n")
	dotBuilder.WriteString("    fillcolor=\"#FFF9C4\";\n")

	// Nodos para inodos
	for i, info := range inodeInfo {
		var shape, fillcolor, typeStr string
		inode := inodes[i]

		if info.Type == 0 {
			shape = "folder"
			fillcolor = "#E8F5E9"
			typeStr = "Directorio"
		} else if info.Type == 1 {
			shape = "note"
			fillcolor = "#E3F2FD"
			typeStr = "Archivo"
		} else {
			shape = "box"
			fillcolor = "#E0E0E0"
			typeStr = "Otro"
		}

		// Formatear permisos
		permStr := fmt.Sprintf("%o%o%o", inode.IPerm[0], inode.IPerm[1], inode.IPerm[2])

		// Formatear fechas
		cTimeStr := inode.ICtime.Format("2006-01-02 15:04:05")
		mTimeStr := inode.IMtime.Format("2006-01-02 15:04:05")

		// Etiqueta detallada para el inodo
		label := fmt.Sprintf("%s\\nInodo %d - %s\\nTamaño: %d bytes\\nUID: %d GID: %d\\nPermisos: %s\\nCreado: %s\\nÚltima mod: %s",
			info.Name, i, typeStr, info.Size, inode.IUid, inode.IGid, permStr, cTimeStr, mTimeStr)

		dotBuilder.WriteString(fmt.Sprintf("    node%d [label=\"%s\", shape=%s, fillcolor=\"%s\", color=\"black\"];\n",
			i, label, shape, fillcolor))
	}

	// Nodos para contenido de archivos y directorios
	nextNodeId := 10000
	for i, info := range inodeInfo {
		if info.Size <= 0 {
			continue
		}

		contentNodeId := nextNodeId
		nextNodeId++

		var contentLabel string

		if info.Type == 0 { // Directorio
			contentTitle := fmt.Sprintf("Contenido del directorio %s", info.Name)
			contentText := fmt.Sprintf("Tamaño: %d bytes", info.Size)
			inode := inodes[i]

			// Información sobre el bloque
			var blockInfo string
			for j := 0; j < 12; j++ {
				if inode.IBlock[j] > 0 {
					blockInfo = fmt.Sprintf("\\n\\nBloque de datos: %d", inode.IBlock[j])
					break
				}
			}

			if len(info.Children) > 0 {
				contentText += blockInfo + "\\n\\nEntradas del directorio:"
				for _, child := range info.Children {
					if child.Name != "." && child.Name != ".." {
						contentText += fmt.Sprintf("\\n- %s (inodo %d)", child.Name, child.InodeID)
					}
				}
			} else {
				contentText += blockInfo + "\\n\\n[No se encontraron entradas]"
			}

			contentLabel = fmt.Sprintf("%s\\n%s", contentTitle, contentText)

		} else if info.Type == 1 { // Archivo
			inode := inodes[i]
			// Obtener información del bloque utilizado
			var blockInfo string
			for j := 0; j < 12; j++ {
				if inode.IBlock[j] > 0 {
					blockInfo = fmt.Sprintf("Bloque de datos: %d", inode.IBlock[j])
					break
				}
			}

			contentTitle := fmt.Sprintf("Contenido del archivo %s", info.Name)
			contentTitle += fmt.Sprintf("\\n%s", blockInfo)

			content := fileContents[i]
			if content == "" {
				content = fmt.Sprintf("[No se pudo leer el contenido - Archivo de %d bytes]", info.Size)
			} else {
				// Limitar el contenido para mejor visualización
				if len(content) > 32 {
					content = content[:32] + "..."
					// Añadir información sobre el tamaño total
					content += fmt.Sprintf(" (%d bytes en total)", info.Size)
				}
			}

			// Escapar saltos de línea para DOT
			content = strings.Replace(content, "\n", "\\n", -1)

			contentLabel = fmt.Sprintf("%s\\n%s", contentTitle, content)
		}

		fillcolor := "#FFFFCC"
		dotBuilder.WriteString(fmt.Sprintf("    content%d [label=\"%s\", shape=note, fillcolor=\"%s\", color=\"brown\"];\n",
			contentNodeId, contentLabel, fillcolor))

		// Enlace al contenido
		dotBuilder.WriteString(fmt.Sprintf("    node%d -> content%d [label=\"contenido\", color=\"brown\", style=\"dashed\"];\n",
			i, contentNodeId))
	}

	dotBuilder.WriteString("  }\n\n")

	// Relaciones entre inodos
	for parent, children := range relationships {
		for child, name := range children {
			// Ignorar relaciones circulares y especiales
			if parent == child && (name == "." || name == "..") {
				continue
			}

			dotBuilder.WriteString(fmt.Sprintf("  node%d -> node%d [label=\"%s\", color=\"blue\"];\n",
				parent, child, name))
		}
	}

	dotBuilder.WriteString("}\n")

	// 8. Generar imagen
	dotFile := path + ".dot"
	err = os.WriteFile(dotFile, []byte(dotBuilder.String()), 0644)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir archivo DOT: %s", err)
	}

	fmt.Printf("Abriendo reporte: %s\n", path+".png")
	cmd := exec.Command("dot", "-Tjpg", dotFile, "-o", path)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("Error al ejecutar Graphviz: %v\nStdout: %s\nStderr: %s\nArchivo DOT: %s",
			err, stdout.String(), stderr.String(), dotFile)
	}

	return true, fmt.Sprintf("Reporte generado exitosamente en: %s", path)
}

// ReadDirectoryBlockFromDisc lee un bloque de directorio correctamente
func ReadDirectoryBlockFromDisc(file *os.File, blockSize int64) (*DirectoryBlock, error) {
	dirBlock := &DirectoryBlock{}

	// Leer todo el bloque como bytes
	blockData := make([]byte, blockSize)
	_, err := file.Read(blockData)
	if err != nil {
		return nil, err
	}

	// Procesar cada entrada del directorio
	reader := bytes.NewReader(blockData)
	for i := 0; i < B_CONTENT_COUNT; i++ {
		// Leer el nombre (12 bytes fijos)
		nameData := make([]byte, B_NAME_SIZE)
		_, err := reader.Read(nameData)
		if err != nil {
			return nil, err
		}

		// Copiar el nombre sin modificarlo
		copy(dirBlock.BContent[i].BName[:], nameData)

		// Leer el número de inodo (4 bytes)
		err = binary.Read(reader, binary.LittleEndian, &dirBlock.BContent[i].BInodo)
		if err != nil {
			return nil, err
		}
	}

	return dirBlock, nil
}

// ReadFileContent lee el contenido de un archivo correctamente
func ReadFileContent(file *os.File, size int32) (string, error) {
	// Leer el tamaño exacto solicitado
	contentData := make([]byte, size)
	bytesRead, err := file.Read(contentData)
	if err != nil {
		return "", err
	}

	// Verificar que se leyó la cantidad correcta
	if bytesRead < int(size) {
		return "", fmt.Errorf("solo se pudieron leer %d de %d bytes", bytesRead, size)
	}

	// Filtrar caracteres nulos y no imprimibles para texto legible
	filteredContent := []byte{}
	for _, b := range contentData {
		if b >= 32 && b <= 126 || b == 10 || b == 13 || b == 9 {
			filteredContent = append(filteredContent, b)
		}
	}

	return string(filteredContent), nil
}

// Función auxiliar para procesar entradas de directorio
func processDirectoryEntries(file *os.File, inode *Inode, inodeNum int,
	blocksStart int64, blockSize int32) []struct {
	Name     string
	InodeNum int
} {
	result := []struct {
		Name     string
		InodeNum int
	}{}

	// Procesar bloques directos
	for j := 0; j < 12; j++ {
		blockNum := inode.IBlock[j]
		if blockNum <= 0 {
			continue
		}

		// Leer el bloque de directorio
		blockPos := blocksStart + int64(blockNum)*int64(blockSize)
		_, err := file.Seek(blockPos, 0)
		if err != nil {
			continue
		}

		// Leer el bloque completo
		blockData := make([]byte, blockSize)
		_, err = file.Read(blockData)
		if err != nil {
			continue
		}

		// Procesar las entradas
		for k := 0; k < B_CONTENT_COUNT; k++ {
			// Posición en el buffer para la entrada k
			entryPos := k * (B_NAME_SIZE + 4) // nombre (12 bytes) + inodo (4 bytes)

			// Extraer el nombre
			nameBytes := blockData[entryPos : entryPos+B_NAME_SIZE]
			name := ""
			for l := 0; l < B_NAME_SIZE && nameBytes[l] != 0; l++ {
				name += string(nameBytes[l])
			}

			// Extraer el número de inodo
			inodeBytes := blockData[entryPos+B_NAME_SIZE : entryPos+B_NAME_SIZE+4]
			entryInodeNum := int(binary.LittleEndian.Uint32(inodeBytes))

			// Solo considerar entradas válidas
			if name != "" && entryInodeNum > 0 && entryInodeNum < 10000 {
				result = append(result, struct {
					Name     string
					InodeNum int
				}{
					Name:     name,
					InodeNum: entryInodeNum,
				})

				fmt.Printf("Entrada válida en directorio %d: %s -> %d\n",
					inodeNum, name, entryInodeNum)
			}
		}
	}

	return result
}
