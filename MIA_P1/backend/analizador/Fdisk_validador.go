package analizador

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

func handleFdisk(c *gin.Context, comando string) {
	params, errores, _ := AnalizarFdisk(comando)

	if len(errores) > 0 {
		mensajeError := "Errores encontrados:\n"
		for _, err := range errores {
			mensajeError += fmt.Sprintf("- %s: %s\n", err.Parametro, err.Mensaje)
		}
		c.JSON(http.StatusOK, gin.H{
			"mensaje": mensajeError,
			"exito":   false,
		})
		return
	}

	// Por ahora solo mostraremos los parámetros validados
	mensaje := fmt.Sprintf("Parámetros de partición validados:\nNombre: %s\nTamaño: %d%s\nTipo: %s\nAjuste: %s",
		params.Name, params.Size, params.Unit, params.Type, params.Fit)

	c.JSON(http.StatusOK, gin.H{
		"mensaje":    mensaje,
		"nombre":     params.Name,
		"tamanio":    fmt.Sprintf("%d%s", params.Size, params.Unit),
		"tipo":       params.Type,
		"ajuste":     params.Fit,
		"parametros": params,
		"exito":      true,
	})
}
