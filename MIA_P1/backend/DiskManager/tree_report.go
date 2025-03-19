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

	for i, inode := range inodes {
		// Solo procesar inodos de tipo directorio (0)
		if inode.IType != 0 {
			continue
		}

		for j := 0; j < 12; j++ { // Recorrer bloques directos
			blockNum := inode.IBlock[j]
			if blockNum <= 0 {
				continue
			}

			// Leer el bloque de directorio
			blockPos := blocksStart + int64(blockNum)*int64(superblock.SBlockSize)
			_, err = file.Seek(blockPos, 0)
			if err != nil {
				continue
			}

			// Leer el bloque completo
			blockData := make([]byte, superblock.SBlockSize)
			_, err = file.Read(blockData)
			if err != nil {
				continue
			}

			reader := bytes.NewReader(blockData)

			// Leer las entradas del directorio
			for k := 0; k < B_CONTENT_COUNT; k++ {
				// Leer el nombre (array fijo)
				nameBytes := make([]byte, B_NAME_SIZE)
				_, err := reader.Read(nameBytes)
				if err != nil {
					break
				}

				// Leer el número de inodo
				var inodeNum int32
				err = binary.Read(reader, binary.LittleEndian, &inodeNum)
				if err != nil {
					break
				}

				// Procesar solo entradas válidas
				name := ""
				for l := 0; l < B_NAME_SIZE && nameBytes[l] != 0; l++ {
					name += string(nameBytes[l])
				}

				if name != "" && inodeNum > 0 {
					fmt.Printf("Encontrada entrada en directorio %d: %s -> %d\n", i, name, inodeNum)

					// Registrar la relación
					if relationships[i] == nil {
						relationships[i] = make(map[int]string)
					}
					relationships[i][int(inodeNum)] = name

					// Actualizar información del inodo hijo
					if _, exists := inodeInfo[int(inodeNum)]; exists {
						temp := inodeInfo[int(inodeNum)]
						temp.Name = name
						inodeInfo[int(inodeNum)] = temp
					}

					// Actualizar los hijos del directorio
					temp := inodeInfo[i]
					temp.Children = append(temp.Children, struct {
						Name    string
						InodeID int
					}{
						Name:    name,
						InodeID: int(inodeNum),
					})
					inodeInfo[i] = temp
				}
			}
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
			fmt.Printf("ADVERTENCIA: No se encontró un bloque válido para el archivo %d (%s)\n",
				i, inodeInfo[i].Name)
			continue
		}

		// Calcular la posición del bloque
		blockPos := blocksStart + int64(blockNum)*int64(superblock.SBlockSize)
		fmt.Printf("Leyendo contenido desde la posición %d (bloque %d)\n", blockPos, blockNum)

		_, err = file.Seek(blockPos, 0)
		if err != nil {
			fmt.Printf("Error al posicionarse en el bloque %d: %s\n", blockNum, err)
			continue
		}

		// Calcular el tamaño real a leer
		contentSize := 0
		if int(inode.ISize) < int(superblock.SBlockSize) {
			contentSize = int(inode.ISize)
		} else {
			contentSize = int(superblock.SBlockSize)
		}

		fmt.Printf("Preparando para leer %d bytes (tamaño del archivo: %d, tamaño del bloque: %d)\n",
			contentSize, inode.ISize, superblock.SBlockSize)

		// Leer el contenido
		contentData := make([]byte, contentSize)
		bytesRead, err := file.Read(contentData)
		if err != nil {
			fmt.Printf("Error al leer contenido: %s\n", err)
			continue
		}

		fmt.Printf("Bytes leídos: %d\n", bytesRead)

		// Ver los primeros bytes en hex para diagnóstico
		fmt.Printf("Primeros bytes (hex): ")
		for k := 0; k < min(bytesRead, 16); k++ {
			fmt.Printf("%02X ", contentData[k])
		}
		fmt.Println()

		// Convertir a string
		content := string(contentData)
		// Limitar para logs
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

// contains verifica si un slice de enteros contiene un valor
func contains(slice []int, value int) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
