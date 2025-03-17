package analizador

import (
	"MIA_P1/backend/DiskManager"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

// HandleMounted maneja el comando mounted para mostrar particiones montadas
func HandleMounted(c *gin.Context, comando string) {
	// Verificar que el comando comience con "mounted"
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(comando)), "mounted") {
		c.JSON(http.StatusBadRequest, gin.H{
			"mensaje": "El comando debe comenzar con 'mounted'",
			"exito":   false,
		})
		return
	}

	// Obtener particiones montadas
	mountedPartitions := DiskManager.GetMountedPartitions()

	if len(mountedPartitions) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"mensaje":     "No hay particiones montadas actualmente",
			"exito":       true,
			"particiones": []string{},
		})
		return
	}

	// Construir mensaje de respuesta
	mensaje := "PARTICIONES MONTADAS\n"
	mensaje += "===================\n\n"

	// Agregar cada partición al mensaje
	for i, mp := range mountedPartitions {
		mensaje += fmt.Sprintf("%d. ID: %s\n", i+1, mp.ID)
		mensaje += fmt.Sprintf("   Nombre: %s\n", mp.PartitionName)
		mensaje += fmt.Sprintf("   Disco: %s\n", mp.DiskPath)
		mensaje += fmt.Sprintf("   Tipo: %c\n", mp.PartitionType)
		mensaje += fmt.Sprintf("   Letra: %c\n", mp.Letter)
		mensaje += fmt.Sprintf("   Número: %d\n", mp.Number)
		mensaje += "\n"
	}

	// Preparar datos para la respuesta JSON
	partitionsData := make([]map[string]interface{}, 0, len(mountedPartitions))
	for _, mp := range mountedPartitions {
		partitionsData = append(partitionsData, map[string]interface{}{
			"id":     mp.ID,
			"path":   mp.DiskPath,
			"name":   mp.PartitionName,
			"type":   string(mp.PartitionType),
			"letter": string(mp.Letter),
			"number": mp.Number,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"mensaje":     mensaje,
		"exito":       true,
		"particiones": partitionsData,
	})
}
