package analizador

import (
	"MIA_P1/backend/DiskManager"
	"MIA_P1/backend/utils"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"strings"
)

func handleRmdisk(c *gin.Context, comando string) {
	path, errores := AnalizarRmdisk(comando)

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

	// Validar que el archivo exista
	exists, mensaje, _ := utils.ValidarRuta(path)
	if exists {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": "El disco no existe en la ruta especificada",
			"exito":   false,
		})
		return
	}

	// Si el mensaje contiene "Ya existe un disco", entonces podemos eliminar
	if !strings.Contains(mensaje, "Ya existe un disco") {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": mensaje,
			"exito":   false,
		})
		return
	}

	// Eliminar el archivo
	if err := os.Remove(path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"mensaje": fmt.Sprintf("Error al eliminar el disco: %s", err),
			"exito":   false,
		})
		return
	}

	// Eliminar el disco de la estructura en memoria
	removed := DiskManager.RemoveDisk(path)

	mensaje = fmt.Sprintf("Disco eliminado exitosamente: %s", path)
	if removed {
		mensaje += "\nRegistro del disco eliminado de la memoria"
	} else {
		mensaje += "\nNota: El disco no estaba registrado en memoria"
	}

	c.JSON(http.StatusOK, gin.H{
		"mensaje": mensaje,
		"exito":   true,
		"path":    path,
	})
}
