// handleLogin.go
package analizador

import (
	"MIA_P1/backend/DiskManager" // Ajusta la ruta según tu proyecto
	"MIA_P1/backend/common"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
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
	UserID      int32 // Añadir ID de usuario
	GroupID     int32 // Añadir ID de grupo
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
	isValid, isAdmin, userGroup := validateCredentials(content, params.User, params.Pass)

	if !isValid {

		c.JSON(http.StatusOK, gin.H{
			"mensaje": "Error: Credenciales incorrectas. Verifique usuario y contraseña.",
			"exito":   false,
		})
		return
	}

	// Ahora que tenemos el grupo, buscar los IDs
	userID := int32(0)
	groupID := int32(0)

	// Buscar el ID de usuario
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		// Verificar si es un usuario (U) y coincide con el nombre
		if len(parts) >= 5 && strings.TrimSpace(parts[1]) == "U" && strings.TrimSpace(parts[2]) == params.User {
			// Obtener ID de usuario
			if id, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
				userID = int32(id)
			}
			break
		}
	}

	// Buscar el ID de grupo
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		// Verificar si es un grupo (G) y coincide con el grupo del usuario
		if len(parts) >= 3 && strings.TrimSpace(parts[1]) == "G" && strings.TrimSpace(parts[2]) == userGroup {
			// Obtener ID de grupo
			if id, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
				groupID = int32(id)
			}
			break
		}
	}

	// Crear sesión con los IDs
	CurrentSession = &SessionInfo{
		Username:    params.User,
		PartitionID: params.ID,
		IsAdmin:     isAdmin,
		UserGroup:   userGroup,
		UserID:      userID,
		GroupID:     groupID,
	}
	common.SetActiveUser(CurrentSession.UserID, CurrentSession.GroupID)

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

// Incluye los IDs de usuario y grupo si la sesión está activa
func GetSessionUserInfo() (bool, int32, int32, string, string) {
	if CurrentSession == nil {
		return false, 0, 0, "", ""
	}

	return true,
		CurrentSession.UserID,
		CurrentSession.GroupID,
		CurrentSession.Username,
		CurrentSession.UserGroup
}
