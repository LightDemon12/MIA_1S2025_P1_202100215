// handleRmgrp.go
package analizador

import (
	"MIA_P1/backend/DiskManager"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
)

// HandleRmgrp procesa el comando rmgrp
func HandleRmgrp(c *gin.Context, comando string) {
	// Verificar que haya una sesión activa
	if CurrentSession == nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": "Error: No hay una sesión activa. Debe iniciar sesión primero.",
			"exito":   false,
		})
		return
	}

	// Verificar que el usuario sea root (admin)
	if !CurrentSession.IsAdmin {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": "Error: Solo el usuario root puede eliminar grupos.",
			"exito":   false,
		})
		return
	}

	// Validar los parámetros del comando
	params, errores := ValidarRmgrp(comando)
	if len(errores) > 0 {
		mostrarErrores(c, errores)
		return
	}

	// Leer el archivo users.txt
	content, err := DiskManager.EXT2FileOperation(CurrentSession.PartitionID, "/users.txt", DiskManager.FILE_READ, "")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error al leer archivo de usuarios: %s", err),
			"exito":   false,
		})
		return
	}

	// Buscar el grupo a eliminar
	lines := strings.Split(content, "\n")
	groupFound := false
	originalGroupId := 0

	// Construir el nuevo contenido del archivo
	var newLines []string

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			// Preservar líneas vacías
			newLines = append(newLines, line)
			continue
		}

		parts := strings.Split(trimmedLine, ",")

		// Verificar si es un grupo (G)
		if len(parts) >= 3 && strings.TrimSpace(parts[1]) == "G" {
			groupName := strings.TrimSpace(parts[2])

			// Si encontramos el grupo a eliminar
			if groupName == params.Name {
				groupId, _ := strconv.Atoi(strings.TrimSpace(parts[0]))

				// No podemos eliminar el grupo root (ID=1)
				if groupName == "root" {
					c.JSON(http.StatusOK, gin.H{
						"mensaje": "Error: No se puede eliminar el grupo root",
						"exito":   false,
					})
					return
				}

				// Verificar que el grupo no esté ya eliminado (ID=0)
				if groupId == 0 {
					c.JSON(http.StatusOK, gin.H{
						"mensaje": fmt.Sprintf("Error: El grupo '%s' ya está eliminado", params.Name),
						"exito":   false,
					})
					return
				}

				// Guardar el ID original para la respuesta
				originalGroupId = groupId

				// Crear la nueva línea con ID=0
				parts[0] = "0"
				newLine := strings.Join(parts, ",")
				newLines = append(newLines, newLine)

				groupFound = true
			} else {
				// Mantener la línea sin cambios
				newLines = append(newLines, line)
			}
		} else {
			// No es un grupo, mantener la línea sin cambios
			newLines = append(newLines, line)
		}
	}

	// Verificar si el grupo fue encontrado
	if !groupFound {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error: El grupo '%s' no existe", params.Name),
			"exito":   false,
		})
		return
	}

	// Unir las líneas y escribir el archivo actualizado
	updatedContent := strings.Join(newLines, "\n")
	if !strings.HasSuffix(updatedContent, "\n") {
		updatedContent += "\n"
	}

	_, err = DiskManager.EXT2FileOperation(CurrentSession.PartitionID, "/users.txt", DiskManager.FILE_WRITE, updatedContent)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error al actualizar archivo de usuarios: %s", err),
			"exito":   false,
		})
		return
	}

	// Responder con éxito
	c.JSON(http.StatusOK, gin.H{
		"mensaje": fmt.Sprintf("Grupo '%s' (ID original: %d) eliminado exitosamente", params.Name, originalGroupId),
		"exito":   true,
	})
}
