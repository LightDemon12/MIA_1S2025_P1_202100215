package analizador

import (
	"MIA_P1/backend/DiskManager"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"path/filepath"
	"strings"
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
	var isTextReport bool = false

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
	case "block":
		// Llamar a la función de reporte de bloques directamente con el ID
		success, mensaje := DiskManager.BlockReporter(params.ID, params.Path)
		if !success {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": mensaje,
				"exito":   false,
			})
			return
		}
		reportPath = params.Path
		reportErr = nil // Asignamos nil para que no entre en el siguiente if de error
	case "bm_inode":
		// Implementando el reporte de bitmap de inodos
		success, mensaje := DiskManager.BmInodeReporter(params.ID, params.Path)
		if !success {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": mensaje,
				"exito":   false,
			})
			return
		}
		reportPath = params.Path
		reportErr = nil
		isTextReport = true // Marcamos como reporte de texto
	case "bm_block":
		// Implementando el reporte de bitmap de bloques
		success, mensaje := DiskManager.BmBlockReporter(params.ID, params.Path)
		if !success {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": mensaje,
				"exito":   false,
			})
			return
		}

		// Extraer la ruta del archivo del mensaje (que contiene la ruta correcta con extensión)
		parts := strings.Split(mensaje, ": ")
		if len(parts) > 1 {
			reportPath = parts[len(parts)-1] // Obtener la última parte que contiene la ruta
		} else {
			reportPath = params.Path
		}

		reportErr = nil
		isTextReport = true
	case "sb":
		// Implementando el reporte de superbloque
		success, mensaje := DiskManager.SbReporter(params.ID, params.Path)
		if !success {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": mensaje,
				"exito":   false,
			})
			return
		}

		// Extraer la ruta del archivo del mensaje
		parts := strings.Split(mensaje, ": ")
		if len(parts) > 1 {
			reportPath = parts[len(parts)-1]
		} else {
			reportPath = params.Path
		}

		reportErr = nil
	case "tree":
		// Implementando el reporte de árbol de directorios
		success, mensaje := DiskManager.TreeReporter(params.ID, params.Path)
		if !success {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": mensaje,
				"exito":   false,
			})
			return
		}
		reportPath = params.Path
		reportErr = nil
	case "file":
		if params.PathFileLS == "" {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": "Error: Se requiere el parámetro path_file_ls para el reporte file",
				"exito":   false,
			})
			return
		}

		// Forzar extensión .txt para el reporte file
		if !strings.HasSuffix(params.Path, ".txt") {
			params.Path = strings.TrimSuffix(params.Path, filepath.Ext(params.Path)) + ".txt"
		}

		success, mensaje := DiskManager.FileReporter(params.ID, params.Path, params.PathFileLS)
		if !success {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": mensaje,
				"exito":   false,
			})
			return
		}
		reportPath = params.Path
		reportErr = nil
		isTextReport = true
	case "ls":
		if params.PathFileLS == "" {
			params.PathFileLS = "/" // Default to root if not specified
		}

		// Ensure path has .png extension for graphical report
		if !strings.HasSuffix(params.Path, ".png") {
			params.Path = strings.TrimSuffix(params.Path, filepath.Ext(params.Path)) + ".png"
		}

		success, mensaje := DiskManager.LSReporter(params.ID, params.Path, params.PathFileLS)
		if !success {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": mensaje,
				"exito":   false,
			})
			return
		}
		reportPath = params.Path
		reportErr = nil
		isTextReport = false
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

	// Abrir el reporte según su tipo
	var openErr error
	if isTextReport {
		// Para reportes de texto, simplemente informamos que ha sido generado
		fmt.Printf("Reporte de texto generado: %s\n", reportPath)
		openErr = nil
	} else {
		// Para reportes gráficos, intentamos abrirlos
		openErr = DiskManager.OpenReport(reportPath)
	}

	if openErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Reporte generado exitosamente en: %s (No se pudo abrir automáticamente: %s)",
				reportPath, openErr),
			"path":  reportPath,
			"exito": true,
		})
	} else {
		// Mensaje específico según tipo de reporte
		var mensaje string
		if isTextReport {
			mensaje = fmt.Sprintf("Reporte de texto generado exitosamente en: %s", reportPath)
		} else {
			mensaje = fmt.Sprintf("Reporte generado exitosamente y abierto en: %s", reportPath)
		}

		c.JSON(http.StatusOK, gin.H{
			"mensaje": mensaje,
			"path":    reportPath,
			"exito":   true,
		})
	}
}
