package DiskManager

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Estructura para almacenar información sobre las particiones y espacios
type DiskSection struct {
	start       int64
	size        int64
	percentage  float64
	sectionType string // "MBR", "PRIMARY", "EXTENDED", "EBR", "LOGICAL", "FREE"
	name        string
	fit         byte
	status      byte
	index       int
	subSections []DiskSection // Para particiones lógicas dentro de extendidas
}

func GenerateDiskReport(diskPath, outputPath string) (string, error) {
	// 1. Abrir el disco
	file, err := os.OpenFile(diskPath, os.O_RDONLY, 0666)
	if err != nil {
		return "", fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	// 2. Leer el MBR
	mbr := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, mbr); err != nil {
		return "", fmt.Errorf("error leyendo MBR: %v", err)
	}

	// 3. Verificar directorio destino
	dir := filepath.Dir(outputPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", fmt.Errorf("el directorio no existe: %s", dir)
	}

	// 4. Crear archivo DOT
	dotPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".dot"
	dotFile, err := os.Create(dotPath)
	if err != nil {
		return "", fmt.Errorf("error creando archivo dot: %v", err)
	}
	defer dotFile.Close()

	// 5. Recolectar información de las secciones del disco
	var sections []DiskSection
	mbrSize := int64(binary.Size(mbr))
	totalSize := mbr.MbrTamanio

	// Agregar MBR
	sections = append(sections, DiskSection{
		start:       0,
		size:        mbrSize,
		percentage:  float64(mbrSize) / float64(totalSize) * 100,
		sectionType: "MBR",
	})

	// Recolectar particiones
	var partitions []DiskSection
	for i, p := range mbr.MbrPartitions {
		if p.Size > 0 {
			partName := strings.TrimRight(string(p.Name[:]), " \x00")
			sectionType := ""
			switch p.Type {
			case PARTITION_PRIMARY:
				sectionType = "PRIMARY"
			case PARTITION_EXTENDED:
				sectionType = "EXTENDED"

				// Procesar particiones lógicas para la extendida
				var logicals []DiskSection
				currentPos := p.Start

				for currentPos != -1 {
					if _, err := file.Seek(currentPos, 0); err != nil {
						break
					}

					ebr := &EBR{}
					if err := binary.Read(file, binary.LittleEndian, ebr); err != nil {
						break
					}

					ebrSize := int64(binary.Size(ebr))

					// Agregar EBR
					logicals = append(logicals, DiskSection{
						start:       currentPos,
						size:        ebrSize,
						percentage:  float64(ebrSize) / float64(totalSize) * 100,
						sectionType: "EBR",
						index:       len(logicals) + 1,
					})

					// Agregar partición lógica si tiene tamaño
					if ebr.Size > 0 {
						logicName := strings.TrimRight(string(ebr.Name[:]), " \x00")
						logicals = append(logicals, DiskSection{
							start:       currentPos + ebrSize,
							size:        ebr.Size,
							percentage:  float64(ebr.Size) / float64(totalSize) * 100,
							sectionType: "LOGICAL",
							name:        logicName,
							fit:         ebr.Fit,
							status:      ebr.Status,
							index:       len(logicals) + 1,
						})
					}

					if ebr.Next <= 0 {
						break
					}
					currentPos = ebr.Next
				}

				// Agregar subsecciones a la partición extendida
				partitions = append(partitions, DiskSection{
					start:       p.Start,
					size:        p.Size,
					percentage:  float64(p.Size) / float64(totalSize) * 100,
					sectionType: sectionType,
					name:        partName,
					fit:         p.Fit,
					status:      p.Status,
					index:       i + 1,
					subSections: logicals,
				})
				continue
			}

			partitions = append(partitions, DiskSection{
				start:       p.Start,
				size:        p.Size,
				percentage:  float64(p.Size) / float64(totalSize) * 100,
				sectionType: sectionType,
				name:        partName,
				fit:         p.Fit,
				status:      p.Status,
				index:       i + 1,
			})
		}
	}

	// Ordenar particiones por posición
	sort.Slice(partitions, func(i, j int) bool {
		return partitions[i].start < partitions[j].start
	})

	// Agregar particiones a las secciones
	sections = append(sections, partitions...)

	// Ordenar todas las secciones por posición
	sort.Slice(sections, func(i, j int) bool {
		return sections[i].start < sections[j].start
	})

	// 6. Agregar espacios libres
	var allSections []DiskSection
	lastEnd := int64(0)

	for _, section := range sections {
		// Si hay un espacio antes de esta sección
		if section.start > lastEnd {
			freeSize := section.start - lastEnd
			allSections = append(allSections, DiskSection{
				start:       lastEnd,
				size:        freeSize,
				percentage:  float64(freeSize) / float64(totalSize) * 100,
				sectionType: "FREE",
			})
		}

		// Agregar la sección actual
		allSections = append(allSections, section)

		// Actualizar última posición
		sectionEnd := section.start + section.size
		if sectionEnd > lastEnd {
			lastEnd = sectionEnd
		}
	}

	// Verificar espacio libre al final
	if lastEnd < totalSize {
		freeSize := totalSize - lastEnd
		allSections = append(allSections, DiskSection{
			start:       lastEnd,
			size:        freeSize,
			percentage:  float64(freeSize) / float64(totalSize) * 100,
			sectionType: "FREE",
		})
	}

	// Ordenar nuevamente
	sort.Slice(allSections, func(i, j int) bool {
		return allSections[i].start < allSections[j].start
	})

	// 7. Generar el DOT
	var buffer strings.Builder
	buffer.WriteString(`digraph DiskStructure {
    rankdir=LR;
    node [shape=plaintext, fontname="Arial"];
    layout=dot;
    
    // Marco del reporte
    graph [bgcolor="#ffffff", pencolor="#333333", penwidth=2.0, style="rounded"];
    
    disk [
        label=<
        <TABLE BORDER="0" CELLBORDER="1" CELLSPACING="0" CELLPADDING="2">
            <TR>
                <TD COLSPAN="2" BGCOLOR="#B3D9FF" BORDER="1" COLOR="black"><B>DISCO: `)

	buffer.WriteString(filepath.Base(diskPath))
	buffer.WriteString(fmt.Sprintf(`</B> (Tamaño Total: %d bytes)</TD>
            </TR>
            <TR>
                <TD COLSPAN="2">
                    <TABLE BORDER="0" CELLBORDER="1" CELLSPACING="0" CELLPADDING="0">
                        <TR>`, mbr.MbrTamanio))

	// Generar las celdas para cada sección con bordes negros
	for _, section := range allSections {
		cellWidth := int(float64(section.percentage) * 8) // Ancho proporcional al porcentaje
		if cellWidth < 1 {
			cellWidth = 1 // Mínimo ancho
		}

		// Determinar color y etiqueta según tipo
		var bgcolor, label string
		switch section.sectionType {
		case "MBR":
			bgcolor = "#E6F3FF"
			label = fmt.Sprintf("MBR<BR/>%d bytes<BR/>%.2f%%", section.size, section.percentage)
		case "PRIMARY":
			bgcolor = "#E8F5E9"
			label = fmt.Sprintf("Primaria %d<BR/>%s<BR/>%d bytes<BR/>%.2f%%",
				section.index, section.name, section.size, section.percentage)
		case "EXTENDED":
			bgcolor = "#FFF3E0"

			// Para extendida, crear tabla interna con sus particiones lógicas y bordes negros
			label = fmt.Sprintf(`<TABLE BORDER="0" CELLBORDER="1" CELLSPACING="0" CELLPADDING="1" WIDTH="%d" COLOR="black">
                <TR><TD COLSPAN="%d" BGCOLOR="#FFB74D" BORDER="1" COLOR="black">Extendida %d: %s (%.2f%%)</TD></TR>
                <TR><TD>
                    <TABLE BORDER="0" CELLBORDER="1" CELLSPACING="0" CELLPADDING="0" COLOR="black">
                        <TR>`,
				cellWidth, len(section.subSections)*2, section.index, section.name, section.percentage)

			// Agregar celdas para cada subsección (EBRs y lógicas) con bordes negros
			for _, sub := range section.subSections {
				subWidth := 1
				subColor := "#F3E5F5"
				subLabel := ""

				switch sub.sectionType {
				case "EBR":
					subColor = "#CE93D8"
					subLabel = fmt.Sprintf("EBR %d<BR/>(%.2f%%)", sub.index, sub.percentage)
				case "LOGICAL":
					subColor = "#F3E5F5"
					subLabel = fmt.Sprintf("Lógica %d<BR/>%s<BR/>(%.2f%%)",
						sub.index, sub.name, sub.percentage)
				}

				label += fmt.Sprintf(`<TD BGCOLOR="%s" WIDTH="%d" BORDER="1" COLOR="black">%s</TD>`,
					subColor, subWidth, subLabel)
			}

			label += `</TR>
                    </TABLE>
                </TD></TR>
            </TABLE>`

		case "FREE":
			bgcolor = "#F5F5F5"
			label = fmt.Sprintf("Libre<BR/>%d bytes<BR/>%.2f%%", section.size, section.percentage)
		}

		// Usar COLOR="black" para el borde de cada celda
		buffer.WriteString(fmt.Sprintf(`<TD BGCOLOR="%s" WIDTH="%d" BORDER="1" COLOR="black">%s</TD>`,
			bgcolor, cellWidth, label))
	}

	buffer.WriteString(`
                        </TR>
                    </TABLE>
                </TD>
            </TR>
        </TABLE>
        >
    ];
}`)

	// Escribir al archivo
	if _, err := dotFile.WriteString(buffer.String()); err != nil {
		return "", fmt.Errorf("error escribiendo archivo dot: %v", err)
	}
	dotFile.Close()

	// 8. Generar imagen
	cmd := exec.Command("dot", "-Tpng", "-Gdpi=300", dotPath, "-o", outputPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error ejecutando Graphviz: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}

	// Verificar que la imagen se creó
	if _, err := os.Stat(outputPath); err != nil {
		return "", fmt.Errorf("archivo de reporte no generado: %v", err)
	}

	return outputPath, nil
}
