package DiskManager

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// BlockReporter genera un reporte gráfico de los bloques utilizados
func BlockReporter(id, path string) (bool, string) {
	// 1. Encontrar la partición montada
	mountedPartition, err := FindMountedPartitionById(id)
	if err != nil {
		return false, fmt.Sprintf("Error: %s", err)
	}
	fmt.Printf("Usando partición: %s en disco: %s\n", mountedPartition.ID, mountedPartition.DiskPath)

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
	fmt.Printf("Superbloque leído: inodos=%d, bloques=%d\n", superblock.SInodesCount, superblock.SBlocksCount)
	fmt.Printf("Inicio bitmap inodos: %d, Inicio bitmap bloques: %d\n", superblock.SBmInodeStart, superblock.SBmBlockStart)
	fmt.Printf("Inicio tabla inodos: %d, Inicio bloques: %d\n", superblock.SInodeStart, superblock.SBlockStart)
	fmt.Printf("Tamaño inodo: %d, Tamaño bloque: %d\n", superblock.SInodeSize, superblock.SBlockSize)

	// 5. Preparar DOT
	var dot strings.Builder
	dot.WriteString("digraph FileSystem {\n")
	dot.WriteString("  rankdir=LR;\n")
	dot.WriteString("  node [shape=record, fontname=\"Arial\"];\n")
	dot.WriteString("  edge [color=\"#333333\", penwidth=1.2];\n")
	dot.WriteString("  bgcolor=\"white\";\n")
	dot.WriteString("  labelloc=\"t\";\n")
	dot.WriteString("  label=\"Reporte de Bloques - EXT2\";\n\n")

	// 6. Definir posiciones importantes
	bmInodePos := startByte + int64(superblock.SBmInodeStart)
	bmBlockPos := startByte + int64(superblock.SBmBlockStart)
	blocksStartPos := startByte + int64(superblock.SBlockStart)

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

	// 9. Contar y mostrar inodos y bloques en uso
	inodeCount := 0
	for i := 0; i < int(superblock.SInodesCount); i++ {
		bytePos := i / 8
		bitPos := i % 8
		if bytePos < len(inodeBitmap) && (inodeBitmap[bytePos]&(1<<bitPos)) != 0 {
			inodeCount++
		}
	}
	fmt.Printf("Inodos en uso según bitmap: %d\n", inodeCount)

	blockCount := 0
	for i := 0; i < int(superblock.SBlocksCount); i++ {
		bytePos := i / 8
		bitPos := i % 8
		if bytePos < len(blockBitmap) && (blockBitmap[bytePos]&(1<<bitPos)) != 0 {
			blockCount++
		}
	}
	fmt.Printf("Bloques en uso según bitmap: %d\n", blockCount)

	// 10. Mapa para seguimiento de bloques
	blockTypes := make(map[int]string)    // Tipo de bloque: "directory", "file", "pointer"
	blockContents := make(map[int]string) // Contenido formateado del bloque

	// 11. Inspeccionar todos los bloques marcados como utilizados
	for i := 0; i < int(superblock.SBlocksCount); i++ {
		bytePos := i / 8
		bitPos := i % 8

		// Verificar si este bloque está en uso según el bitmap
		if bytePos >= len(blockBitmap) || (blockBitmap[bytePos]&(1<<bitPos)) == 0 {
			continue
		}

		// Leer el bloque
		blockPos := blocksStartPos + int64(i)*int64(superblock.SBlockSize)
		_, err = file.Seek(blockPos, 0)
		if err != nil {
			fmt.Printf("Error al posicionarse en bloque %d: %s\n", i, err)
			continue
		}

		blockData := make([]byte, superblock.SBlockSize)
		_, err = file.ReadAt(blockData, blockPos)
		if err != nil {
			fmt.Printf("Error al leer bloque %d: %s\n", i, err)
			continue
		}

		// Verificar si el bloque tiene datos
		isEmpty := true
		for _, b := range blockData {
			if b != 0 {
				isEmpty = false
				break
			}
		}

		if isEmpty {
			fmt.Printf("Bloque %d: Marcado como usado pero está vacío\n", i)
			continue
		}

		fmt.Printf("Bloque %d: En uso y contiene datos\n", i)

		// Determinar el tipo de bloque examinando su contenido

		// Intentar como directorio
		isDir := false
		dirBlock := DirectoryBlock{}
		err = binary.Read(bytes.NewReader(blockData), binary.LittleEndian, &dirBlock)
		if err == nil {
			validEntries := 0
			for j := 0; j < B_CONTENT_COUNT; j++ {
				if dirBlock.BContent[j].BInodo >= 0 && dirBlock.BContent[j].BInodo < int32(superblock.SInodesCount) {
					name := ""
					for k := 0; k < B_NAME_SIZE && dirBlock.BContent[j].BName[k] != 0; k++ {
						name += string(dirBlock.BContent[j].BName[k])
					}
					if name != "" {
						validEntries++
						fmt.Printf("  Bloque %d: Parece directorio, entrada: '%s' -> inodo %d\n",
							i, name, dirBlock.BContent[j].BInodo)
					}
				}
			}

			if validEntries > 0 {
				isDir = true
				blockTypes[i] = "directory"

				// Construir contenido formateado
				var content strings.Builder
				content.WriteString(fmt.Sprintf("Bloque %d - Directorio | { Nombre | Inodo }", i))

				for j := 0; j < B_CONTENT_COUNT; j++ {
					if dirBlock.BContent[j].BInodo >= 0 && dirBlock.BContent[j].BInodo < int32(superblock.SInodesCount) {
						name := ""
						for k := 0; k < B_NAME_SIZE && dirBlock.BContent[j].BName[k] != 0; k++ {
							if dirBlock.BContent[j].BName[k] >= 32 && dirBlock.BContent[j].BName[k] <= 126 {
								name += string(dirBlock.BContent[j].BName[k])
							}
						}
						if name != "" {
							name = strings.ReplaceAll(name, "|", "\\|")
							name = strings.ReplaceAll(name, "{", "\\{")
							name = strings.ReplaceAll(name, "}", "\\}")
							content.WriteString(fmt.Sprintf(" | { %s | %d }", name, dirBlock.BContent[j].BInodo))
						}
					}
				}

				blockContents[i] = content.String()
			}
		}

		// Si no es directorio, intentar como punteros
		if !isDir {
			ptrBlock := PointerBlock{}
			err = binary.Read(bytes.NewReader(blockData), binary.LittleEndian, &ptrBlock)
			if err == nil {
				validPtrs := 0
				for j := 0; j < POINTERS_PER_BLOCK; j++ {
					if ptrBlock.BPointers[j] > 0 && ptrBlock.BPointers[j] < int32(superblock.SBlocksCount) {
						validPtrs++
						fmt.Printf("  Bloque %d: Parece apuntadores, ptr[%d] -> bloque %d\n",
							i, j, ptrBlock.BPointers[j])
					}
				}

				if validPtrs > 0 {
					blockTypes[i] = "pointer"

					// Construir contenido formateado
					var content strings.Builder
					content.WriteString(fmt.Sprintf("Bloque %d - Apuntadores", i))

					// Agrupar punteros en filas
					var row1, row2 []string
					for j := 0; j < POINTERS_PER_BLOCK/2; j++ {
						if ptrBlock.BPointers[j] > 0 && ptrBlock.BPointers[j] < int32(superblock.SBlocksCount) {
							row1 = append(row1, fmt.Sprintf("%d", ptrBlock.BPointers[j]))
						}
					}

					for j := POINTERS_PER_BLOCK / 2; j < POINTERS_PER_BLOCK; j++ {
						if ptrBlock.BPointers[j] > 0 && ptrBlock.BPointers[j] < int32(superblock.SBlocksCount) {
							row2 = append(row2, fmt.Sprintf("%d", ptrBlock.BPointers[j]))
						}
					}

					if len(row1) > 0 {
						content.WriteString(" | { " + strings.Join(row1, " | ") + " }")
					}

					if len(row2) > 0 {
						content.WriteString(" | { " + strings.Join(row2, " | ") + " }")
					}

					blockContents[i] = content.String()
				}
			}
		}

		// Si no es directorio ni punteros, entonces es archivo
		if !isDir && blockTypes[i] != "pointer" {
			blockTypes[i] = "file"

			// Formar vista previa del contenido
			preview := ""
			for j := 0; j < min(64, len(blockData)); j++ {
				if blockData[j] >= 32 && blockData[j] <= 126 {
					preview += string(blockData[j])
				} else {
					preview += "."
				}
			}

			if len(blockData) > 64 {
				preview += "..."
			}

			// Formatear en líneas
			var formattedContent strings.Builder
			formattedContent.WriteString(fmt.Sprintf("Bloque %d - Archivo | ", i))

			for j := 0; j < len(preview); j += 30 {
				end := min(j+30, len(preview))
				formattedContent.WriteString(preview[j:end])
				if end < len(preview) {
					formattedContent.WriteString("\\n")
				}
			}

			blockContents[i] = formattedContent.String()

			fmt.Printf("  Bloque %d: Tratado como archivo, primeros bytes: %s\n", i, preview[:min(20, len(preview))])
		}
	}

	// 12. Crear nodos para cada bloque
	for blockNum, blockType := range blockTypes {
		content := blockContents[blockNum]
		if content == "" {
			continue
		}

		switch blockType {
		case "directory":
			dot.WriteString(fmt.Sprintf("  block%d [label=\"%s\", style=filled, fillcolor=\"#E8F5E9\", color=\"#4CAF50\"];\n",
				blockNum, content))

		case "pointer":
			dot.WriteString(fmt.Sprintf("  block%d [label=\"%s\", style=filled, fillcolor=\"#FFF3E0\", color=\"#FF9800\"];\n",
				blockNum, content))

		case "file":
			dot.WriteString(fmt.Sprintf("  block%d [label=\"%s\", style=filled, fillcolor=\"#E3F2FD\", color=\"#2196F3\"];\n",
				blockNum, content))
		}
	}

	// 13. Crear conexiones entre bloques
	// Para bloques de punteros, conectar con los bloques a los que apuntan
	for blockNum, blockType := range blockTypes {
		if blockType == "pointer" {
			// Leer el bloque nuevamente
			blockPos := blocksStartPos + int64(blockNum)*int64(superblock.SBlockSize)
			blockData := make([]byte, superblock.SBlockSize)
			_, err = file.ReadAt(blockData, blockPos)
			if err != nil {
				continue
			}

			ptrBlock := PointerBlock{}
			err = binary.Read(bytes.NewReader(blockData), binary.LittleEndian, &ptrBlock)
			if err != nil {
				continue
			}

			for i := 0; i < POINTERS_PER_BLOCK; i++ {
				targetBlock := ptrBlock.BPointers[i]
				if targetBlock > 0 && targetBlock < int32(superblock.SBlocksCount) {
					// Verificar si el bloque objetivo existe en nuestro mapa
					if _, exists := blockTypes[int(targetBlock)]; exists {
						dot.WriteString(fmt.Sprintf("  block%d -> block%d [label=\"ptr[%d]\"];\n",
							blockNum, targetBlock, i))
					}
				}
			}
		}
	}

	// 14. Para bloques de directorio, conectar con bloques de contenido si es posible
	// Esta es una aproximación simple, idealmente necesitaríamos analizar los inodos
	for blockNum, blockType := range blockTypes {
		if blockType == "directory" {
			// Por ejemplo, conectar el directorio con el primer archivo no directorio
			for targetBlock, targetType := range blockTypes {
				if targetType == "file" && targetBlock != blockNum {
					dot.WriteString(fmt.Sprintf("  block%d -> block%d [label=\"contiene\"];\n",
						blockNum, targetBlock))
					break
				}
			}
		}
	}

	// 15. Agregar nodo de información (como un nodo normal, no HTML)
	dot.WriteString(fmt.Sprintf("  info [shape=record, label=\"{Información|{Partición|%s}|{Inodos Total|%d}|{Bloques Total|%d}|{Inodos Usados|%d}|{Bloques Usados|%d}|{Bloques Procesados|%d}}\", style=filled, fillcolor=\"#E1BEE7\"];\n",
		mountedPartition.ID, superblock.SInodesCount, superblock.SBlocksCount, inodeCount, blockCount, len(blockTypes)))

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
	cmd := exec.Command("dot", "-Tjpg", dotFile, "-o", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("Error al ejecutar Graphviz: %v\nStdout: %s\nStderr: %s\nArchivo DOT guardado en: %s",
			err, stdout.String(), stderr.String(), dotFile)
	}

	return true, fmt.Sprintf("Reporte generado exitosamente en: %s\nConsulta la terminal para ver información de depuración.", path)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
