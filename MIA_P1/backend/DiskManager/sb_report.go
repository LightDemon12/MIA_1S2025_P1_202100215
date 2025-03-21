package DiskManager

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SbReporter genera un reporte gráfico del superbloque
func SbReporter(id, path string) (bool, string) {
	// 1. Encontrar la partición montada
	mountedPartition, err := FindMountedPartitionById(id)
	if err != nil {
		return false, fmt.Sprintf("Error: %s", err)
	}

	// 2. Abrir el disco
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return false, fmt.Sprintf("Error al abrir el disco: %s", err)
	}
	defer file.Close()

	// 3. Obtener detalles de la partición
	startByte, _, err := GetPartitionDetails(file, mountedPartition)
	if err != nil {
		return false, fmt.Sprintf("Error al obtener detalles de la partición: %s", err)
	}

	// 4. Leer el superbloque
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en el superbloque: %s", err)
	}

	superblock, err := ReadSuperBlockFromDisc(file)
	if err != nil {
		return false, fmt.Sprintf("Error al leer el superbloque: %s", err)
	}

	// 5. Generar el DOT para Graphviz
	var dot strings.Builder
	dot.WriteString("digraph SuperBlock {\n")
	dot.WriteString("  node [shape=plaintext, fontname=\"Arial\"];\n")
	dot.WriteString("  ranksep=0.5;\n")
	dot.WriteString("  nodesep=0.5;\n")
	dot.WriteString("  bgcolor=\"white\";\n")
	dot.WriteString("  labelloc=\"t\";\n")
	dot.WriteString("  fontname=\"Arial\";\n")
	dot.WriteString("  fontsize=20;\n")
	dot.WriteString(fmt.Sprintf("  label=\"Reporte de Superbloque - Partición %s\";\n\n", mountedPartition.ID))

	// Crear la tabla HTML para mostrar los datos del superbloque
	dot.WriteString("  superblock [label=<\n")
	dot.WriteString("    <TABLE BORDER=\"0\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\">\n")

	// Cabecera de la tabla
	dot.WriteString("      <TR><TD BGCOLOR=\"#673AB7\" COLSPAN=\"2\"><FONT COLOR=\"white\" POINT-SIZE=\"14\">INFORMACIÓN DEL SUPERBLOQUE</FONT></TD></TR>\n")

	// Función para añadir una fila a la tabla
	addRow := func(name, value string, altColor bool) {
		bg := ""
		if altColor {
			bg = " BGCOLOR=\"#F3E5F5\""
		}
		dot.WriteString(fmt.Sprintf("      <TR><TD%s>%s</TD><TD%s>%s</TD></TR>\n", bg, name, bg, value))
	}

	// Información general del disco y partición
	diskName := filepath.Base(mountedPartition.DiskPath)
	addRow("Nombre del disco", diskName, true)
	addRow("ID de partición", mountedPartition.ID, false)

	// Información del sistema de archivos
	fsType := "EXT2"
	if superblock.SFilesystemType == 1 {
		fsType = "EXT3"
	}
	addRow("Tipo de sistema de archivos", fsType, true)

	// Inodos
	inodeUsagePercent := float64(superblock.SInodesCount-superblock.SFreeInodesCount) * 100.0 / float64(superblock.SInodesCount)
	addRow("Número total de inodos", fmt.Sprintf("%d", superblock.SInodesCount), false)
	addRow("Número de inodos libres", fmt.Sprintf("%d", superblock.SFreeInodesCount), true)
	addRow("Porcentaje de uso de inodos", fmt.Sprintf("%.2f%%", inodeUsagePercent), false)
	addRow("Tamaño de cada inodo", fmt.Sprintf("%d bytes", superblock.SInodeSize), true)
	addRow("Primer inodo libre", fmt.Sprintf("%d", superblock.SFirstIno), false)

	// Bloques
	blockUsagePercent := float64(superblock.SBlocksCount-superblock.SFreeBlocksCount) * 100.0 / float64(superblock.SBlocksCount)
	addRow("Número total de bloques", fmt.Sprintf("%d", superblock.SBlocksCount), true)
	addRow("Número de bloques libres", fmt.Sprintf("%d", superblock.SFreeBlocksCount), false)
	addRow("Porcentaje de uso de bloques", fmt.Sprintf("%.2f%%", blockUsagePercent), true)
	addRow("Tamaño de cada bloque", fmt.Sprintf("%d bytes", superblock.SBlockSize), false)
	addRow("Primer bloque libre", fmt.Sprintf("%d", superblock.SFirstBlo), true)

	// Posiciones
	addRow("Inicio del bitmap de inodos", fmt.Sprintf("%d", superblock.SBmInodeStart), false)
	addRow("Inicio del bitmap de bloques", fmt.Sprintf("%d", superblock.SBmBlockStart), true)
	addRow("Inicio de la tabla de inodos", fmt.Sprintf("%d", superblock.SInodeStart), false)
	addRow("Inicio de la tabla de bloques", fmt.Sprintf("%d", superblock.SBlockStart), true)

	// Fechas y montajes
	mountTime := superblock.SMtime.Format("2006-01-02 15:04:05")

	// Verificar si la fecha de desmontaje es válida (año > 1900)
	unmountTime := "----------"
	if superblock.SUmtime.Year() > 1900 {
		unmountTime = superblock.SUmtime.Format("2006-01-02 15:04:05")
	}

	addRow("Última fecha de montaje", mountTime, false)
	addRow("Última fecha de desmontaje", unmountTime, true)
	addRow("Número de veces montado", fmt.Sprintf("%d", superblock.SMntCount), false)

	// Valor mágico
	magicHex := fmt.Sprintf("0x%X", superblock.SMagic)
	isValidMagic := superblock.SMagic == 0xEF53
	magicStatus := "Válido"
	if !isValidMagic {
		magicStatus = "Inválido"
	}
	addRow("Valor mágico", fmt.Sprintf("%s (%s)", magicHex, magicStatus), true)

	// Cerrar la tabla
	dot.WriteString("    </TABLE>\n")
	dot.WriteString("  >];\n")

	// Añadir visualización de uso de inodos y bloques con gráficos de barras
	dot.WriteString("\n  // Gráfico de uso de inodos\n")
	dot.WriteString("  inodeUsage [label=<\n")
	dot.WriteString("    <TABLE BORDER=\"0\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\">\n")
	dot.WriteString("      <TR><TD BGCOLOR=\"#2196F3\" COLSPAN=\"2\"><FONT COLOR=\"white\">Uso de Inodos</FONT></TD></TR>\n")
	dot.WriteString("      <TR><TD COLSPAN=\"2\">\n")
	dot.WriteString("        <TABLE BORDER=\"0\" CELLBORDER=\"0\" CELLSPACING=\"0\" CELLPADDING=\"0\" WIDTH=\"300\">\n")
	dot.WriteString("          <TR>\n")

	// Barra de uso (azul)
	usedInodes := superblock.SInodesCount - superblock.SFreeInodesCount
	usedInodeWidth := int(300.0 * float64(usedInodes) / float64(superblock.SInodesCount))
	freeInodeWidth := 300 - usedInodeWidth

	// Asegurar que tengamos al menos 1px de ancho si hay algún inodo usado (para visibilidad)
	if usedInodes > 0 && usedInodeWidth < 1 {
		usedInodeWidth = 1
		freeInodeWidth = 299
	}

	if usedInodeWidth > 0 {
		dot.WriteString(fmt.Sprintf("            <TD BGCOLOR=\"#2196F3\" WIDTH=\"%d\"></TD>\n", usedInodeWidth))
	}
	// Barra libre (gris claro)
	if freeInodeWidth > 0 {
		dot.WriteString(fmt.Sprintf("            <TD BGCOLOR=\"#E0E0E0\" WIDTH=\"%d\"></TD>\n", freeInodeWidth))
	}
	dot.WriteString("          </TR>\n")
	dot.WriteString("        </TABLE>\n")
	dot.WriteString("      </TD></TR>\n")
	dot.WriteString(fmt.Sprintf("      <TR><TD>Usados: %d (%.2f%%)</TD><TD>Libres: %d (%.2f%%)</TD></TR>\n",
		usedInodes, inodeUsagePercent,
		superblock.SFreeInodesCount, 100.0-inodeUsagePercent))
	dot.WriteString("    </TABLE>\n")
	dot.WriteString("  >];\n")

	dot.WriteString("\n  // Gráfico de uso de bloques\n")
	dot.WriteString("  blockUsage [label=<\n")
	dot.WriteString("    <TABLE BORDER=\"0\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\">\n")
	dot.WriteString("      <TR><TD BGCOLOR=\"#4CAF50\" COLSPAN=\"2\"><FONT COLOR=\"white\">Uso de Bloques</FONT></TD></TR>\n")
	dot.WriteString("      <TR><TD COLSPAN=\"2\">\n")
	dot.WriteString("        <TABLE BORDER=\"0\" CELLBORDER=\"0\" CELLSPACING=\"0\" CELLPADDING=\"0\" WIDTH=\"300\">\n")
	dot.WriteString("          <TR>\n")

	// Barra de uso (verde)
	usedBlocks := superblock.SBlocksCount - superblock.SFreeBlocksCount
	usedBlockWidth := int(300.0 * float64(usedBlocks) / float64(superblock.SBlocksCount))
	freeBlockWidth := 300 - usedBlockWidth

	// Asegurar que tengamos al menos 1px de ancho si hay algún bloque usado (para visibilidad)
	if usedBlocks > 0 && usedBlockWidth < 1 {
		usedBlockWidth = 1
		freeBlockWidth = 299
	}

	if usedBlockWidth > 0 {
		dot.WriteString(fmt.Sprintf("            <TD BGCOLOR=\"#4CAF50\" WIDTH=\"%d\"></TD>\n", usedBlockWidth))
	}
	// Barra libre (gris claro)
	if freeBlockWidth > 0 {
		dot.WriteString(fmt.Sprintf("            <TD BGCOLOR=\"#E0E0E0\" WIDTH=\"%d\"></TD>\n", freeBlockWidth))
	}
	dot.WriteString("          </TR>\n")
	dot.WriteString("        </TABLE>\n")
	dot.WriteString("      </TD></TR>\n")
	dot.WriteString(fmt.Sprintf("      <TR><TD>Usados: %d (%.2f%%)</TD><TD>Libres: %d (%.2f%%)</TD></TR>\n",
		usedBlocks, blockUsagePercent,
		superblock.SFreeBlocksCount, 100.0-blockUsagePercent))
	dot.WriteString("    </TABLE>\n")
	dot.WriteString("  >];\n")

	// Conexiones para mostrar jerarquía
	dot.WriteString("\n  // Conexiones\n")
	dot.WriteString("  superblock -> inodeUsage [style=invis];\n")
	dot.WriteString("  inodeUsage -> blockUsage [style=invis];\n")

	// Organización vertical
	dot.WriteString("\n  // Organización\n")
	dot.WriteString("  { rank=same; superblock }\n")
	dot.WriteString("  { rank=same; inodeUsage }\n")
	dot.WriteString("  { rank=same; blockUsage }\n")

	// Cerrar el grafo
	dot.WriteString("}\n")

	// 6. Guardar el DOT
	dotFile := path + ".dot"
	err = os.WriteFile(dotFile, []byte(dot.String()), 0644)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir archivo DOT: %s", err)
	}

	// 7. Generar imagen
	cmd := exec.Command("dot", "-Tjpg", dotFile, "-o", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("Error al ejecutar Graphviz: %v\nStdout: %s\nStderr: %s\nArchivo DOT guardado en: %s",
			err, stdout.String(), stderr.String(), dotFile)
	}

	return true, fmt.Sprintf("Reporte de superbloque generado exitosamente en: %s", path)
}
