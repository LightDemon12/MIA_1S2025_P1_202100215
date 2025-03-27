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
	mountedPartition, err := FindMountedPartitionById(id)
	if err != nil {
		return false, fmt.Sprintf("Error: %s", err)
	}

	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return false, fmt.Sprintf("Error al abrir el disco: %s", err)
	}
	defer file.Close()

	// 2. Obtener inicio de la partición y leer superbloque
	startByte, _, err := GetPartitionDetails(file, mountedPartition)
	if err != nil {
		return false, fmt.Sprintf("Error al obtener detalles de la partición: %s", err)
	}

	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en el superbloque: %s", err)
	}

	superblock, err := ReadSuperBlockFromDisc(file)
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

	fmt.Println("Analizando bloques del sistema de archivos...")

	// Leer el bitmap de bloques
	blockBitmapPos := startByte + int64(superblock.SBmBlockStart)
	_, err = file.Seek(blockBitmapPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en el bitmap de bloques: %s", err)
	}

	blockBitmap := make([]byte, superblock.SBlocksCount/8+1)
	_, err = file.Read(blockBitmap)
	if err != nil {
		return false, fmt.Sprintf("Error al leer el bitmap de bloques: %s", err)
	}

	// Mapa para almacenar información de los bloques
	type ExtendedBlockInfo struct {
		Number        int32  // Número de bloque
		Type          string // Tipo: "directory", "file", "pointer", "system", "free"
		InUse         bool   // Si está en uso
		ReferencedBy  []int  // Inodos que lo referencian
		IndirectLevel int    // 0=directo, 1=simple, 2=doble, 3=triple
		Parent        int32  // Bloque indirecto que lo referencia (si aplica)
	}

	blocks := make(map[int32]ExtendedBlockInfo)

	// 1. Identificar bloques en uso según el bitmap
	for i := 0; i < int(superblock.SBlocksCount); i++ {
		bytePos := i / 8
		bitPos := i % 8
		inUse := false

		if bytePos < len(blockBitmap) && (blockBitmap[bytePos]&(1<<bitPos)) != 0 {
			inUse = true
		}

		blocks[int32(i)] = ExtendedBlockInfo{
			Number:        int32(i),
			Type:          "unknown",
			InUse:         inUse,
			ReferencedBy:  []int{},
			IndirectLevel: 0,
		}
	}

	// 2. Analizar referencias a bloques desde inodos
	fmt.Println("Analizando referencias de bloques desde inodos...")
	blocksStart := startByte + int64(superblock.SBlockStart)

	for inodeNum, inode := range inodes {
		// Bloques directos (0-11)
		for i := 0; i < 12; i++ {
			blockNum := inode.IBlock[i]
			if blockNum <= 0 {
				continue
			}

			if info, exists := blocks[blockNum]; exists {
				// Determinar tipo según el tipo de inodo
				if inode.IType == 0 {
					info.Type = "directory"
				} else {
					info.Type = "file"
				}

				info.ReferencedBy = append(info.ReferencedBy, inodeNum)
				blocks[blockNum] = info
			}
		}

		// Bloque indirecto simple (12)
		if inode.IBlock[12] > 0 {
			blockNum := inode.IBlock[12]

			if info, exists := blocks[blockNum]; exists {
				info.Type = "pointer"
				info.IndirectLevel = 1
				info.ReferencedBy = append(info.ReferencedBy, inodeNum)
				blocks[blockNum] = info

				// Leer el bloque de punteros
				blockPos := blocksStart + int64(blockNum)*int64(superblock.SBlockSize)
				_, err := file.Seek(blockPos, 0)
				if err == nil {
					// Leer como un PointerBlock
					var pointerBlock PointerBlock

					// Leer cada puntero (int32)
					for j := 0; j < POINTERS_PER_BLOCK; j++ {
						err := binary.Read(file, binary.LittleEndian, &pointerBlock.BPointers[j])
						if err != nil {
							break
						}
					}

					// Procesar punteros válidos
					for j := 0; j < POINTERS_PER_BLOCK; j++ {
						refBlockNum := pointerBlock.BPointers[j]
						if refBlockNum <= 0 || refBlockNum == POINTER_UNUSED_VALUE {
							continue
						}

						if refInfo, exists := blocks[refBlockNum]; exists {
							if inode.IType == 0 {
								refInfo.Type = "directory"
							} else {
								refInfo.Type = "file"
							}
							refInfo.Parent = blockNum
							refInfo.ReferencedBy = append(refInfo.ReferencedBy, inodeNum)
							blocks[refBlockNum] = refInfo
						}
					}
				}
			}
		}

		// Bloque indirecto doble (13)
		if inode.IBlock[13] > 0 {
			blockNum := inode.IBlock[13]

			if info, exists := blocks[blockNum]; exists {
				info.Type = "pointer"
				info.IndirectLevel = 2
				info.ReferencedBy = append(info.ReferencedBy, inodeNum)
				blocks[blockNum] = info

				// Leer el bloque de punteros nivel 1
				blockPos := blocksStart + int64(blockNum)*int64(superblock.SBlockSize)
				_, err := file.Seek(blockPos, 0)
				if err == nil {
					var l1PointerBlock PointerBlock

					// Leer cada puntero de nivel 1
					for j := 0; j < POINTERS_PER_BLOCK; j++ {
						err := binary.Read(file, binary.LittleEndian, &l1PointerBlock.BPointers[j])
						if err != nil {
							break
						}
					}

					// Procesar punteros válidos nivel 1
					for j := 0; j < POINTERS_PER_BLOCK; j++ {
						l1BlockNum := l1PointerBlock.BPointers[j]
						if l1BlockNum <= 0 || l1BlockNum == POINTER_UNUSED_VALUE {
							continue
						}

						// Marcar el bloque de punteros nivel 1
						if l1Info, exists := blocks[l1BlockNum]; exists {
							l1Info.Type = "pointer"
							l1Info.IndirectLevel = 1
							l1Info.Parent = blockNum
							l1Info.ReferencedBy = append(l1Info.ReferencedBy, inodeNum)
							blocks[l1BlockNum] = l1Info

							// Leer punteros de nivel 2 (datos)
							l1BlockPos := blocksStart + int64(l1BlockNum)*int64(superblock.SBlockSize)
							_, err := file.Seek(l1BlockPos, 0)
							if err == nil {
								var l2PointerBlock PointerBlock

								// Leer punteros de nivel 2
								for k := 0; k < POINTERS_PER_BLOCK; k++ {
									err := binary.Read(file, binary.LittleEndian, &l2PointerBlock.BPointers[k])
									if err != nil {
										break
									}
								}

								// Procesar punteros nivel 2
								for k := 0; k < POINTERS_PER_BLOCK; k++ {
									dataBlockNum := l2PointerBlock.BPointers[k]
									if dataBlockNum <= 0 || dataBlockNum == POINTER_UNUSED_VALUE {
										continue
									}

									// Marcar el bloque de datos
									if dataInfo, exists := blocks[dataBlockNum]; exists {
										if inode.IType == 0 {
											dataInfo.Type = "directory"
										} else {
											dataInfo.Type = "file"
										}
										dataInfo.Parent = l1BlockNum
										dataInfo.ReferencedBy = append(dataInfo.ReferencedBy, inodeNum)
										blocks[dataBlockNum] = dataInfo
									}
								}
							}
						}
					}
				}
			}
		}

		// Bloque indirecto triple (14)
		if inode.IBlock[14] > 0 {
			blockNum := inode.IBlock[14]

			if info, exists := blocks[blockNum]; exists {
				info.Type = "pointer"
				info.IndirectLevel = 3
				info.ReferencedBy = append(info.ReferencedBy, inodeNum)
				blocks[blockNum] = info

			}
		}
	}

	// 5. Procesar los directorios y leer sus entradas
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
	dotBuilder.WriteString("    label=\"Estructura de Archivos y Bloques\";\n")
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
		aTimeStr := inode.IAtime.Format("2006-01-02 15:04:05")

		blocksUsed := 0
		for j := 0; j < 15; j++ {
			if inode.IBlock[j] > 0 {
				blocksUsed++
			}
		}

		// Verificar bloques indirectos
		hasIndirect := false
		for j := 12; j < 15; j++ {
			if inode.IBlock[j] > 0 {
				hasIndirect = true
				break
			}
		}

		// Obtener información sobre punteros indirectos
		var indirectInfo string
		if hasIndirect {
			if inode.IBlock[12] > 0 {
				indirectInfo += fmt.Sprintf("\\nIndirecto simple: Bloque %d", inode.IBlock[12])
			}
			if inode.IBlock[13] > 0 {
				indirectInfo += fmt.Sprintf("\\nIndirecto doble: Bloque %d", inode.IBlock[13])
			}
			if inode.IBlock[14] > 0 {
				indirectInfo += fmt.Sprintf("\\nIndirecto triple: Bloque %d", inode.IBlock[14])
			}
		}

		// Etiqueta detallada para el inodo - incluir información de indirectos
		label := fmt.Sprintf("%s\\nInodo %d - %s\\nTamaño: %d bytes\\nUID: %d GID: %d\\nPermisos: %s\\nBloques usados: %d\\nCreado: %s\\nÚltima mod: %s\\nÚltimo acceso: %s%s",
			info.Name, i, typeStr, info.Size, inode.IUid, inode.IGid, permStr, blocksUsed, cTimeStr, mTimeStr, aTimeStr, indirectInfo)

		dotBuilder.WriteString(fmt.Sprintf("    node%d [label=\"%s\", shape=%s, fillcolor=\"%s\", color=\"black\"];\n",
			i, label, shape, fillcolor))
	}

	dotBuilder.WriteString("\n    // Nodos para bloques de datos\n")

	// Colores para diferentes tipos de bloques
	blockColors := map[string]string{
		"directory": "#C8E6C9", // Verde claro
		"file":      "#BBDEFB", // Azul claro
		"pointer":   "#FFE0B2", // Naranja claro
		"system":    "#E1BEE7", // Púrpura claro
	}

	// Crear nodos para bloques y conectarlos con sus inodos
	blocksCreated := make(map[int32]bool)

	// Primero crear los bloques directos
	for inodeNum, inode := range inodes {
		// Bloques directos (0-11)
		for i := 0; i < 12; i++ {
			blockNum := inode.IBlock[i]
			if blockNum <= 0 {
				continue
			}

			if _, exists := blocksCreated[blockNum]; !exists {
				blocksCreated[blockNum] = true

				blockInfo, hasInfo := blocks[blockNum]
				blockType := "unknown"
				if hasInfo {
					blockType = blockInfo.Type
				}

				fillColor := "#E0E0E0" // Gris por defecto
				if color, exists := blockColors[blockType]; exists {
					fillColor = color
				}

				// Personalización para bloques de directorio
				label := fmt.Sprintf("Bloque %d\\n(%s)", blockNum, blockType)

				// Si es un bloque de directorio, mostrar información adicional
				if blockType == "directory" {
					// Leer el bloque de directorio para mostrar entradas . y ..
					blockPos := blocksStart + int64(blockNum)*int64(superblock.SBlockSize)
					_, err := file.Seek(blockPos, 0)
					if err == nil {
						dirBlock, err := ReadDirectoryBlockFromDisc(file, int64(superblock.SBlockSize))
						if err == nil {
							// Buscar entradas . y ..
							var dotInodeNum, dotDotInodeNum int32

							for _, entry := range dirBlock.BContent {
								// Convertir el nombre
								name := ""
								for j := 0; j < len(entry.BName) && entry.BName[j] != 0; j++ {
									name += string(entry.BName[j])
								}

								if name == "." && entry.BInodo > 0 {
									dotInodeNum = entry.BInodo
								} else if name == ".." && entry.BInodo > 0 {
									dotDotInodeNum = entry.BInodo
								}
							}

							// Añadir a la etiqueta - usar comillas para escape
							if dotInodeNum > 0 {
								label += fmt.Sprintf("\\n\\\".\\\" → Inodo %d", dotInodeNum)
							}
							if dotDotInodeNum > 0 {
								label += fmt.Sprintf("\\n\\\"..\\\" → Inodo %d", dotDotInodeNum)
							}
						}
					}
				}

				// Crear nodo para el bloque con la etiqueta actualizada
				dotBuilder.WriteString(fmt.Sprintf("    block%d [label=\"%s\", shape=box, fillcolor=\"%s\"];\n",
					blockNum, label, fillColor))

				// Conectar inodo con bloque
				dotBuilder.WriteString(fmt.Sprintf("    node%d -> block%d [label=\"directo[%d]\", color=\"green\"];\n",
					inodeNum, blockNum, i))
			} else {
				// Si el bloque ya existe, solo crear la conexión
				dotBuilder.WriteString(fmt.Sprintf("    node%d -> block%d [label=\"directo[%d]\", color=\"green\"];\n",
					inodeNum, blockNum, i))
			}
		}
	}

	// Ahora crear bloques indirectos y sus conexiones
	for inodeNum, inode := range inodes {
		// Indirecto simple (12)
		if inode.IBlock[12] > 0 {
			blockNum := inode.IBlock[12]

			// Crear nodo para el bloque indirecto si no existe
			if _, exists := blocksCreated[blockNum]; !exists {
				blocksCreated[blockNum] = true
				dotBuilder.WriteString(fmt.Sprintf("    block%d [label=\"Bloque %d\\n(indirecto simple)\", shape=box, fillcolor=\"%s\"];\n",
					blockNum, blockNum, blockColors["pointer"]))
			}

			// Conectar inodo con bloque indirecto
			dotBuilder.WriteString(fmt.Sprintf("    node%d -> block%d [label=\"indirecto[0]\", color=\"orange\"];\n",
				inodeNum, blockNum))

			// Leer el bloque indirecto para obtener referencias
			blockPos := blocksStart + int64(blockNum)*int64(superblock.SBlockSize)
			_, err := file.Seek(blockPos, 0)
			if err == nil {
				var pointerBlock PointerBlock

				// Leer cada puntero
				for j := 0; j < POINTERS_PER_BLOCK; j++ {
					err := binary.Read(file, binary.LittleEndian, &pointerBlock.BPointers[j])
					if err != nil {
						break
					}
				}

				// Procesar punteros válidos
				for j := 0; j < POINTERS_PER_BLOCK; j++ {
					refBlockNum := pointerBlock.BPointers[j]
					if refBlockNum <= 0 || refBlockNum == POINTER_UNUSED_VALUE {
						continue
					}

					// Crear nodo para el bloque referenciado si no existe
					if _, exists := blocksCreated[refBlockNum]; !exists {
						blocksCreated[refBlockNum] = true

						// Determinar tipo de bloque
						blockType := "file"
						if blockInfo, exists := blocks[refBlockNum]; exists {
							blockType = blockInfo.Type
						}

						fillColor := blockColors["file"] // Por defecto asumimos que es de archivo
						if color, exists := blockColors[blockType]; exists {
							fillColor = color
						}

						dotBuilder.WriteString(fmt.Sprintf("    block%d [label=\"Bloque %d\\n(%s)\", shape=box, fillcolor=\"%s\"];\n",
							refBlockNum, refBlockNum, blockType, fillColor))
					}

					// Conectar bloque indirecto con bloque de datos
					dotBuilder.WriteString(fmt.Sprintf("    block%d -> block%d [label=\"[%d]\", color=\"orange\", style=\"dashed\"];\n",
						blockNum, refBlockNum, j))
				}
			}
		}

		// Indirecto doble (13)
		if inode.IBlock[13] > 0 {
			blockNum := inode.IBlock[13]

			// Crear nodo para el bloque indirecto doble si no existe
			if _, exists := blocksCreated[blockNum]; !exists {
				blocksCreated[blockNum] = true
				dotBuilder.WriteString(fmt.Sprintf("    block%d [label=\"Bloque %d\\n(indirecto doble)\", shape=box, fillcolor=\"%s\"];\n",
					blockNum, blockNum, blockColors["pointer"]))
			}

			// Conectar inodo con bloque indirecto doble
			dotBuilder.WriteString(fmt.Sprintf("    node%d -> block%d [label=\"indirecto[1]\", color=\"red\"];\n",
				inodeNum, blockNum))

			// Leer el bloque indirecto doble para obtener referencias a bloques indirectos simples
			blockPos := blocksStart + int64(blockNum)*int64(superblock.SBlockSize)
			_, err := file.Seek(blockPos, 0)
			if err == nil {
				var l1PointerBlock PointerBlock

				// Leer cada puntero nivel 1
				for j := 0; j < POINTERS_PER_BLOCK; j++ {
					err := binary.Read(file, binary.LittleEndian, &l1PointerBlock.BPointers[j])
					if err != nil {
						break
					}
				}

				// Procesar punteros nivel 1 válidos
				for j := 0; j < POINTERS_PER_BLOCK; j++ {
					l1BlockNum := l1PointerBlock.BPointers[j]
					if l1BlockNum <= 0 || l1BlockNum == POINTER_UNUSED_VALUE {
						continue
					}

					// Crear nodo para el bloque indirecto simple si no existe
					if _, exists := blocksCreated[l1BlockNum]; !exists {
						blocksCreated[l1BlockNum] = true
						dotBuilder.WriteString(fmt.Sprintf("    block%d [label=\"Bloque %d\\n(indirecto simple)\", shape=box, fillcolor=\"%s\"];\n",
							l1BlockNum, l1BlockNum, blockColors["pointer"]))
					}

					// Conectar indirecto doble con indirecto simple
					dotBuilder.WriteString(fmt.Sprintf("    block%d -> block%d [label=\"[%d]\", color=\"red\", style=\"dashed\"];\n",
						blockNum, l1BlockNum, j))

					// Leer el bloque indirecto simple para obtener referencias a bloques de datos
					l1BlockPos := blocksStart + int64(l1BlockNum)*int64(superblock.SBlockSize)
					_, err := file.Seek(l1BlockPos, 0)
					if err == nil {
						var l2PointerBlock PointerBlock

						// Leer cada puntero nivel 2
						for k := 0; k < POINTERS_PER_BLOCK; k++ {
							err := binary.Read(file, binary.LittleEndian, &l2PointerBlock.BPointers[k])
							if err != nil {
								break
							}
						}

						// Procesar punteros nivel 2 válidos
						for k := 0; k < POINTERS_PER_BLOCK; k++ {
							l2BlockNum := l2PointerBlock.BPointers[k]
							if l2BlockNum <= 0 || l2BlockNum == POINTER_UNUSED_VALUE {
								continue
							}

							// Crear nodo para el bloque de datos si no existe
							if _, exists := blocksCreated[l2BlockNum]; !exists {
								blocksCreated[l2BlockNum] = true

								// Determinar tipo de bloque
								blockType := "file"
								if blockInfo, exists := blocks[l2BlockNum]; exists {
									blockType = blockInfo.Type
								}

								fillColor := blockColors["file"] // Por defecto asumimos que es de archivo
								if color, exists := blockColors[blockType]; exists {
									fillColor = color
								}

								dotBuilder.WriteString(fmt.Sprintf("    block%d [label=\"Bloque %d\\n(%s)\", shape=box, fillcolor=\"%s\"];\n",
									l2BlockNum, l2BlockNum, blockType, fillColor))
							}

							// Conectar indirecto simple con bloque de datos
							dotBuilder.WriteString(fmt.Sprintf("    block%d -> block%d [label=\"[%d]\", color=\"orange\", style=\"dashed\"];\n",
								l1BlockNum, l2BlockNum, k))
						}
					}
				}
			}
		}

		// Indirecto triple (14)
		if inode.IBlock[14] > 0 {
			blockNum := inode.IBlock[14]

			// Crear nodo para el bloque indirecto triple si no existe
			if _, exists := blocksCreated[blockNum]; !exists {
				blocksCreated[blockNum] = true
				dotBuilder.WriteString(fmt.Sprintf("    block%d [label=\"Bloque %d\\n(indirecto triple)\", shape=box, fillcolor=\"%s\"];\n",
					blockNum, blockNum, blockColors["pointer"]))
			}

			// Conectar inodo con bloque indirecto triple
			dotBuilder.WriteString(fmt.Sprintf("    node%d -> block%d [label=\"indirecto[2]\", color=\"purple\"];\n",
				inodeNum, blockNum))

		}
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

			// Procesar el directorio para encontrar entradas especiales
			entries := processDirectoryEntries(i, inodes[i])

			// Sección de entradas especiales
			contentText += "\\n\\nEntradas especiales:"
			dotFound := false
			dotDotFound := false

			for _, entry := range entries {
				if entry.Name == "." {
					contentText += fmt.Sprintf("\\n• \\\".\\\" → Inodo %d (Este directorio)", entry.InodeNum)
					dotFound = true
				} else if entry.Name == ".." {
					contentText += fmt.Sprintf("\\n• \\\"..\\\" → Inodo %d (Directorio padre)", entry.InodeNum)
					dotDotFound = true
				}
			}

			if !dotFound {
				contentText += "\\n• \\\".\\\" → No encontrado"
			}
			if !dotDotFound {
				contentText += "\\n• \\\"..\\\" → No encontrado"
			}

			// Sección de entradas regulares
			if len(info.Children) > 0 {
				contentText += "\\n\\nOtras entradas:"
				for _, child := range info.Children {
					if child.Name != "." && child.Name != ".." {
						contentText += fmt.Sprintf("\\n- %s (inodo %d)", child.Name, child.InodeID)
					}
				}
			} else {
				contentText += "\\n\\n[No se encontraron otras entradas]"
			}

			contentLabel = fmt.Sprintf("%s\\n%s", contentTitle, contentText)
		} else if info.Type == 1 { // Archivo
			contentTitle := fmt.Sprintf("Contenido del archivo %s", info.Name)

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

	// Subgrafo para estadísticas de bloques
	dotBuilder.WriteString("  subgraph cluster_stats {\n")
	dotBuilder.WriteString("    label=\"Estadísticas de Bloques\";\n")
	dotBuilder.WriteString("    style=filled;\n")
	dotBuilder.WriteString("    fillcolor=\"#E1F5FE\";\n")
	dotBuilder.WriteString("    node [shape=box, style=filled];\n")

	// Agrupar bloques por tipo
	directoryBlocks := []int32{}
	fileBlocks := []int32{}
	pointerBlocks := []int32{}
	systemBlocks := []int32{} // Bloques usados por el sistema (superbloques, bitmaps, etc.)

	for blockNum, info := range blocks {
		if !info.InUse {
			continue
		}

		switch info.Type {
		case "directory":
			directoryBlocks = append(directoryBlocks, blockNum)
		case "file":
			fileBlocks = append(fileBlocks, blockNum)
		case "pointer":
			pointerBlocks = append(pointerBlocks, blockNum)
		case "unknown":
			if info.InUse {
				// Bloques en uso pero no asignados a archivos/directorios/punteros
				systemBlocks = append(systemBlocks, blockNum)
			}
		}
	}

	// Estadísticas de bloques
	freeBlocks := int(superblock.SFreeBlocksCount)
	totalBlocks := int(superblock.SBlocksCount)
	usedBlocks := totalBlocks - freeBlocks

	dotBuilder.WriteString("    blockStats [label=<")
	dotBuilder.WriteString("<table border='0' cellborder='1' cellspacing='0'>")
	dotBuilder.WriteString("<tr><td colspan='2' bgcolor='#D1C4E9'><b>Estadísticas de Bloques</b></td></tr>")
	dotBuilder.WriteString(fmt.Sprintf("<tr><td>Total</td><td>%d</td></tr>", totalBlocks))
	dotBuilder.WriteString(fmt.Sprintf("<tr><td>En uso</td><td>%d (%.1f%%)</td></tr>",
		usedBlocks, float64(usedBlocks)*100/float64(totalBlocks)))
	dotBuilder.WriteString(fmt.Sprintf("<tr><td>Libres</td><td>%d (%.1f%%)</td></tr>",
		freeBlocks, float64(freeBlocks)*100/float64(totalBlocks)))
	dotBuilder.WriteString(fmt.Sprintf("<tr><td>Directorio</td><td>%d</td></tr>", len(directoryBlocks)))
	dotBuilder.WriteString(fmt.Sprintf("<tr><td>Archivo</td><td>%d</td></tr>", len(fileBlocks)))
	dotBuilder.WriteString(fmt.Sprintf("<tr><td>Punteros</td><td>%d</td></tr>", len(pointerBlocks)))
	dotBuilder.WriteString(fmt.Sprintf("<tr><td>Sistema</td><td>%d</td></tr>", len(systemBlocks)))
	dotBuilder.WriteString("</table>>];\n")

	dotBuilder.WriteString("  }\n")

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
