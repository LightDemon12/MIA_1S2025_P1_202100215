package DiskManager

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileReporter genera un reporte del contenido de un archivo dentro del sistema EXT2
func FileReporter(id string, reportPath string, filePath string) (bool, string) {
	// Verificar que la partición exista (sin guardar la variable si no se usa)
	_, err := FindMountedPartitionById(id)
	if err != nil {
		return false, fmt.Sprintf("Error: %v", err)
	}

	// Leer el contenido del archivo usando EXT2FileOperation
	content, err := EXT2FileOperation(id, filePath, FILE_READ, "")
	if err != nil {
		return false, fmt.Sprintf("Error leyendo archivo '%s': %v", filePath, err)
	}

	// Asegurar que el directorio del reporte existe
	reportDir := filepath.Dir(reportPath)
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		return false, fmt.Sprintf("Error creando directorio para reporte: %v", err)
	}

	// Escribir el contenido al archivo de reporte
	err = os.WriteFile(reportPath, []byte(content), 0644)
	if err != nil {
		return false, fmt.Sprintf("Error escribiendo reporte: %v", err)
	}
	// Abrir el archivo de texto automáticamente con la función existente
	if err := OpenTextFile(reportPath); err != nil {
		// No retornamos error si falla la apertura, solo lo registramos
		fmt.Printf("Advertencia: No se pudo abrir el archivo automáticamente: %v\n", err)
	}
	return true, fmt.Sprintf("Reporte file generado exitosamente en: %s", reportPath)
}
