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

	// Asignar tipo usando constantes
	switch params.Type {
	case "P":
		partition.Type = DiskManager.PARTITION_PRIMARY
	case "E":
		partition.Type = DiskManager.PARTITION_EXTENDED
	case "L":
		partition.Type = DiskManager.PARTITION_LOGIC
	default:
		partition.Type = DiskManager.PARTITION_PRIMARY
	}

	// Asignar fit usando constantes
	switch params.Fit {
	case "BF":
		partition.Fit = DiskManager.FIT_BEST
	case "FF":
		partition.Fit = DiskManager.FIT_FIRST
	case "WF":
		partition.Fit = DiskManager.FIT_WORST
	default:
		partition.Fit = DiskManager.FIT_FIRST
	}

	partition.Size = int64(params.Size)
	copy(partition.Name[:], params.Name)
	partition.Status = DiskManager.PARTITION_NOT_MOUNTED

	fmt.Printf("Debug: Creando partición con Type=%c, Fit=%c, Size=%d, Name=%s\n",
		partition.Type, partition.Fit, partition.Size, params.Name)

	// Determinar qué función usar según el tipo de partición
	var resultado error
	if partition.Type == DiskManager.PARTITION_LOGIC {
		// Si es lógica, usar CreateLogicalPartition
		fmt.Printf("Debug: Usando CreateLogicalPartition\n")
		resultado = partitionManager.CreateLogicalPartition(&partition, params.Unit)
	} else {
		// Si es primaria o extendida, usar CreatePartition
		fmt.Printf("Debug: Usando CreatePartition\n")
		resultado = partitionManager.CreatePartition(&partition, params.Unit)
	}

	// Verificar si hubo error
	if resultado != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"mensaje": fmt.Sprintf("Error al crear la partición: %s", resultado),
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
