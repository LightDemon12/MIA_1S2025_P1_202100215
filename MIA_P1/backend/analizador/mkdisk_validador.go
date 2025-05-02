package analizador

import (
	"MIA_P1/backend/DiskManager"
	"MIA_P1/backend/utils"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

// HandleMkdisk maneja la creación de discos
func HandleMkdisk(c *gin.Context, comando string) {
	params, errores, requiereConfirmacion, dirPath := AnalizarMkdisk(comando)

	if requiereConfirmacion {
		solicitarConfirmacionDirectorio(c, comando, dirPath)
		return
	}

	if len(errores) > 0 {
		mostrarErrores(c, errores)
		return
	}

	if err := DiskManager.CreateDisk(params); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"mensaje": fmt.Sprintf("Error al crear el disco: %s", err),
			"exito":   false,
		})
		return
	}

	DiskManager.LogMBR(params.Path)
	mostrarExitoMkdisk(c, params)
}

// mostrarExitoMkdisk muestra mensaje de éxito de creación del disco junto a su información
func mostrarExitoMkdisk(c *gin.Context, params utils.DiskConfig) {
	sizeStr := fmt.Sprintf("%d%s", params.Size, params.Unit)
	mensaje := fmt.Sprintf("Disco creado exitosamente:\nNombre: %s\nTamaño: %s\nRuta: %s",
		params.Name, sizeStr, params.Path)

	c.JSON(http.StatusOK, gin.H{
		"mensaje":    mensaje,
		"nombre":     params.Name,
		"tamanio":    sizeStr,
		"ruta":       params.Path,
		"parametros": params,
		"exito":      true,
	})
}
