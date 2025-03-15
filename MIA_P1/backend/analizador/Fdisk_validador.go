package analizador

import (
	"MIA_P1/backend/DiskManager"
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

	// Create new partition manager
	partitionManager, err := DiskManager.NewPartitionManager(params.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"mensaje": fmt.Sprintf("Error al abrir el disco: %s", err),
			"exito":   false,
		})
		return
	}

	// Create partition
	partition := DiskManager.NewPartition()
	partition.Type = byte(params.Type[0])
	partition.Fit = byte(params.Fit[0])
	partition.Size = int64(params.Size)
	copy(partition.Name[:], params.Name)
	partition.Status = DiskManager.PARTITION_NOT_MOUNTED

	// Create partition using the manager
	if err := partitionManager.CreatePartition(&partition, params.Unit); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"mensaje": fmt.Sprintf("Error al crear la partición: %s", err),
			"exito":   false,
		})
		return
	}

	mensaje := fmt.Sprintf("Partición creada exitosamente:\nNombre: %s\nTamaño: %d%s\nTipo: %s\nAjuste: %s",
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
