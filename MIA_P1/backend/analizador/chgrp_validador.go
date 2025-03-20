// handleChgrp.go - con validación adicional
package analizador

import (
	"MIA_P1/backend/DiskManager"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
)

// HandleChgrp procesa el comando chgrp
func HandleChgrp(c *gin.Context, comando string) {
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
			"mensaje": "Error: Solo el usuario root puede cambiar el grupo de un usuario.",
			"exito":   false,
		})
		return
	}

	// Validar los parámetros del comando
	params, errores := ValidarChgrp(comando)
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

	// Verificar si el usuario existe y si el grupo existe
	lines := strings.Split(content, "\n")
	userFound := false
	groupExists := false
	groupId := 0
	var oldGroup string // Declaración para almacenar el grupo anterior

	// Primero verificar si el grupo existe y está activo
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		parts := strings.Split(trimmedLine, ",")

		// Verificar si es un grupo (G)
		if len(parts) >= 3 && strings.TrimSpace(parts[1]) == "G" {
			groupName := strings.TrimSpace(parts[2])
			currentGroupId, _ := strconv.Atoi(strings.TrimSpace(parts[0]))

			// Verificar si el grupo existe y está activo (ID > 0)
			if groupName == params.Group && currentGroupId > 0 {
				groupExists = true
				groupId = currentGroupId
				break
			}
		}
	}

	// Verificar que el grupo exista
	if !groupExists {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error: El grupo '%s' no existe o está eliminado", params.Group),
			"exito":   false,
		})
		return
	}

	// Construir las nuevas líneas del archivo
	var newLines []string

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			// Mantener líneas vacías
			newLines = append(newLines, line)
			continue
		}

		parts := strings.Split(trimmedLine, ",")

		// Verificar si es un usuario (U) y es el usuario que buscamos
		if len(parts) >= 5 && strings.TrimSpace(parts[1]) == "U" {
			userName := strings.TrimSpace(parts[2])
			userId, _ := strconv.Atoi(strings.TrimSpace(parts[0]))

			if userName == params.User {
				// No podemos modificar usuarios eliminados (ID=0)
				if userId == 0 {
					c.JSON(http.StatusOK, gin.H{
						"mensaje": fmt.Sprintf("Error: El usuario '%s' está eliminado", params.User),
						"exito":   false,
					})
					return
				}

				// Obtener el grupo actual del usuario
				oldGroup = strings.TrimSpace(parts[3])

				// Validar si el usuario ya está en el grupo solicitado
				if oldGroup == params.Group {
					c.JSON(http.StatusOK, gin.H{
						"mensaje": fmt.Sprintf("El usuario '%s' ya pertenece al grupo '%s'", params.User, params.Group),
						"exito":   true,
					})
					return
				}

				// Actualizar el grupo del usuario
				parts[0] = fmt.Sprintf("%d", groupId) // Actualizar ID según el nuevo grupo
				parts[3] = params.Group               // Actualizar nombre del grupo

				newLines = append(newLines, strings.Join(parts, ","))
				userFound = true
			} else {
				// No es el usuario que buscamos, mantener línea sin cambios
				newLines = append(newLines, line)
			}
		} else {
			// No es un usuario, mantener la línea sin cambios
			newLines = append(newLines, line)
		}
	}

	// Verificar si el usuario existe
	if !userFound {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error: El usuario '%s' no existe", params.User),
			"exito":   false,
		})
		return
	}

	// Unir las líneas y escribir el archivo actualizado
	updatedContent := strings.Join(newLines, "\n")
	if !strings.HasSuffix(updatedContent, "\n") {
		updatedContent += "\n"
	}

	// Escribir el archivo actualizado
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
		"mensaje": fmt.Sprintf("Grupo del usuario '%s' cambiado exitosamente de '%s' a '%s'", params.User, oldGroup, params.Group),
		"exito":   true,
	})
}
