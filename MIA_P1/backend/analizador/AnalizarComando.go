package analizador

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

type ComandoRequest struct {
	Comando string `json:"comando"`
}

func AnalizarComando(c *gin.Context) {
	comandoBytes, err := c.GetRawData()
	if err != nil {
		c.String(http.StatusBadRequest, "Error al leer comando")
		return
	}

	comando := string(comandoBytes)
	// Como estamos en el mismo paquete, podemos llamar a AnalizarMkdisk directamente
	params, errores := AnalizarMkdisk(comando)

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

	// Agregar el disco a la lista global
	DiscosList[params.Path] = params

	// Crear mensaje de éxito con información del disco
	sizeStr := fmt.Sprintf("%d%s", params.Size, params.Unit)
	mensaje := fmt.Sprintf("Disco creado exitosamente:\nNombre: %s\nTamaño: %s",
		params.Name, sizeStr)

	c.JSON(http.StatusOK, gin.H{
		"mensaje":    mensaje,
		"nombre":     params.Name,
		"tamanio":    sizeStr,
		"parametros": params,
		"discos":     DiscosList, // Agregamos la lista de discos a la respuesta
		"exito":      true,
	})

}
