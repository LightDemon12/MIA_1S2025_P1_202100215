package DiskManager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BmInodeReporter genera un reporte del bitmap de inodos
func BmInodeReporter(id, path string) (bool, string) {
	// 1. Encontrar la partición montada
	mountedPartition, err := findMountedPartitionById(id)
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
	startByte, _, err := getPartitionDetails(file, mountedPartition)
	if err != nil {
		return false, fmt.Sprintf("Error al obtener detalles de la partición: %s", err)
	}

	// 4. Leer el superbloque
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en el superbloque: %s", err)
	}

	superblock, err := readSuperBlockFromDisc(file)
	if err != nil {
		return false, fmt.Sprintf("Error al leer el superbloque: %s", err)
	}

	// 5. Posicionarse en el bitmap de inodos
	bmInodePos := startByte + int64(superblock.SBmInodeStart)
	_, err = file.Seek(bmInodePos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en el bitmap de inodos: %s", err)
	}

	// 6. Leer el bitmap de inodos
	inodeBitmapSize := (superblock.SInodesCount + 7) / 8 // Tamaño en bytes del bitmap
	inodeBitmap := make([]byte, inodeBitmapSize)
	bytesRead, err := file.Read(inodeBitmap)
	if err != nil {
		return false, fmt.Sprintf("Error al leer el bitmap de inodos: %s", err)
	}

	if bytesRead < int(inodeBitmapSize) {
		return false, fmt.Sprintf("Se esperaban %d bytes pero se leyeron %d bytes", inodeBitmapSize, bytesRead)
	}

	// 7. Generar el reporte
	var report strings.Builder

	// Encabezado del reporte
	report.WriteString("===========================================================\n")
	report.WriteString(fmt.Sprintf("           REPORTE DE BITMAP DE INODOS - PARTICIÓN %s\n", mountedPartition.ID))
	report.WriteString("===========================================================\n\n")

	report.WriteString(fmt.Sprintf("- Partición montada: %s\n", mountedPartition.ID))
	report.WriteString(fmt.Sprintf("- Cantidad total de inodos: %d\n", superblock.SInodesCount))
	report.WriteString(fmt.Sprintf("- Tamaño del bitmap: %d bytes\n\n", inodeBitmapSize))

	report.WriteString("BITMAP DE INODOS (0 = libre, 1 = ocupado)\n")
	report.WriteString("-----------------------------------------------------------\n")

	// Contador para inodos en uso
	usedInodeCount := 0

	// Mostrar 20 registros por línea
	for i := 0; i < int(superblock.SInodesCount); i++ {
		bytePos := i / 8
		bitPos := i % 8

		// Verificar límites del array
		if bytePos >= len(inodeBitmap) {
			break
		}

		// Determinar si el bit está encendido (1) o apagado (0)
		var bit byte
		if (inodeBitmap[bytePos] & (1 << bitPos)) != 0 {
			bit = 1
			usedInodeCount++
		} else {
			bit = 0
		}

		// Agregar el bit al reporte
		report.WriteString(fmt.Sprintf("%d", bit))

		// Agregar separador cada 20 registros
		if (i+1)%20 == 0 {
			report.WriteString("\n")
		} else {
			report.WriteString(" ")
		}
	}

	// Asegurar que termine con nueva línea
	if superblock.SInodesCount%20 != 0 {
		report.WriteString("\n")
	}

	// Agregar estadísticas finales
	report.WriteString("\n-----------------------------------------------------------\n")
	report.WriteString(fmt.Sprintf("Total inodos utilizados: %d (%.2f%%)\n",
		usedInodeCount, float64(usedInodeCount)*100.0/float64(superblock.SInodesCount)))
	report.WriteString(fmt.Sprintf("Total inodos libres: %d (%.2f%%)\n",
		int(superblock.SInodesCount)-usedInodeCount,
		(float64(superblock.SInodesCount)-float64(usedInodeCount))*100.0/float64(superblock.SInodesCount)))

	// 8. Guardar el reporte en un archivo de texto
	outputPath := path

	// Quitar todas las extensiones existentes
	basePath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath))

	// Añadir la extensión .txt
	outputPath = basePath + ".txt"

	// Guardar el reporte
	err = os.WriteFile(outputPath, []byte(report.String()), 0644)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir el archivo de reporte: %s", err)
	}

	return true, fmt.Sprintf("Reporte de bitmap de inodos generado exitosamente en: %s", outputPath)
}
