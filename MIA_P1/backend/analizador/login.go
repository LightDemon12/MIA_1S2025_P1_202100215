// handleLogin.go
package analizador

import (
	"MIA_P1/backend/DiskManager" // Ajusta la ruta según tu proyecto
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

// Variable global para mantener el estado de la sesión
var (
	CurrentSession *SessionInfo
)

// SessionInfo almacena información de la sesión activa
type SessionInfo struct {
	Username    string
	PartitionID string
	IsAdmin     bool
	UserGroup   string
}

// HandleLogin procesa el comando login
func HandleLogin(c *gin.Context, comando string) {
	// Validar los parámetros del comando
	params, errores := ValidarLogin(comando)
	if len(errores) > 0 {
		mostrarErrores(c, errores)
		return
	}

	// Verificar que no haya una sesión activa
	if CurrentSession != nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error: Ya hay una sesión activa para el usuario %s. Debe cerrar sesión primero.", CurrentSession.Username),
			"exito":   false,
		})
		return
	}

	// Verificar que la partición esté montada
	exists, err := DiskManager.IsPartitionMounted(params.ID)
	if err != nil || !exists {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error: La partición con ID %s no está montada", params.ID),
			"exito":   false,
		})
		return
	}

	// Leer el archivo users.txt para verificar credenciales
	content, err := DiskManager.EXT2FileOperation(params.ID, "/users.txt", DiskManager.FILE_READ, "")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error al leer archivo de usuarios: %s", err),
			"exito":   false,
		})
		return
	}

	// Verificar credenciales
	isValid, isAdmin, userGroup := validateCredentials(content, params.User, params.Pass)

	if !isValid {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": "Error: Credenciales incorrectas. Verifique usuario y contraseña.",
			"exito":   false,
		})
		return
	}

	// Crear sesión
	CurrentSession = &SessionInfo{
		Username:    params.User,
		PartitionID: params.ID,
		IsAdmin:     isAdmin,
		UserGroup:   userGroup,
	}

	// Responder con éxito
	c.JSON(http.StatusOK, gin.H{
		"mensaje": fmt.Sprintf("Sesión iniciada correctamente. Usuario: %s, Partición: %s", params.User, params.ID),
		"exito":   true,
		"usuario": params.User,
		"admin":   isAdmin,
		"grupo":   userGroup,
	})
}

// validateCredentials verifica si las credenciales son válidas usando el archivo users.txt
// Devuelve: (es válido, es admin, grupo)
func validateCredentials(usersContent, username, password string) (bool, bool, string) {
	lines := strings.Split(usersContent, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Formato esperado: 1, G, root (para grupos)
		// Formato esperado: 1, U, root, root, 123 (para usuarios)
		parts := strings.Split(line, ",")

		// Verificar si es un usuario (U)
		if len(parts) >= 5 && strings.TrimSpace(parts[1]) == "U" {
			// Verificar username
			userInFile := strings.TrimSpace(parts[2])
			groupInFile := strings.TrimSpace(parts[3])
			passInFile := strings.TrimSpace(parts[4])

			// La comparación debe ser exacta (case sensitive)
			if userInFile == username && passInFile == password {
				// Verificar si es admin (root)
				isAdmin := (userInFile == "root")
				return true, isAdmin, groupInFile
			}
		}
	}

	return false, false, ""
}
