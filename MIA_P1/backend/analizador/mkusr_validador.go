// handleMkusr.go
package analizador

import (
	"MIA_P1/backend/DiskManager"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
)

// HandleMkusr procesa el comando mkusr
func HandleMkusr(c *gin.Context, comando string) {
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
			"mensaje": "Error: Solo el usuario root puede crear usuarios.",
			"exito":   false,
		})
		return
	}

	// Validar los parámetros del comando
	params, errores := ValidarMkusr(comando)
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
	userExists := false
	userIsDeleted := false
	originalUserGroupId := 0
	groupExists := false
	groupId := 0

	// Buscar si el grupo existe y si el usuario existe (activo o eliminado)
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		parts := strings.Split(trimmedLine, ",")

		// Verificar si es un usuario (U)
		if len(parts) >= 5 && strings.TrimSpace(parts[1]) == "U" {
			userName := strings.TrimSpace(parts[2])

			// Verificar si el usuario ya existe (en cualquier grupo)
			if userName == params.User {
				userId, _ := strconv.Atoi(strings.TrimSpace(parts[0]))

				if userId > 0 {
					// Usuario activo
					userExists = true
					break
				} else {
					// Usuario eliminado (ID = 0)
					userIsDeleted = true

					// Buscar el ID original del usuario en registros históricos
					for _, historyLine := range lines {
						historyParts := strings.Split(strings.TrimSpace(historyLine), ",")
						if len(historyParts) >= 5 && strings.TrimSpace(historyParts[1]) == "U" {
							historyName := strings.TrimSpace(historyParts[2])
							historyId, _ := strconv.Atoi(strings.TrimSpace(historyParts[0]))

							if historyName == params.User && historyId > 0 && historyId > originalUserGroupId {
								originalUserGroupId = historyId
							}
						}
					}
				}
			}
		}

		// Verificar si es un grupo (G)
		if len(parts) >= 3 && strings.TrimSpace(parts[1]) == "G" {
			groupName := strings.TrimSpace(parts[2])
			currentGroupId, _ := strconv.Atoi(strings.TrimSpace(parts[0]))

			// Verificar si el grupo existe y está activo (ID > 0)
			if groupName == params.Group && currentGroupId > 0 {
				groupExists = true
				groupId = currentGroupId
			}
		}
	}

	// Verificar que el usuario no exista (activo)
	if userExists {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error: El usuario '%s' ya existe", params.User),
			"exito":   false,
		})
		return
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
	userRestored := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			newLines = append(newLines, line) // Mantener líneas vacías
			continue
		}

		parts := strings.Split(trimmedLine, ",")

		// Si encontramos el usuario eliminado, restaurarlo
		if userIsDeleted && !userRestored &&
			len(parts) >= 5 && strings.TrimSpace(parts[1]) == "U" &&
			strings.TrimSpace(parts[2]) == params.User &&
			strings.TrimSpace(parts[0]) == "0" {

			// Decidir qué ID usar para restaurar
			var restoredId int
			if originalUserGroupId > 0 {
				// Usar el ID histórico más alto
				restoredId = originalUserGroupId
			} else {
				// Usar el ID del grupo actual
				restoredId = groupId
			}

			// Crear la línea restaurada
			parts[0] = fmt.Sprintf("%d", restoredId)
			parts[3] = params.Group // Actualizar grupo
			parts[4] = params.Pass  // Actualizar contraseña

			newLine := strings.Join(parts, ",")
			newLines = append(newLines, newLine)
			userRestored = true
		} else {
			newLines = append(newLines, line)
		}
	}

	// Si el usuario no existe o no se restauró, añadir nuevo
	if !userIsDeleted || !userRestored {
		// Determinar el ID para el nuevo usuario (usar el mismo ID del grupo)
		newUserLine := fmt.Sprintf("%d, U, %s, %s, %s", groupId, params.User, params.Group, params.Pass)
		newLines = append(newLines, newUserLine)
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
	var mensaje string
	if userIsDeleted && userRestored {
		mensaje = fmt.Sprintf("Usuario '%s' restaurado exitosamente en el grupo '%s'", params.User, params.Group)
	} else {
		mensaje = fmt.Sprintf("Usuario '%s' creado exitosamente en el grupo '%s'", params.User, params.Group)
	}

	c.JSON(http.StatusOK, gin.H{
		"mensaje": mensaje,
		"exito":   true,
	})
}
