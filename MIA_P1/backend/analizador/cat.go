// handleCat.go
package analizador

import (
	"MIA_P1/backend/DiskManager"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

// HandleCat procesa el comando cat
func HandleCat(c *gin.Context, comando string) {
	// Verificar que haya una sesión activa
	if CurrentSession == nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": "Error: No hay una sesión activa. Debe iniciar sesión primero.",
			"exito":   false,
		})
		return
	}

	// Validar los parámetros del comando
	params, errores := ValidarCat(comando)
	if len(errores) > 0 {
		mostrarErrores(c, errores)
		return
	}

	// Procesar cada archivo
	var resultContent strings.Builder
	var filesProcessed int

	for i, filePath := range params.Files {
		// Verificar existencia del archivo
		exists, pathType, err := DiskManager.ValidateEXT2Path(CurrentSession.PartitionID, filePath)
		if err != nil || !exists {
			errMsg := fmt.Sprintf("Error: El archivo '%s' no existe", filePath)
			if err != nil {
				errMsg = fmt.Sprintf("Error al verificar archivo '%s': %s", filePath, err)
			}
			c.JSON(http.StatusOK, gin.H{
				"mensaje": errMsg,
				"exito":   false,
			})
			return
		}

		if pathType != "archivo" {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": fmt.Sprintf("Error: '%s' no es un archivo regular", filePath),
				"exito":   false,
			})
			return
		}

		// Verificar permisos de lectura
		canRead, err := hasReadPermission(CurrentSession.PartitionID, filePath)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": fmt.Sprintf("Error al verificar permisos: %s", err),
				"exito":   false,
			})
			return
		}

		if !canRead {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": fmt.Sprintf("Error: No tiene permisos de lectura para '%s'", filePath),
				"exito":   false,
			})
			return
		}

		// Leer contenido del archivo
		content, err := DiskManager.EXT2FileOperation(CurrentSession.PartitionID, filePath, DiskManager.FILE_READ, "")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": fmt.Sprintf("Error al leer archivo '%s': %s", filePath, err),
				"exito":   false,
			})
			return
		}

		// Añadir contenido a la respuesta
		if i > 0 {
			resultContent.WriteString("\n") // Separar archivos con salto de línea
		}
		resultContent.WriteString(content)
		filesProcessed++
	}

	// Responder con éxito y el contenido concatenado
	c.JSON(http.StatusOK, gin.H{
		"mensaje":   fmt.Sprintf("Lectura exitosa de %d archivo(s):\n\n%s", filesProcessed, resultContent.String()),
		"contenido": resultContent.String(),
		"exito":     true,
	})
}

// hasReadPermission verifica si el usuario actual tiene permisos de lectura para un archivo
func hasReadPermission(partitionID, filePath string) (bool, error) {
	// Siempre retorna true para permitir la lectura de cualquier archivo
	return true, nil
}
