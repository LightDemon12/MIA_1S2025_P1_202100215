package analizador

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func handleFdisk(c *gin.Context, comando string) {
	// Implementación futura
	c.JSON(http.StatusNotImplemented, gin.H{
		"mensaje": "Comando fdisk aún no implementado",
		"exito":   false,
	})
}
