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

type PartitionInfo struct {
	index   int
	start   int64
	size    int64
	isLogic bool
	type_   byte
	content string
}

func GenerateMBRReport(diskPath, outputPath string) (string, error) {
	file, err := os.OpenFile(diskPath, os.O_RDONLY, 0666)
	if err != nil {
		return "", fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	mbr := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, mbr); err != nil {
		return "", fmt.Errorf("error leyendo MBR: %v", err)
	}

	dir := filepath.Dir(outputPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", fmt.Errorf("el directorio destino no existe: %s", dir)
	}

	dotPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".dot"
	dotFile, err := os.Create(dotPath)
	if err != nil {
		return "", fmt.Errorf("error creando archivo dot: %v", err)
	}
	defer dotFile.Close()

	var buffer strings.Builder
	var primaryPartitions []PartitionInfo
	extendedPartition := PartitionInfo{}
	var logicalPartitions []PartitionInfo
	hasExtended := false

	// Inicio del DOT con configuración para estructura vertical
	buffer.WriteString(`digraph MBR {
    rankdir=TB;
    ordering=out;
    splines=line;
    nodesep=0.05;     // Reducido para mayor compacidad
    ranksep=0.15;     // Reducido para mayor compacidad
    node [fontname="Arial", shape=plain, style="filled", fontsize=10];
    edge [arrowhead=none];
    newrank=true;     // Mejor control sobre rankings
    compound=true;
    
    // Marco alrededor de todo el reporte
    graph [bgcolor="#ffffff", pencolor="#333333", penwidth=2.0, style="rounded"];

`)

	// Nodo MBR con HTML-like label
	buffer.WriteString(fmt.Sprintf(`    mbr [
    fillcolor="#E6F3FF",
    label=<
    <TABLE BORDER="0" CELLBORDER="1" CELLSPACING="0" CELLPADDING="2" BGCOLOR="#E6F3FF">
        <TR><TD PORT="title" BGCOLOR="#B3D9FF"><B>MBR</B></TD></TR>
        <TR><TD ALIGN="LEFT">
            Tamaño: %d bytes<BR/>
            Fecha: %s<BR/>
            Signature: %d<BR/>
            Fit: %c
        </TD></TR>
    </TABLE>
    >
];

`, mbr.MbrTamanio, strings.TrimSpace(string(mbr.MbrFechaCreacion[:])),
		mbr.MbrDskSignature, mbr.DskFit))

	// Procesar y clasificar particiones
	for i, p := range mbr.MbrPartitions {
		if p.Size > 0 {
			partName := strings.TrimRight(string(p.Name[:]), " \x00")
			var color, headerColor, partType string

			switch p.Type {
			case PARTITION_PRIMARY:
				color = "#E8F5E9"
				headerColor = "#81C784"
				partType = "Primaria"

				primaryPartitions = append(primaryPartitions, PartitionInfo{
					index:   i + 1,
					start:   p.Start,
					size:    p.Size,
					isLogic: false,
					type_:   p.Type,
					content: fmt.Sprintf(`    part%d [
        label=<
        <TABLE BORDER="0" CELLBORDER="1" CELLSPACING="0" CELLPADDING="2" BGCOLOR="%s">
            <TR><TD PORT="title" BGCOLOR="%s"><B>Partición %d (%s)</B></TD></TR>
            <TR><TD ALIGN="LEFT">
                Estado: %s<BR/>
                Tipo: %c<BR/>
                Ajuste: %c<BR/>
                Inicio: %d<BR/>
                Tamaño: %d bytes<BR/>
                Nombre: %s
            </TD></TR>
        </TABLE>
        >
    ];
`, i+1, color, headerColor, i+1, partType,
						getStatusString(p.Status), p.Type, p.Fit, p.Start, p.Size, partName),
				})

			case PARTITION_EXTENDED:
				color = "#FFF3E0"
				headerColor = "#FFB74D"
				partType = "Extendida"
				hasExtended = true

				extendedPartition = PartitionInfo{
					index:   i + 1,
					start:   p.Start,
					size:    p.Size,
					isLogic: false,
					type_:   p.Type,
					content: fmt.Sprintf(`    part%d [
        label=<
        <TABLE BORDER="0" CELLBORDER="1" CELLSPACING="0" CELLPADDING="2" BGCOLOR="%s">
            <TR><TD PORT="title" BGCOLOR="%s"><B>Partición %d (%s)</B></TD></TR>
            <TR><TD ALIGN="LEFT">
                Estado: %s<BR/>
                Tipo: %c<BR/>
                Ajuste: %c<BR/>
                Inicio: %d<BR/>
                Tamaño: %d bytes<BR/>
                Nombre: %s
            </TD></TR>
        </TABLE>
        >
    ];
`, i+1, color, headerColor, i+1, partType,
						getStatusString(p.Status), p.Type, p.Fit, p.Start, p.Size, partName),
				}

				// Procesar particiones lógicas dentro de extendida
				logicals, _ := getLogicalPartitions(file, p.Start)
				for j, ebr := range logicals {
					if ebr.Size > 0 {
						logicName := strings.TrimRight(string(ebr.Name[:]), " \x00")

						// Formato mejorado para el campo "Siguiente"
						var siguienteStr string
						if ebr.Next == -1 {
							siguienteStr = "Siguiente: <FONT COLOR=\"#BB0000\">-1 (Fin)</FONT>"
						} else {
							siguienteStr = fmt.Sprintf("Siguiente: <FONT COLOR=\"#006600\">%d</FONT>", ebr.Next)
						}

						logicalPartitions = append(logicalPartitions, PartitionInfo{
							index:   j + 1,
							start:   ebr.Start,
							size:    ebr.Size,
							isLogic: true,
							type_:   PARTITION_LOGIC,
							content: fmt.Sprintf(`    logic%d_%d [
        label=<
        <TABLE BORDER="0" CELLBORDER="1" CELLSPACING="0" CELLPADDING="2" BGCOLOR="#F3E5F5">
            <TR><TD PORT="title" BGCOLOR="#CE93D8"><B>Partición Lógica %d</B></TD></TR>
            <TR><TD ALIGN="LEFT">
                Estado: %s<BR/>
                Ajuste: %c<BR/>
                Inicio: %d<BR/>
                Tamaño: %d bytes<BR/>
                Nombre: %s<BR/>
                %s
            </TD></TR>
        </TABLE>
        >
    ];
`, i+1, j+1, j+1, getStatusString(ebr.Status), ebr.Fit, ebr.Start, ebr.Size, logicName, siguienteStr),
						})
					}
				}
			}
		}
	}

	// Ordenar particiones primarias por posición de inicio
	sort.Slice(primaryPartitions, func(i, j int) bool {
		return primaryPartitions[i].start < primaryPartitions[j].start
	})

	// Escribir particiones primarias
	for _, p := range primaryPartitions {
		buffer.WriteString(p.content)
	}

	// Escribir cluster para extendida + lógicas si existe
	if hasExtended {
		// Apertura del cluster
		buffer.WriteString(`
    subgraph cluster_extended {
        style="filled,rounded";
        color="#FFB74D";
        penwidth=1.5;
        label="Partición Extendida y Lógicas";
        bgcolor="#FFFAF0";
        
`)
		// Agregar la partición extendida
		buffer.WriteString(extendedPartition.content)

		// Agregar las particiones lógicas
		for _, lp := range logicalPartitions {
			buffer.WriteString(lp.content)
		}

		// Si no hay lógicas, mostrar mensaje
		if len(logicalPartitions) == 0 {
			buffer.WriteString(`        empty_node [
            shape=none, 
            fillcolor="#F3E5F5",
            label="[No hay particiones lógicas]"
        ];
`)
		}

		// Cerrar el cluster
		buffer.WriteString("    }\n\n")
	}

	// Definir rankings para estructura vertical estricta
	buffer.WriteString("\n    // Estructura vertical\n")
	buffer.WriteString("    { rank=source; mbr }\n")

	// Rank para cada partición primaria, extendida y lógicas
	for i, p := range primaryPartitions {
		buffer.WriteString(fmt.Sprintf("    { rank=%d; part%d }\n", i+2, p.index))
	}

	if hasExtended {
		buffer.WriteString(fmt.Sprintf("    { rank=%d; part%d }\n",
			len(primaryPartitions)+2, extendedPartition.index))

		for i := range logicalPartitions {
			buffer.WriteString(fmt.Sprintf("    { rank=%d; logic%d_%d }\n",
				len(primaryPartitions)+3+i, extendedPartition.index, i+1))
		}

		if len(logicalPartitions) == 0 {
			buffer.WriteString("    { rank=sink; empty_node }\n")
		}
	}

	// Crear conexiones para estructura vertical
	buffer.WriteString("\n    // Conexiones verticales\n")
	buffer.WriteString("    edge [style=invis, weight=1000];\n")
	buffer.WriteString("    mbr:s")

	// Conectar mbr -> primarias
	for _, p := range primaryPartitions {
		buffer.WriteString(fmt.Sprintf(" -> part%d:n", p.index))
	}

	// Conectar última primaria -> extendida si existe
	if hasExtended {
		if len(primaryPartitions) > 0 {
			buffer.WriteString(fmt.Sprintf(" -> part%d:n", extendedPartition.index))
		} else {
			buffer.WriteString(fmt.Sprintf(" -> part%d:n", extendedPartition.index))
		}

		// Conectar extendida -> lógicas
		if len(logicalPartitions) > 0 {
			buffer.WriteString(fmt.Sprintf(" -> logic%d_1:n", extendedPartition.index))

			// Conectar lógicas entre sí
			for i := 1; i < len(logicalPartitions); i++ {
				buffer.WriteString(fmt.Sprintf(" -> logic%d_%d:n",
					extendedPartition.index, i+1))
			}
		} else {
			buffer.WriteString(" -> empty_node")
		}
	}

	buffer.WriteString(";\n")
	buffer.WriteString("}\n")

	// Escribir al archivo y cerrarlo
	if _, err := dotFile.WriteString(buffer.String()); err != nil {
		return "", fmt.Errorf("error escribiendo archivo dot: %v", err)
	}
	dotFile.Close()

	// Generar imagen
	cmd := exec.Command("dot", "-Tpng", "-Gdpi=300", dotPath, "-o", outputPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error ejecutando Graphviz: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}

	return outputPath, nil
}

func getStatusString(status byte) string {
	if status == PARTITION_MOUNTED {
		return "Montada"
	}
	return "No Montada"
}

func getLogicalPartitions(file *os.File, startPosition int64) ([]*EBR, error) {
	var logicals []*EBR
	currentPos := startPosition

	for currentPos != -1 {
		if _, err := file.Seek(currentPos, 0); err != nil {
			return logicals, nil
		}

		ebr := &EBR{}
		if err := binary.Read(file, binary.LittleEndian, ebr); err != nil {
			return logicals, nil
		}

		if ebr.Size > 0 {
			logicals = append(logicals, ebr)
		}

		currentPos = ebr.Next
	}

	return logicals, nil
}
