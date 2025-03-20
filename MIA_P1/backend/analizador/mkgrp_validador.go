// handleMkgrp.go
package analizador

import (
	"MIA_P1/backend/DiskManager"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
)

// HandleMkgrp procesa el comando mkgrp
func HandleMkgrp(c *gin.Context, comando string) {
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
			"mensaje": "Error: Solo el usuario root puede crear grupos.",
			"exito":   false,
		})
		return
	}

	// Validar los parámetros del comando
	params, errores := ValidarMkgrp(comando)
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

	// Variables para el proceso
	lines := strings.Split(content, "\n")
	lastGroupId := 1 // El primer grupo es root con ID 1
	var deletedGroupId int = -1
	var newLines []string
	groupExists := false

	// Procesar cada línea
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			newLines = append(newLines, line)
			continue
		}

		parts := strings.Split(trimmedLine, ",")

		// Verificar si es un grupo (G)
		if len(parts) >= 3 && strings.TrimSpace(parts[1]) == "G" {
			groupName := strings.TrimSpace(parts[2])
			groupId, _ := strconv.Atoi(strings.TrimSpace(parts[0]))

			// La comparación debe ser exacta (case sensitive)
			if groupName == params.Name {
				// Si el grupo existe y está activo (ID > 0), es un error
				if groupId > 0 {
					c.JSON(http.StatusOK, gin.H{
						"mensaje": fmt.Sprintf("Error: El grupo '%s' ya existe", params.Name),
						"exito":   false,
					})
					return
				}

				// Si está eliminado (ID = 0), lo restauraremos
				deletedGroupId = groupId
				groupExists = true
			}

			// Mantener registro del último ID de grupo activo
			if groupId > lastGroupId {
				lastGroupId = groupId
			}
		}
	}

	// Manejar grupo eliminado o crear nuevo
	var newGroupId int
	var action string

	if groupExists {
		// Buscar el ID original en las líneas existentes
		for i, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if trimmedLine == "" {
				continue
			}

			parts := strings.Split(trimmedLine, ",")
			if len(parts) >= 3 && strings.TrimSpace(parts[1]) == "G" {
				groupName := strings.TrimSpace(parts[2])
				groupId, _ := strconv.Atoi(strings.TrimSpace(parts[0]))

				// Encontramos el grupo eliminado
				if groupName == params.Name && groupId == 0 {
					// Buscar el ID original más alto que se usó para este grupo
					for _, otherLine := range lines {
						otherParts := strings.Split(strings.TrimSpace(otherLine), ",")
						if len(otherParts) >= 3 && strings.TrimSpace(otherParts[1]) == "G" {
							otherName := strings.TrimSpace(otherParts[2])
							otherId, _ := strconv.Atoi(strings.TrimSpace(otherParts[0]))

							// Si encontramos una entrada duplicada con ID mayor (histórica)
							if otherName == params.Name && otherId > 0 {
								deletedGroupId = otherId
							}
						}
					}

					// Si encontramos un ID histórico, usarlo; si no, generar uno nuevo
					if deletedGroupId > 0 {
						newGroupId = deletedGroupId
					} else {
						newGroupId = lastGroupId + 1
					}

					// Restaurar el grupo (cambiar ID de 0 a newGroupId)
					parts[0] = fmt.Sprintf("%d", newGroupId)
					lines[i] = strings.Join(parts, ",")
					action = "restaurado"
					break
				}
			}
		}
	} else {
		// Crear nuevo grupo (siguiente ID)
		newGroupId = lastGroupId + 1
		newGroupLine := fmt.Sprintf("%d, G, %s", newGroupId, params.Name)
		lines = append(lines, newGroupLine)
		action = "creado"
	}

	// Unir las líneas y escribir el archivo actualizado
	updatedContent := strings.Join(lines, "\n")
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
		"mensaje": fmt.Sprintf("Grupo '%s' %s exitosamente con ID %d", params.Name, action, newGroupId),
		"exito":   true,
	})
}
