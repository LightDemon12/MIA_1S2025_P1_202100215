package analizador

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func handleRmdisk(c *gin.Context, comando string) {
	// Implementación futura
	c.JSON(http.StatusNotImplemented, gin.H{
		"mensaje": "Comando rmdisk aún no implementado",
		"exito":   false,
	})
}
