package analizador

import (
	"MIA_P1/backend/DiskManager"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func HandleExt2AutoInject(c *gin.Context, comando string) {
	// Extraer el ID directamente, similar a como lo hace el comando mkfs
	id := ""

	// Eliminar la parte del comando
	comando = strings.ReplaceAll(comando, "ext2autoinject", "")
	comando = strings.TrimSpace(comando)

	// Buscar el parámetro id
	params := strings.Split(comando, " ")
	for _, param := range params {
		param = strings.TrimSpace(param)
		if strings.HasPrefix(param, "-id=") {
			id = strings.TrimPrefix(param, "-id=")
			break
		} else if strings.HasPrefix(param, "->id=") {
			id = strings.TrimPrefix(param, "->id=")
			break
		}
	}

	// Verificar que se proporcionó un ID
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": "Error: Debe especificar un ID de partición con -id=XXX",
			"exito":   false,
		})
		return
	}

	// Conservar el ID exactamente como se proporcionó (sin convertir a minúsculas)
	success, message := DiskManager.EXT2AutoInjector(id)

	if success {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": message,
			"exito":   true,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": message,
			"exito":   false,
		})
	}
}
