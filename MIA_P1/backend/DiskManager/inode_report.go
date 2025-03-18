package DiskManager

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type SuperBlockDisk struct {
	SFilesystemType  int32     // Número identificador del sistema de archivos
	SInodesCount     int32     // Número total de inodos
	SBlocksCount     int32     // Número total de bloques
	SFreeBlocksCount int32     // Número de bloques libres
	SFreeInodesCount int32     // Número de inodos libres
	SMtime           int64     // Timestamp Unix para la última montada
	SUmtime          int64     // Timestamp Unix para la última desmontada
	SMntCount        int32     // Número de veces que se ha montado el sistema
	SMagic           int32     // Valor mágico que identifica al sistema (0xEF53)
	SInodeSize       int32     // Tamaño de cada inodo
	SBlockSize       int32     // Tamaño de cada bloque
	SFirstIno        int32     // Dirección del primer inodo libre
	SFirstBlo        int32     // Dirección del primer bloque libre
	SBmInodeStart    int32     // Inicio del bitmap de inodos
	SBmBlockStart    int32     // Inicio del bitmap de bloques
	SInodeStart      int32     // Inicio de la tabla de inodos
	SBlockStart      int32     // Inicio de la tabla de bloques
	SPadding         [808]byte // Padding para asegurar un tamaño total de 1024 bytes
}

// GenerateInodeReport genera un reporte gráfico de los inodos utilizados
func GenerateInodeReport(diskPath string, partitionStartByte int64, outputPath string) (string, error) {
	// 1. Abrir el archivo de disco
	file, err := os.Open(diskPath)
	if err != nil {
		return "", fmt.Errorf("error abriendo el disco: %w", err)
	}
	defer file.Close()

	// 2. Leer el superbloque usando SuperBlockDisk para obtener información de la partición
	var sbDisk SuperBlockDisk
	_, err = file.Seek(partitionStartByte, 0)
	if err != nil {
		return "", fmt.Errorf("error posicionándose en la partición: %w", err)
	}

	err = binary.Read(file, binary.LittleEndian, &sbDisk)
	if err != nil {
		return "", fmt.Errorf("error leyendo el superbloque: %w", err)
	}

	// Verificar si es un sistema de archivos EXT2
	if sbDisk.SMagic != 0xEF53 { // EXT2 Magic Number
		return "", fmt.Errorf("el sistema de archivos no es EXT2 (magic: %x)", sbDisk.SMagic)
	}

	// 3. Leer el bitmap de inodos para identificar cuáles están en uso
	bmInodePos := partitionStartByte + int64(sbDisk.SBmInodeStart)
	_, err = file.Seek(bmInodePos, 0)
	if err != nil {
		return "", fmt.Errorf("error posicionándose en el bitmap de inodos: %w", err)
	}

	bmInodes := make([]byte, sbDisk.SInodesCount/8+1)
	_, err = file.Read(bmInodes)
	if err != nil {
		return "", fmt.Errorf("error leyendo el bitmap de inodos: %w", err)
	}

	// 4. Preparar el contenido del gráfico DOT con estilo mejorado
	dotContent := `digraph G {
  // Configuración global del gráfico
  bgcolor="#f8f9fa";
  fontname="Arial,sans-serif";
  fontsize=18;
  
  // Configuración de nodos - cambiando a HTML-like
  node [
    shape=none,
    fontname="Arial,sans-serif",
    fontsize=12
  ];
  
  edge [
    color="#3F51B5",
    style="dashed",
    penwidth=1.2,
    arrowhead=vee,
    fontsize=10,
    fontcolor="#666666"
  ];
  
  // Orden de arriba hacia abajo, pero los nodos se organizarán horizontalmente
  rankdir=LR;
  
  // Título del gráfico
  labelloc="t";
  label=<<FONT FACE="Arial,sans-serif" POINT-SIZE="22" COLOR="#333333"><B>Reporte de Inodos EXT2</B></FONT>>;

`

	// 5. Procesar cada inodo en uso
	inodeTablePos := partitionStartByte + int64(sbDisk.SInodeStart)
	var activeInodes []int // Almacenar los IDs de los inodos activos para crear conexiones

	for i := 0; i < int(sbDisk.SInodesCount); i++ {
		// Verificar si el inodo está en uso según el bitmap
		byteIndex := i / 8
		bitIndex := i % 8

		if byteIndex < len(bmInodes) && (bmInodes[byteIndex]&(1<<bitIndex)) != 0 {
			// Inodo en uso según el bitmap, leerlo
			inodePos := inodeTablePos + int64(i)*int64(sbDisk.SInodeSize)
			_, err = file.Seek(inodePos, 0)
			if err != nil {
				return "", fmt.Errorf("error posicionándose en inodo %d: %w", i, err)
			}

			// Leer el inodo
			inode, err := readInodeFromDisk(file, int(sbDisk.SInodeSize))
			if err != nil {
				return "", fmt.Errorf("error leyendo inodo %d: %w", i, err)
			}

			// Verificar si el inodo realmente tiene contenido útil
			hasAssignedBlocks := false
			for _, blockID := range inode.IBlock {
				if blockID != -1 {
					hasAssignedBlocks = true
					break
				}
			}

			// Si no tiene contenido útil, saltarlo - PERO mostramos el inodo 2 (directorio raíz) siempre
			if inode.ISize == 0 && !hasAssignedBlocks && i != 2 {
				continue // Saltar este inodo
			}

			// Agregar el ID del inodo a la lista de activos
			activeInodes = append(activeInodes, i)

			// Determinar el tipo de inodo
			inodeType := "Carpeta"
			headerColor := "#7986CB" // Azul para carpetas
			if inode.IType == INODE_FILE {
				inodeType = "Archivo"
				headerColor = "#66BB6A" // Verde para archivos
			}

			// Formatear permisos en modo legible (rwx)
			permissionStr := formatPermissions(inode.IPerm)

			// Crear la etiqueta del nodo con formato HTML, ahora usando shape=none
			dotContent += fmt.Sprintf(`  inode%d [
    label=<<TABLE BORDER="0" CELLBORDER="1" CELLSPACING="0" CELLPADDING="4" STYLE="ROUNDED">
    <TR><TD COLSPAN="2" BGCOLOR="%s" BORDER="1" STYLE="ROUNDED"><FONT COLOR="white"><B>Inodo %d</B></FONT></TD></TR>
`, i, headerColor, i)

			// Información básica con colores mejorados
			dotContent += `    <TR><TD COLSPAN="2">
      <TABLE BORDER="0" CELLBORDER="0" CELLSPACING="2" CELLPADDING="1">
`
			dotContent += fmt.Sprintf("        <TR><TD ALIGN=\"LEFT\"><B>Tipo:</B></TD><TD ALIGN=\"LEFT\">%s</TD></TR>\n", inodeType)
			dotContent += fmt.Sprintf("        <TR><TD ALIGN=\"LEFT\"><B>Tamaño:</B></TD><TD ALIGN=\"LEFT\">%d bytes</TD></TR>\n", inode.ISize)
			dotContent += fmt.Sprintf("        <TR><TD ALIGN=\"LEFT\"><B>UID:</B></TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", inode.IUid)
			dotContent += fmt.Sprintf("        <TR><TD ALIGN=\"LEFT\"><B>GID:</B></TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n", inode.IGid)
			dotContent += fmt.Sprintf("        <TR><TD ALIGN=\"LEFT\"><B>Permisos:</B></TD><TD ALIGN=\"LEFT\"><FONT FACE=\"monospace\">%s</FONT> (%d%d%d)</TD></TR>\n",
				permissionStr, inode.IPerm[0], inode.IPerm[1], inode.IPerm[2])
			dotContent += fmt.Sprintf("        <TR><TD ALIGN=\"LEFT\"><B>Creación:</B></TD><TD ALIGN=\"LEFT\">%s</TD></TR>\n",
				inode.ICtime.Format("2006-01-02 15:04:05"))
			dotContent += fmt.Sprintf("        <TR><TD ALIGN=\"LEFT\"><B>Modificación:</B></TD><TD ALIGN=\"LEFT\">%s</TD></TR>\n",
				inode.IMtime.Format("2006-01-02 15:04:05"))
			dotContent += fmt.Sprintf("        <TR><TD ALIGN=\"LEFT\"><B>Acceso:</B></TD><TD ALIGN=\"LEFT\">%s</TD></TR>\n",
				inode.IAtime.Format("2006-01-02 15:04:05"))
			dotContent += `      </TABLE>
    </TD></TR>
`

			// Bloques directos con coloración suave
			dotContent += `    <TR><TD COLSPAN="2">
      <TABLE BORDER="0" CELLBORDER="0" CELLSPACING="2" CELLPADDING="1">
        <TR><TD COLSPAN="2" ALIGN="LEFT" BGCOLOR="#E8EAF6"><B>Bloques directos:</B></TD></TR>
`

			blocksDirectosVacios := true
			for j := 0; j < 12; j++ {
				if inode.IBlock[j] != -1 {
					blocksDirectosVacios = false
					dotContent += fmt.Sprintf("        <TR><TD ALIGN=\"RIGHT\"><FONT COLOR=\"#5C6BC0\">%d:</FONT></TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n",
						j, inode.IBlock[j])
				}
			}

			if blocksDirectosVacios {
				dotContent += "        <TR><TD COLSPAN=\"2\" ALIGN=\"LEFT\"><FONT COLOR=\"#9E9E9E\">[Ninguno]</FONT></TD></TR>\n"
			}
			dotContent += `      </TABLE>
    </TD></TR>
`

			// Bloques indirectos con coloración diferente
			dotContent += `    <TR><TD COLSPAN="2">
      <TABLE BORDER="0" CELLBORDER="0" CELLSPACING="2" CELLPADDING="1">
        <TR><TD COLSPAN="2" ALIGN="LEFT" BGCOLOR="#FFF8E1"><B>Bloques indirectos:</B></TD></TR>
`

			indirectosVacios := true

			// Indirecto simple
			if inode.IBlock[INDIRECT_BLOCK_INDEX] != -1 {
				indirectosVacios = false
				dotContent += fmt.Sprintf("        <TR><TD ALIGN=\"RIGHT\"><FONT COLOR=\"#FFB300\">Simple:</FONT></TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n",
					inode.IBlock[INDIRECT_BLOCK_INDEX])
			}

			// Indirecto doble
			if inode.IBlock[DOUBLE_INDIRECT_BLOCK_INDEX] != -1 {
				indirectosVacios = false
				dotContent += fmt.Sprintf("        <TR><TD ALIGN=\"RIGHT\"><FONT COLOR=\"#FF8F00\">Doble:</FONT></TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n",
					inode.IBlock[DOUBLE_INDIRECT_BLOCK_INDEX])
			}

			// Indirecto triple
			if inode.IBlock[TRIPLE_INDIRECT_BLOCK_INDEX] != -1 {
				indirectosVacios = false
				dotContent += fmt.Sprintf("        <TR><TD ALIGN=\"RIGHT\"><FONT COLOR=\"#EF6C00\">Triple:</FONT></TD><TD ALIGN=\"LEFT\">%d</TD></TR>\n",
					inode.IBlock[TRIPLE_INDIRECT_BLOCK_INDEX])
			}

			if indirectosVacios {
				dotContent += "        <TR><TD COLSPAN=\"2\" ALIGN=\"LEFT\"><FONT COLOR=\"#9E9E9E\">[Ninguno]</FONT></TD></TR>\n"
			}
			dotContent += `      </TABLE>
    </TD></TR>
`

			// Cierre de la tabla principal
			dotContent += `  </TABLE>>];
`
		}
	}

	// Agregar conexiones entre inodos
	if len(activeInodes) > 1 {
		dotContent += "\n  // Conexiones entre inodos\n"
		for i := 0; i < len(activeInodes)-1; i++ {
			dotContent += fmt.Sprintf("  inode%d -> inode%d [label=\" siguiente \"];\n",
				activeInodes[i], activeInodes[i+1])
		}
	}

	// 6. Cerrar el gráfico DOT
	dotContent += "}\n"

	// 7. Asegurar que el directorio exista
	err = os.MkdirAll(filepath.Dir(outputPath), 0755)
	if err != nil {
		return "", fmt.Errorf("error creando directorios para la salida: %w", err)
	}

	// 8. Escribir el archivo DOT
	dotOutputPath := outputPath
	if !strings.HasSuffix(dotOutputPath, ".dot") {
		dotOutputPath = strings.TrimSuffix(dotOutputPath, filepath.Ext(dotOutputPath)) + ".dot"
	}

	err = os.WriteFile(dotOutputPath, []byte(dotContent), 0644)
	if err != nil {
		return "", fmt.Errorf("error escribiendo archivo DOT: %w", err)
	}

	// 9. Convertir DOT a imagen usando Graphviz
	outputFilePath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath))
	outputExt := filepath.Ext(outputPath)
	if outputExt == "" || outputExt == ".dot" {
		outputExt = ".png" // Por defecto PNG
		outputPath = outputFilePath + outputExt
	}

	// Ejecutar el comando dot para generar la imagen
	format := strings.TrimPrefix(outputExt, ".")
	cmd := exec.Command("dot", "-T"+format, dotOutputPath, "-o", outputPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return dotOutputPath, fmt.Errorf("error generando la imagen (se guarda el archivo DOT): %w\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}

	return outputPath, nil
}

// formatPermissions convierte los valores octales de permisos a formato rwx
func formatPermissions(perm [3]byte) string {
	var result string

	// User permissions
	result += permissionToRWX(perm[0])

	// Group permissions
	result += permissionToRWX(perm[1])

	// Other permissions
	result += permissionToRWX(perm[2])

	return result
}

// permissionToRWX convierte un número octal de permisos a rwx
func permissionToRWX(p byte) string {
	var result string

	// Read permission (4)
	if p&4 != 0 {
		result += "r"
	} else {
		result += "-"
	}

	// Write permission (2)
	if p&2 != 0 {
		result += "w"
	} else {
		result += "-"
	}

	// Execute permission (1)
	if p&1 != 0 {
		result += "x"
	} else {
		result += "-"
	}

	return result
}

// readInodeFromDisk lee un inodo desde la posición actual del archivo
func readInodeFromDisk(file *os.File, inodeSize int) (*Inode, error) {
	inode := &Inode{}

	// Leer campos en orden preciso
	if err := binary.Read(file, binary.LittleEndian, &inode.IUid); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &inode.IGid); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &inode.ISize); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &inode.IPerm); err != nil {
		return nil, err
	}

	// Leer timestamps
	var aTime, cTime, mTime int64
	if err := binary.Read(file, binary.LittleEndian, &aTime); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &cTime); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &mTime); err != nil {
		return nil, err
	}

	// Convertir los timestamps
	inode.IAtime = time.Unix(aTime, 0)
	inode.ICtime = time.Unix(cTime, 0)
	inode.IMtime = time.Unix(mTime, 0)

	// Leer bloques
	if err := binary.Read(file, binary.LittleEndian, &inode.IBlock); err != nil {
		return nil, err
	}

	// Leer tipo de inodo
	if err := binary.Read(file, binary.LittleEndian, &inode.IType); err != nil {
		return nil, err
	}

	// Leer padding
	if err := binary.Read(file, binary.LittleEndian, &inode.IPadding); err != nil {
		return nil, err
	}

	// Manejar el posible padding adicional
	remainingSize := inodeSize - 104 // Padding restante

	if remainingSize > 0 {
		padding := make([]byte, remainingSize)
		if _, err := file.Read(padding); err != nil {
			return nil, err
		}
	}

	return inode, nil
}
