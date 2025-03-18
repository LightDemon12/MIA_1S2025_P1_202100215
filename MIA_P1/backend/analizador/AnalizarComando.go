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
	cmdType := IdentificarComando(comando)

	switch cmdType {
	case CMD_MKDISK:
		HandleMkdisk(c, comando)
	case CMD_RMDISK:
		handleRmdisk(c, comando)
	case CMD_FDISK:
		handleFdisk(c, comando)
	case CMD_MOUNT: // Agregar este caso
		handleMount(c, comando)
	case CMD_MOUNTED: // Nuevo caso
		HandleMounted(c, comando)
	case CMD_REP: // Agregar este caso
		HandleRep(c, comando)
	case CMD_MKFS:
		HandleMkfs(c, comando)
	case CMD_EXT2AUTOINJECT:
		HandleExt2AutoInject(c, comando) // Usar el controlador correcto
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"mensaje": "Comando no reconocido",
			"exito":   false,
		})
	}
}

func mostrarErrores(c *gin.Context, errores []Error) {
	mensajeError := "Errores encontrados:\n"
	for _, err := range errores {
		mensajeError += fmt.Sprintf("- %s: %s\n", err.Parametro, err.Mensaje)
	}
	c.JSON(http.StatusOK, gin.H{
		"mensaje": mensajeError,
		"exito":   false,
	})
}
