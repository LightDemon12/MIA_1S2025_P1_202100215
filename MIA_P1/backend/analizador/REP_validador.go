package analizador

import (
	"MIA_P1/backend/DiskManager"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

func HandleRep(c *gin.Context, comando string) {
	fmt.Printf("Procesando comando REP: %s\n", comando)

	// Usamos la nueva versión de AnalizarRep
	params, errores, valido, requiereConfirmacion, dirPath := AnalizarRep(comando)

	// Si hay errores, mostrarlos y salir
	if !valido || len(errores) > 0 {
		mensajeError := "Errores encontrados:\n"
		for _, err := range errores {
			mensajeError += fmt.Sprintf("- %s: %s\n", err.Parametro, err.Mensaje)
		}
		c.JSON(http.StatusOK, gin.H{
			"mensaje": mensajeError,
			"exito":   false,
		})
		return
	}

	// Si requiere confirmación para crear directorio, solicitar y salir
	if requiereConfirmacion {
		fmt.Printf("Solicitando confirmación para crear directorio: %s\n", dirPath)
		solicitarConfirmacionDirectorio(c, comando, dirPath)
		return
	}

	// Verificar que el ID de partición exista
	partitionInfo, err := DiskManager.GetMountedPartitionByID(params.ID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error: %s", err),
			"exito":   false,
		})
		return
	}

	// Generar el reporte según el tipo
	var reportPath string
	var reportErr error

	switch params.Name {
	case "mbr":
		reportPath, reportErr = DiskManager.GenerateMBRReport(partitionInfo.DiskPath, params.Path)
	case "disk":
		reportPath, reportErr = DiskManager.GenerateDiskReport(partitionInfo.DiskPath, params.Path)
	case "inode":
		// Obtener la posición de inicio de la partición
		startByte, posErr := DiskManager.GetPartitionStartByte(partitionInfo.DiskPath, partitionInfo.PartitionName)
		if posErr != nil {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": fmt.Sprintf("Error obteniendo la posición de la partición: %s", posErr),
				"exito":   false,
			})
			return
		}

		// Llamar a la función para generar reporte de inodos con la posición obtenida
		reportPath, reportErr = DiskManager.GenerateInodeReport(partitionInfo.DiskPath, startByte, params.Path)
	case "block", "bm_inode", "bm_block", "tree", "sb", "file", "ls":
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Reporte de tipo '%s' aún no implementado.", params.Name),
			"exito":   false,
		})
		return
	default:
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Tipo de reporte no reconocido: %s", params.Name),
			"exito":   false,
		})
		return
	}

	if reportErr != nil {
		fmt.Printf("Error generando reporte: %v\n", reportErr)
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error generando reporte: %s", reportErr),
			"exito":   false,
		})
		return
	}

	// Intentar abrir el reporte automáticamente
	openErr := DiskManager.OpenReport(reportPath)
	if openErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Reporte generado exitosamente en: %s (No se pudo abrir automáticamente: %s)",
				reportPath, openErr),
			"path":  reportPath,
			"exito": true,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Reporte generado exitosamente y abierto en: %s", reportPath),
			"path":    reportPath,
			"exito":   true,
		})
	}
}
