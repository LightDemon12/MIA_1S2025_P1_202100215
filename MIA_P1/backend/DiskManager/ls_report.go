package DiskManager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LSReporter genera un reporte de todos los inodos del sistema
func LSReporter(id string, reportPath string, dirPath string) (bool, string) {
	fmt.Printf("Generando reporte LS para todos los inodos del sistema\n")

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

	// 5. Iniciar contenido del reporte DOT
	var dot strings.Builder
	dot.WriteString("digraph G {\n")
	dot.WriteString("  node [shape=none];\n")
	dot.WriteString("  rankdir=LR;\n")
	dot.WriteString("  ls_info [label=<\n")
	dot.WriteString("    <table border='0' cellborder='1' cellspacing='0'>\n")

	// 6. Encabezados de la tabla
	dot.WriteString("      <tr>\n")
	dot.WriteString("        <td bgcolor='#90EE90'><b>INODO</b></td>\n")
	dot.WriteString("        <td bgcolor='#90EE90'><b>TIPO</b></td>\n")
	dot.WriteString("        <td bgcolor='#90EE90'><b>PERMISOS</b></td>\n")
	dot.WriteString("        <td bgcolor='#90EE90'><b>PROPIETARIO</b></td>\n")
	dot.WriteString("        <td bgcolor='#90EE90'><b>GRUPO</b></td>\n")
	dot.WriteString("        <td bgcolor='#90EE90'><b>TAMAÑO</b></td>\n")
	dot.WriteString("        <td bgcolor='#90EE90'><b>BLOQUES</b></td>\n")
	dot.WriteString("        <td bgcolor='#90EE90'><b>MODIFICACIÓN</b></td>\n")
	dot.WriteString("      </tr>\n")

	// 7. Leer el bitmap de inodos
	inodeBitmapPos := startByte + int64(superblock.SBmInodeStart)
	_, err = file.Seek(inodeBitmapPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error posicionándose en el bitmap de inodos: %v", err)
	}

	inodeBitmap := make([]byte, superblock.SInodesCount/8+1)
	_, err = file.Read(inodeBitmap)
	if err != nil {
		return false, fmt.Sprintf("Error leyendo el bitmap de inodos: %v", err)
	}

	// 8. Recorrer todos los inodos en uso
	inodeTableStart := startByte + int64(superblock.SInodeStart)
	entryCount := 0

	for i := 0; i < int(superblock.SInodesCount); i++ {
		// Verificar si el inodo está en uso
		bytePos := i / 8
		bitPos := i % 8

		if bytePos < len(inodeBitmap) && (inodeBitmap[bytePos]&(1<<bitPos)) != 0 {
			// Leer el inodo
			inodePos := inodeTableStart + int64(i)*int64(superblock.SInodeSize)
			_, err = file.Seek(inodePos, 0)
			if err != nil {
				fmt.Printf("Error posicionándose en inodo %d: %v\n", i, err)
				continue
			}

			inode, err := readInodeFromDisc(file)
			if err != nil {
				fmt.Printf("Error leyendo inodo %d: %v\n", i, err)
				continue
			}

			// Determinar tipo
			fileType := "Archivo"
			if inode.IType == 0 {
				fileType = "Directorio"
			}

			// Formatear permisos
			permStr := formatPermissions(inode.IPerm)

			// Formatear propietario y grupo
			uidStr := fmt.Sprintf("UID:%d", inode.IUid)
			gidStr := fmt.Sprintf("GID:%d", inode.IGid)

			// Formatear fecha de modificación
			modTimeStr := inode.IMtime.Format("2006-01-02 15:04:05")

			// Contar bloques usados
			blocksUsed := 0
			for j := 0; j < 12; j++ {
				if inode.IBlock[j] != -1 {
					blocksUsed++
				}
			}

			// Añadir fila a la tabla
			dot.WriteString("      <tr>\n")
			dot.WriteString(fmt.Sprintf("        <td>%d</td>\n", i))
			dot.WriteString(fmt.Sprintf("        <td>%s</td>\n", fileType))
			dot.WriteString(fmt.Sprintf("        <td>%s</td>\n", permStr))
			dot.WriteString(fmt.Sprintf("        <td>%s</td>\n", uidStr))
			dot.WriteString(fmt.Sprintf("        <td>%s</td>\n", gidStr))
			dot.WriteString(fmt.Sprintf("        <td>%d bytes</td>\n", inode.ISize))
			dot.WriteString(fmt.Sprintf("        <td>%d</td>\n", blocksUsed))
			dot.WriteString(fmt.Sprintf("        <td>%s</td>\n", modTimeStr))
			dot.WriteString("      </tr>\n")

			entryCount++
		}
	}

	// 9. Si no hay entradas, mostrar mensaje
	if entryCount == 0 {
		dot.WriteString("      <tr>\n")
		dot.WriteString("        <td colspan='8' align='center'>No se encontraron inodos en uso</td>\n")
		dot.WriteString("      </tr>\n")
	}

	// 10. Finalizar tabla y gráfico
	dot.WriteString("    </table>\n")
	dot.WriteString("  >];\n")
	dot.WriteString("}\n")

	// 11. Guardar archivo DOT
	dotPath := strings.TrimSuffix(reportPath, filepath.Ext(reportPath)) + ".dot"
	err = os.WriteFile(dotPath, []byte(dot.String()), 0644)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir archivo DOT: %v", err)
	}

	// 12. Generar imagen usando Graphviz
	fmt.Printf("Generando imagen para: %s\n", reportPath)
	cmd := exec.Command("dot", "-Tpng", dotPath, "-o", reportPath)
	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("Error al generar imagen: %v", err)
	}

	return true, fmt.Sprintf("Reporte LS (todos los inodos) generado exitosamente en: %s", reportPath)
}
