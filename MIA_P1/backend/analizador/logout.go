// handleLogout.go
package analizador

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// HandleLogout procesa el comando logout
func HandleLogout(c *gin.Context, comando string) {
	// Verificar que haya una sesión activa
	if CurrentSession == nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": "Error: No hay ninguna sesión activa",
			"exito":   false,
		})
		return
	}

	// Guardar el nombre de usuario para el mensaje
	username := CurrentSession.Username

	// Cerrar sesión
	CurrentSession = nil

	// Responder con éxito
	c.JSON(http.StatusOK, gin.H{
		"mensaje": "Sesión cerrada correctamente para el usuario " + username,
		"exito":   true,
	})
}
