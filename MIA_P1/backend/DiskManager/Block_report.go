package DiskManager

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// BlockReporter genera un reporte gráfico mejorado de los bloques utilizados
// BlockReporter genera un reporte gráfico simplificado de los bloques utilizados
func BlockReporter(id, path string) (bool, string) {
	// 1. Encontrar la partición montada
	mountedPartition, err := FindMountedPartitionById(id)
	if err != nil {
		return false, fmt.Sprintf("Error: %s", err)
	}
	fmt.Printf("Generando reporte para partición: %s en disco: %s\n", mountedPartition.ID, mountedPartition.DiskPath)

	// 2. Abrir el disco
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return false, fmt.Sprintf("Error al abrir el disco: %s", err)
	}
	defer file.Close()

	// 3. Obtener detalles de la partición
	startByte, size, err := GetPartitionDetails(file, mountedPartition)
	if err != nil {
		return false, fmt.Sprintf("Error al obtener detalles de la partición: %s", err)
	}
	fmt.Printf("Partición inicia en byte: %d, tamaño: %d bytes\n", startByte, size)

	// 4. Leer el superbloque
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en el superbloque: %s", err)
	}

	superblock, err := ReadSuperBlockFromDisc(file)
	if err != nil {
		return false, fmt.Sprintf("Error al leer el superbloque: %s", err)
	}
	fmt.Printf("Leyendo superbloque... OK\n")
	fmt.Printf("- Inodos total: %d, Bloques total: %d\n", superblock.SInodesCount, superblock.SBlocksCount)
	fmt.Printf("- Tamaño de bloque: %d bytes\n", superblock.SBlockSize)

	// 5. Preparar DOT con diseño simplificado
	var dot strings.Builder
	dot.WriteString("digraph FileSystem {\n")
	dot.WriteString("  graph [rankdir=LR, fontname=\"Arial\", fontsize=12];\n")
	dot.WriteString("  node [fontname=\"Arial\", fontsize=11, shape=box, style=filled];\n")
	dot.WriteString("  edge [fontname=\"Arial\", fontsize=10];\n")
	dot.WriteString(fmt.Sprintf("  label=\"Reporte de Bloques EXT2 - %s\";\n", mountedPartition.ID))

	// 6. Definir posiciones importantes
	bmInodePos := startByte + int64(superblock.SBmInodeStart)
	bmBlockPos := startByte + int64(superblock.SBmBlockStart)
	blocksStartPos := startByte + int64(superblock.SBlockStart)
	inodesStartPos := startByte + int64(superblock.SInodeStart)

	// 7. Leer bitmap de inodos
	_, err = file.Seek(bmInodePos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en el bitmap de inodos: %s", err)
	}

	inodeBitmap := make([]byte, superblock.SInodesCount/8+1)
	_, err = file.Read(inodeBitmap)
	if err != nil {
		return false, fmt.Sprintf("Error al leer el bitmap de inodos: %s", err)
	}

	// 8. Leer bitmap de bloques
	_, err = file.Seek(bmBlockPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en el bitmap de bloques: %s", err)
	}

	blockBitmap := make([]byte, superblock.SBlocksCount/8+1)
	_, err = file.Read(blockBitmap)
	if err != nil {
		return false, fmt.Sprintf("Error al leer el bitmap de bloques: %s", err)
	}

	// 9. Calcular bloques e inodos utilizados
	inodeCount := 0
	for i := 0; i < int(superblock.SInodesCount); i++ {
		bytePos := i / 8
		bitPos := i % 8
		if bytePos < len(inodeBitmap) && (inodeBitmap[bytePos]&(1<<bitPos)) != 0 {
			inodeCount++
		}
	}

	blockCount := 0
	for i := 0; i < int(superblock.SBlocksCount); i++ {
		bytePos := i / 8
		bitPos := i % 8
		if bytePos >= len(blockBitmap) || (blockBitmap[bytePos]&(1<<bitPos)) == 0 {
			continue
		}
		blockCount++
	}

	// 10. Rastrear relaciones entre bloques
	blockTypes := make(map[int]string)     // Tipo: directorio, archivo, puntero
	blockDirections := make(map[int][]int) // Mapa de conexiones bloque -> otros bloques
	inodeToBlocks := make(map[int][]int)   // Inodo -> bloques asignados
	blockContents := make(map[int]string)  // Contenido descriptivo para mostrar
	dirStats := 0
	fileStats := 0
	ptrStats := 0
	doublePtrStats := 0

	// 11. Recorrer inodos para encontrar bloques
	fmt.Printf("Analizando estructura de archivos...\n")
	for i := 0; i < int(superblock.SInodesCount); i++ {
		bytePos := i / 8
		bitPos := i % 8

		// Verificar si este inodo está en uso según el bitmap
		if bytePos >= len(inodeBitmap) || (inodeBitmap[bytePos]&(1<<bitPos)) == 0 {
			continue
		}

		// Leer este inodo
		inodePos := inodesStartPos + int64(i)*int64(superblock.SInodeSize)
		_, err = file.Seek(inodePos, 0)
		if err != nil {
			continue
		}

		inode, err := readInodeFromDisc(file)
		if err != nil {
			continue
		}

		// Procesar bloques directos
		for j := 0; j < 12; j++ {
			if inode.IBlock[j] > 0 && inode.IBlock[j] < int32(superblock.SBlocksCount) {
				blockNum := int(inode.IBlock[j])
				inodeToBlocks[i] = append(inodeToBlocks[i], blockNum)
			}
		}

		// Procesar bloque indirecto simple
		if inode.IBlock[12] > 0 && inode.IBlock[12] < int32(superblock.SBlocksCount) {
			blockNum := int(inode.IBlock[12])
			inodeToBlocks[i] = append(inodeToBlocks[i], blockNum)

			// Leer contenido del bloque indirecto
			blockPos := blocksStartPos + int64(blockNum)*int64(superblock.SBlockSize)
			blockData := make([]byte, superblock.SBlockSize)
			_, err = file.ReadAt(blockData, blockPos)
			if err == nil {
				ptrBlock := PointerBlock{}
				err = binary.Read(bytes.NewReader(blockData), binary.LittleEndian, &ptrBlock)
				if err == nil {
					for k := 0; k < POINTERS_PER_BLOCK; k++ {
						if ptrBlock.BPointers[k] > 0 && ptrBlock.BPointers[k] < int32(superblock.SBlocksCount) {
							// Agregar dirección del bloque indirecto al bloque de datos
							blockDirections[blockNum] = append(blockDirections[blockNum], int(ptrBlock.BPointers[k]))
						}
					}
				}
			}
		}

		// Procesar bloque indirecto doble
		if inode.IBlock[13] > 0 && inode.IBlock[13] < int32(superblock.SBlocksCount) {
			blockNum := int(inode.IBlock[13])
			inodeToBlocks[i] = append(inodeToBlocks[i], blockNum)

			// Leer contenido del bloque indirecto doble
			blockPos := blocksStartPos + int64(blockNum)*int64(superblock.SBlockSize)
			blockData := make([]byte, superblock.SBlockSize)
			_, err = file.ReadAt(blockData, blockPos)
			if err == nil {
				ptrBlock := PointerBlock{}
				err = binary.Read(bytes.NewReader(blockData), binary.LittleEndian, &ptrBlock)
				if err == nil {
					for k := 0; k < POINTERS_PER_BLOCK; k++ {
						if ptrBlock.BPointers[k] > 0 && ptrBlock.BPointers[k] < int32(superblock.SBlocksCount) {
							indirectBlockNum := int(ptrBlock.BPointers[k])

							// Agregar dirección del bloque indirecto doble al bloque indirecto
							blockDirections[blockNum] = append(blockDirections[blockNum], indirectBlockNum)

							// Leer bloque indirecto
							indirectBlockPos := blocksStartPos + int64(indirectBlockNum)*int64(superblock.SBlockSize)
							indirectData := make([]byte, superblock.SBlockSize)
							_, err = file.ReadAt(indirectData, indirectBlockPos)
							if err == nil {
								indirectPtrBlock := PointerBlock{}
								err = binary.Read(bytes.NewReader(indirectData), binary.LittleEndian, &indirectPtrBlock)
								if err == nil {
									for l := 0; l < POINTERS_PER_BLOCK; l++ {
										if indirectPtrBlock.BPointers[l] > 0 && indirectPtrBlock.BPointers[l] < int32(superblock.SBlocksCount) {
											// Agregar dirección del bloque indirecto al bloque de datos
											blockDirections[indirectBlockNum] = append(blockDirections[indirectBlockNum], int(indirectPtrBlock.BPointers[l]))
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// 12. Analizar bloques en uso
	fmt.Printf("Analizando contenido de los bloques...\n")
	for i := 0; i < int(superblock.SBlocksCount); i++ {
		bytePos := i / 8
		bitPos := i % 8

		// Verificar si este bloque está en uso según el bitmap
		if bytePos >= len(blockBitmap) || (blockBitmap[bytePos]&(1<<bitPos)) == 0 {
			continue
		}

		// Leer el bloque
		blockPos := blocksStartPos + int64(i)*int64(superblock.SBlockSize)
		blockData := make([]byte, superblock.SBlockSize)
		_, err = file.ReadAt(blockData, blockPos)
		if err != nil {
			continue
		}

		// Intentar interpretar como directorio
		isDirectory := false
		dirBlock := DirectoryBlock{}
		err = binary.Read(bytes.NewReader(blockData), binary.LittleEndian, &dirBlock)
		if err == nil {
			// Contar entradas válidas
			validEntries := 0
			var entries []string
			for j := 0; j < B_CONTENT_COUNT; j++ {
				if dirBlock.BContent[j].BInodo > 0 && dirBlock.BContent[j].BInodo < int32(superblock.SInodesCount) {
					name := ""
					for k := 0; k < B_NAME_SIZE && dirBlock.BContent[j].BName[k] != 0; k++ {
						if dirBlock.BContent[j].BName[k] >= 32 && dirBlock.BContent[j].BName[k] <= 126 {
							name += string(dirBlock.BContent[j].BName[k])
						}
					}
					if name != "" {
						validEntries++
						entries = append(entries, name)
					}
				}
			}

			if validEntries > 0 {
				isDirectory = true
				blockTypes[i] = "directory"
				dirStats++

				// Formato simple para contenido de directorio
				content := fmt.Sprintf("Bloque %d: Directorio\\n", i)
				for j, entry := range entries {
					if j < 5 { // Mostrar solo primeras 5 entradas
						content += fmt.Sprintf("%s ", escapeString(entry))
						if j < len(entries)-1 && j < 4 {
							content += "| "
						}
					}
				}
				if len(entries) > 5 {
					content += fmt.Sprintf("\\n+ %d entradas más", len(entries)-5)
				}
				blockContents[i] = content
			}
		}

		// Si no es directorio, verificar si es bloque de punteros
		if !isDirectory {
			ptrBlock := PointerBlock{}
			err = binary.Read(bytes.NewReader(blockData), binary.LittleEndian, &ptrBlock)
			if err == nil {
				validPtrs := 0
				var ptrs []int32

				// Contar punteros válidos
				for j := 0; j < POINTERS_PER_BLOCK; j++ {
					if ptrBlock.BPointers[j] >= 0 && ptrBlock.BPointers[j] < int32(superblock.SBlocksCount) {
						validPtrs++
						ptrs = append(ptrs, ptrBlock.BPointers[j])
					}
				}

				// Si hay punteros y está en nuestro mapa de direcciones, es un bloque de punteros
				if validPtrs > 0 && len(blockDirections[i]) > 0 {
					// Determinar si es indirecto simple o doble basado en su estructura
					isDouble := false
					for _, target := range blockDirections[i] {
						if len(blockDirections[target]) > 0 {
							isDouble = true
							break
						}
					}

					if isDouble {
						blockTypes[i] = "double_pointer"
						doublePtrStats++
						content := fmt.Sprintf("Bloque %d: Indirecto Doble\\n", i)

						// Mostrar punteros como lista compacta
						ptrList := ""
						for j, ptr := range ptrs {
							if j < 8 { // Mostrar solo primeros punteros
								if ptr >= 0 {
									ptrList += fmt.Sprintf("%d", ptr)
								} else {
									ptrList += "N"
								}

								if j < len(ptrs)-1 && j < 7 {
									ptrList += ","
								}
							}
						}

						if len(ptrs) > 8 {
							ptrList += "..."
						}

						content += ptrList
						blockContents[i] = content
					} else {
						blockTypes[i] = "pointer"
						ptrStats++
						content := fmt.Sprintf("Bloque %d: Indirecto\\n", i)

						// Mostrar punteros como lista compacta
						ptrList := ""
						for j, ptr := range ptrs {
							if j < 8 { // Mostrar solo primeros punteros
								if ptr >= 0 {
									ptrList += fmt.Sprintf("%d", ptr)
								} else {
									ptrList += "N"
								}

								if j < len(ptrs)-1 && j < 7 {
									ptrList += ","
								}
							}
						}

						if len(ptrs) > 8 {
							ptrList += "..."
						}

						content += ptrList
						blockContents[i] = content
					}
				}
			}
		}

		// Si no es directorio ni puntero, es archivo de datos
		if _, exists := blockTypes[i]; !exists {
			blockTypes[i] = "file"
			fileStats++

			// Analizar contenido del bloque
			textCount := 0
			preview := ""
			for j := 0; j < min(50, len(blockData)); j++ {
				if blockData[j] >= 32 && blockData[j] <= 126 {
					preview += string(blockData[j])
					textCount++
				} else if blockData[j] == 0 {
					preview += " "
				} else {
					preview += "."
				}
			}

			// Determinar si es texto o binario
			isText := float64(textCount)/float64(min(50, len(blockData))) > 0.7
			content := fmt.Sprintf("Bloque %d: Datos\\n", i)

			if isText && len(preview) > 0 {
				content += escapeString(preview)
				if len(preview) > 50 {
					content += "..."
				}
			} else {
				content += fmt.Sprintf("Datos Binarios (%d bytes)", len(blockData))
			}

			blockContents[i] = content
		}
	}

	// 13. Agregar nodos al gráfico
	fmt.Printf("Generando reporte gráfico...\n")

	// Agregar nodos de directorios
	for blockNum, blockType := range blockTypes {
		color := ""
		switch blockType {
		case "directory":
			color = "#4CAF50"
		case "file":
			color = "#2196F3"
		case "pointer":
			color = "#FF9800"
		case "double_pointer":
			color = "#F57C00"
		default:
			color = "#9E9E9E"
		}

		// Contenido del nodo
		content := blockContents[blockNum]
		if content == "" {
			content = fmt.Sprintf("Bloque %d", blockNum)
		}

		dot.WriteString(fmt.Sprintf("  block%d [label=\"%s\", fillcolor=\"%s\"];\n",
			blockNum, content, color))
	}

	// 14. Agregar conexiones
	for srcBlock, targetBlocks := range blockDirections {
		for _, targetBlock := range targetBlocks {
			dot.WriteString(fmt.Sprintf("  block%d -> block%d;\n", srcBlock, targetBlock))
		}
	}

	// 15. Agregar estadísticas
	dot.WriteString(fmt.Sprintf("  stats [label=\"Estadísticas\\n"+
		"Inodos Total: %d\\n"+
		"Inodos Usados: %d\\n"+
		"Bloques Total: %d\\n"+
		"Bloques Usados: %d\\n"+
		"Directorios: %d\\n"+
		"Archivos: %d\\n"+
		"Bloques Indirectos: %d\\n"+
		"Bloques Indirectos Dobles: %d\", shape=note, fillcolor=\"#E1BEE7\"];\n",
		superblock.SInodesCount, inodeCount,
		superblock.SBlocksCount, blockCount,
		dirStats, fileStats, ptrStats, doublePtrStats))

	// 16. Cerrar el grafo
	dot.WriteString("}\n")

	// 17. Guardar DOT
	dotFile := path + ".dot"
	err = os.WriteFile(dotFile, []byte(dot.String()), 0644)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir archivo DOT: %s", err)
	}
	fmt.Printf("Archivo DOT guardado en: %s\n", dotFile)

	// 18. Generar imagen
	fmt.Printf("Generando imagen con Graphviz...\n")
	cmd := exec.Command("dot", "-Tpng", dotFile, "-o", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Intentar con SVG como alternativa
		cmdSvg := exec.Command("dot", "-Tsvg", dotFile, "-o", path+".svg")
		if svgErr := cmdSvg.Run(); svgErr == nil {
			return true, fmt.Sprintf("Reporte generado en formato SVG: %s.svg\n\nEstadísticas:\n- Bloques analizados: %d\n- Directorios: %d\n- Archivos: %d\n- Indirectos: %d\n- Indirectos dobles: %d",
				path, len(blockTypes), dirStats, fileStats, ptrStats, doublePtrStats)
		}

		return false, fmt.Sprintf("Error al ejecutar Graphviz: %v\nStdout: %s\nStderr: %s\nArchivo DOT guardado en: %s",
			err, stdout.String(), stderr.String(), dotFile)
	}

	fmt.Printf("Imagen generada exitosamente en: %s\n", path)
	return true, fmt.Sprintf("Reporte generado exitosamente en: %s\n\nEstadísticas:\n- Bloques analizados: %d\n- Directorios: %d\n- Archivos: %d\n- Indirectos: %d\n- Indirectos dobles: %d",
		path, len(blockTypes), dirStats, fileStats, ptrStats, doublePtrStats)
}

// Función auxiliar para escapar caracteres especiales en strings para DOT
func escapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
