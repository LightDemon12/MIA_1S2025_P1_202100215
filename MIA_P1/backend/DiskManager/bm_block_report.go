package DiskManager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BmBlockReporter genera un reporte del bitmap de bloques
func BmBlockReporter(id, path string) (bool, string) {
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

	// 5. Posicionarse en el bitmap de bloques
	bmBlockPos := startByte + int64(superblock.SBmBlockStart)
	_, err = file.Seek(bmBlockPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en el bitmap de bloques: %s", err)
	}

	// 6. Leer el bitmap de bloques
	blockBitmapSize := (superblock.SBlocksCount + 7) / 8 // Tamaño en bytes del bitmap
	blockBitmap := make([]byte, blockBitmapSize)
	bytesRead, err := file.Read(blockBitmap)
	if err != nil {
		return false, fmt.Sprintf("Error al leer el bitmap de bloques: %s", err)
	}

	if bytesRead < int(blockBitmapSize) {
		return false, fmt.Sprintf("Se esperaban %d bytes pero se leyeron %d bytes", blockBitmapSize, bytesRead)
	}

	// 7. Generar el reporte
	var report strings.Builder

	// Encabezado del reporte
	report.WriteString("===========================================================\n")
	report.WriteString(fmt.Sprintf("           REPORTE DE BITMAP DE BLOQUES - PARTICIÓN %s\n", mountedPartition.ID))
	report.WriteString("===========================================================\n\n")

	report.WriteString(fmt.Sprintf("- Partición montada: %s\n", mountedPartition.ID))
	report.WriteString(fmt.Sprintf("- Cantidad total de bloques: %d\n", superblock.SBlocksCount))
	report.WriteString(fmt.Sprintf("- Tamaño del bitmap: %d bytes\n\n", blockBitmapSize))

	report.WriteString("BITMAP DE BLOQUES (0 = libre, 1 = ocupado)\n")
	report.WriteString("-----------------------------------------------------------\n")

	// Contador para bloques en uso
	usedBlockCount := 0

	// Mostrar 20 registros por línea
	for i := 0; i < int(superblock.SBlocksCount); i++ {
		bytePos := i / 8
		bitPos := i % 8

		// Verificar límites del array
		if bytePos >= len(blockBitmap) {
			break
		}

		// Determinar si el bit está encendido (1) o apagado (0)
		var bit byte
		if (blockBitmap[bytePos] & (1 << bitPos)) != 0 {
			bit = 1
			usedBlockCount++
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
	if superblock.SBlocksCount%20 != 0 {
		report.WriteString("\n")
	}

	// Agregar estadísticas finales
	report.WriteString("\n-----------------------------------------------------------\n")
	report.WriteString(fmt.Sprintf("Total bloques utilizados: %d (%.2f%%)\n",
		usedBlockCount, float64(usedBlockCount)*100.0/float64(superblock.SBlocksCount)))
	report.WriteString(fmt.Sprintf("Total bloques libres: %d (%.2f%%)\n",
		int(superblock.SBlocksCount)-usedBlockCount,
		(float64(superblock.SBlocksCount)-float64(usedBlockCount))*100.0/float64(superblock.SBlocksCount)))

	// 8. Asegurarnos de tener un nombre de archivo con extensión .txt
	outputPath := path

	// Quitar todas las extensiones existentes
	basePath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath))

	// Añadir la extensión .txt
	outputPath = basePath + ".txt"

	// Guardar el reporte en un archivo de texto
	err = os.WriteFile(outputPath, []byte(report.String()), 0644)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir el archivo de reporte: %s", err)
	}

	return true, fmt.Sprintf("Reporte de bitmap de bloques generado exitosamente en: %s", outputPath)
}
