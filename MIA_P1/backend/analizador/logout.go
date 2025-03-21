// handleLogout.go
package analizador

import (
	"MIA_P1/backend/common"
	"github.com/gin-gonic/gin"
	"net/http"
)

// HandleLogout procesa el comando logout
func HandleLogout(c *gin.Context, comando string) {
	// Verificar que haya una sesión activa
	if CurrentSession == nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": "Error: No hay una sesión activa.",
			"exito":   false,
		})
		return
	}

	// Restablecer valores en common
	common.SetActiveUser(0, 0)

	// Cerrar sesión
	CurrentSession = nil

	c.JSON(http.StatusOK, gin.H{
		"mensaje": "Sesión cerrada correctamente.",
		"exito":   true,
	})
}
