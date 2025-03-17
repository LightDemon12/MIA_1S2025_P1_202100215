package analizador

import (
	"MIA_P1/backend/DiskManager"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func handleMount(c *gin.Context, comando string) {
	params, errores, valido := AnalizarMount(comando)

	if !valido || len(errores) > 0 {
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

	// Limpiar el nombre de comillas
	params.Name = strings.Trim(params.Name, "\"")

	fmt.Printf("Debug: Intentando montar partición '%s' en disco '%s'\n", params.Name, params.Path)

	// Montar la partición
	id, err := DiskManager.MountPartition(params.Path, params.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"mensaje": fmt.Sprintf("Error al montar la partición: %s", err),
			"exito":   false,
		})
		return
	}

	// Obtener la lista de particiones montadas
	mountedPartitions := DiskManager.GetMountedPartitions()

	// Preparar la respuesta
	mensaje := fmt.Sprintf("Partición montada exitosamente:\nID: %s\nPath: %s\nNombre: %s", id, params.Path, params.Name)

	c.JSON(http.StatusOK, gin.H{
		"mensaje":     mensaje,
		"id":          id,
		"path":        params.Path,
		"name":        params.Name,
		"particiones": mountedPartitions,
		"exito":       true,
	})
}
