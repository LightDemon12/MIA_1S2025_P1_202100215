package DiskManager

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LSReporter genera un reporte de archivos y carpetas para un directorio específico
func LSReporter(id string, reportPath string, dirPath string) (bool, string) {
	fmt.Printf("Generando reporte LS para directorio '%s'\n", dirPath)

	// 1. Obtener información de la partición montada
	mountedPartition, err := FindMountedPartitionById(id)
	if err != nil {
		return false, fmt.Sprintf("Error: %v", err)
	}
	fmt.Printf("Partición encontrada: %s en %s\n", mountedPartition.ID, mountedPartition.DiskPath)

	// 2. Abrir el archivo del disco
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return false, fmt.Sprintf("Error al abrir disco: %v", err)
	}
	defer file.Close()

	// 3. Obtener la posición de inicio de la partición
	startByte, _, err := GetPartitionDetails(file, mountedPartition)
	if err != nil {
		return false, fmt.Sprintf("Error obteniendo detalles de partición: %v", err)
	}
	fmt.Printf("Inicio de partición: %d bytes\n", startByte)

	// 4. Leer el superbloque
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, fmt.Sprintf("Error posicionándose en la partición: %v", err)
	}

	superblock, err := ReadSuperBlockFromDisc(file)
	if err != nil {
		return false, fmt.Sprintf("Error leyendo el superbloque: %v", err)
	}

	// 5. Verificar que la ruta existe y es un directorio
	exists, pathType, err := ValidateEXT2Path(id, dirPath)
	if err != nil {
		return false, fmt.Sprintf("Error validando ruta: %v", err)
	}

	if !exists {
		return false, fmt.Sprintf("El directorio '%s' no existe", dirPath)
	}

	if pathType != "directorio" {
		return false, fmt.Sprintf("La ruta '%s' no es un directorio", dirPath)
	}

	// 6. Obtener el inodo del directorio
	inodeNum, inodePtr, err := FindInodeByPath(file, startByte, superblock, dirPath)
	if err != nil {
		return false, fmt.Sprintf("Error encontrando inodo del directorio: %v", err)
	}

	fmt.Printf("Inodo del directorio '%s': %d\n", dirPath, inodeNum)

	// 7. Iniciar contenido del reporte DOT
	var dot strings.Builder
	dot.WriteString("digraph G {\n")
	dot.WriteString("  node [shape=none];\n")
	dot.WriteString("  rankdir=LR;\n")
	dot.WriteString("  ls_info [label=<\n")
	dot.WriteString("    <table border='0' cellborder='1' cellspacing='0'>\n")

	// 8. Encabezados de la tabla
	dot.WriteString("      <tr>\n")
	dot.WriteString("        <td bgcolor='#90EE90'><b>PERMISOS</b></td>\n")
	dot.WriteString("        <td bgcolor='#90EE90'><b>PROPIETARIO</b></td>\n")
	dot.WriteString("        <td bgcolor='#90EE90'><b>GRUPO</b></td>\n")
	dot.WriteString("        <td bgcolor='#90EE90'><b>TAMAÑO</b></td>\n")
	dot.WriteString("        <td bgcolor='#90EE90'><b>MODIFICACIÓN</b></td>\n")
	dot.WriteString("        <td bgcolor='#90EE90'><b>TIPO</b></td>\n")
	dot.WriteString("        <td bgcolor='#90EE90'><b>NOMBRE</b></td>\n")
	dot.WriteString("      </tr>\n")

	// 9. Leer los bloques del directorio
	entryCount := 0
	blocksStart := startByte + int64(superblock.SBlockStart)
	inodeTableStart := startByte + int64(superblock.SInodeStart)

	// Recorrer los bloques directos del inodo
	for i := 0; i < 12; i++ {
		blockNum := inodePtr.IBlock[i]
		if blockNum <= 0 {
			continue
		}

		// Leer el bloque de directorio
		blockPos := blocksStart + int64(blockNum)*int64(superblock.SBlockSize)
		_, err := file.Seek(blockPos, 0)
		if err != nil {
			fmt.Printf("Error posicionándose en bloque %d: %v\n", blockNum, err)
			continue
		}

		// Leer el bloque como una estructura DirectoryBlock
		var dirBlock DirectoryBlock
		err = readDirectoryBlockFromDisk(file, &dirBlock)
		if err != nil {
			fmt.Printf("Error leyendo bloque de directorio %d: %v\n", blockNum, err)
			continue
		}

		// Procesar cada entrada del directorio
		entries := dirBlock.ListEntries()
		for _, entry := range entries {
			// Ignorar "." y ".."
			if entry.Name == "." || entry.Name == ".." {
				continue
			}

			// Leer el inodo de esta entrada
			entryInodePos := inodeTableStart + int64(entry.InodeNum-1)*int64(superblock.SInodeSize)
			_, err = file.Seek(entryInodePos, 0)
			if err != nil {
				fmt.Printf("Error posicionándose en inodo %d: %v\n", entry.InodeNum, err)
				continue
			}

			entryInode, err := readInodeFromDisc(file)
			if err != nil {
				fmt.Printf("Error leyendo inodo %d: %v\n", entry.InodeNum, err)
				continue
			}

			// Determinar tipo
			fileType := "Archivo"
			if entryInode.IType == 0 {
				fileType = "Directorio"
			}

			// Formatear permisos
			permStr := formatPermissions(entryInode.IPerm)

			// Formatear propietario y grupo
			uidStr := fmt.Sprintf("UID:%d", entryInode.IUid)
			gidStr := fmt.Sprintf("GID:%d", entryInode.IGid)

			// Formatear fecha de modificación
			modTimeStr := entryInode.IMtime.Format("2006-01-02 15:04:05")

			// Añadir fila a la tabla
			dot.WriteString("      <tr>\n")
			dot.WriteString(fmt.Sprintf("        <td>%s</td>\n", permStr))
			dot.WriteString(fmt.Sprintf("        <td>%s</td>\n", uidStr))
			dot.WriteString(fmt.Sprintf("        <td>%s</td>\n", gidStr))
			dot.WriteString(fmt.Sprintf("        <td>%d bytes</td>\n", entryInode.ISize))
			dot.WriteString(fmt.Sprintf("        <td>%s</td>\n", modTimeStr))
			dot.WriteString(fmt.Sprintf("        <td>%s</td>\n", fileType))
			dot.WriteString(fmt.Sprintf("        <td>%s</td>\n", entry.Name))
			dot.WriteString("      </tr>\n")

			entryCount++
		}
	}

	// 10. Si no hay entradas, mostrar mensaje
	if entryCount == 0 {
		dot.WriteString("      <tr>\n")
		dot.WriteString("        <td colspan='7' align='center'>Directorio vacío</td>\n")
		dot.WriteString("      </tr>\n")
	}

	// 11. Finalizar tabla y gráfico
	dot.WriteString("    </table>\n")
	dot.WriteString("  >];\n")
	dot.WriteString("}\n")

	// 12. Guardar archivo DOT
	dotPath := strings.TrimSuffix(reportPath, filepath.Ext(reportPath)) + ".dot"
	err = os.WriteFile(dotPath, []byte(dot.String()), 0644)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir archivo DOT: %v", err)
	}

	// 13. Generar imagen usando Graphviz
	fmt.Printf("Generando imagen: %s\n", reportPath)
	cmd := exec.Command("dot", "-Tpng", dotPath, "-o", reportPath)
	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("Error al generar imagen: %v", err)
	}

	return true, fmt.Sprintf("Reporte LS generado exitosamente en: %s", reportPath)
}

// readDirectoryBlockFromDisk lee un bloque de directorio del disco
func readDirectoryBlockFromDisk(file *os.File, dirBlock *DirectoryBlock) error {
	// Leer cada campo de BContent secuencialmente
	for i := 0; i < B_CONTENT_COUNT; i++ {
		// Leer el nombre (array de B_NAME_SIZE bytes)
		_, err := file.Read(dirBlock.BContent[i].BName[:])
		if err != nil {
			return fmt.Errorf("error leyendo nombre de entrada %d: %v", i, err)
		}

		// Leer el número de inodo (4 bytes)
		err = binary.Read(file, binary.LittleEndian, &dirBlock.BContent[i].BInodo)
		if err != nil {
			return fmt.Errorf("error leyendo inodo de entrada %d: %v", i, err)
		}
	}

	return nil
}
