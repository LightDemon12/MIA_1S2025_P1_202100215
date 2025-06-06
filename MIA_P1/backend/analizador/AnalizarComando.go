package analizador

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ConfirmationRequest struct {
	TipoConfirmacion string `json:"tipoConfirmacion"`
	Confirmar        bool   `json:"confirmar"`
	Comando          string `json:"comando"`
	Path             string `json:"path"`
}

func AnalizarComando(c *gin.Context) {
	// Primero verificar si es una petición de confirmación
	var confirmReq ConfirmationRequest
	if c.Request.ContentLength > 0 {
		// Intentar decodificar como una solicitud de confirmación
		if c.Request.Header.Get("Content-Type") == "application/json" {
			if err := c.ShouldBindJSON(&confirmReq); err == nil && confirmReq.TipoConfirmacion != "" {
				// Es una solicitud de confirmación
				switch confirmReq.TipoConfirmacion {
				case "sobreescribir":
					return

				case "crearDirs":
					if confirmReq.Confirmar {

						c.JSON(http.StatusOK, gin.H{
							"mensaje":   "Por favor use el endpoint /ext2-crear-directorios para esta operación",
							"exito":     false,
							"redirigir": "/ext2-crear-directorios",
						})
					} else {
						c.JSON(http.StatusOK, gin.H{
							"mensaje": "Operación cancelada por el usuario",
							"exito":   true,
						})
					}
					return
				}
				// Si llegamos aquí, el tipo de confirmación no es reconocido
				c.JSON(http.StatusBadRequest, gin.H{
					"mensaje": fmt.Sprintf("Tipo de confirmación no reconocido: %s", confirmReq.TipoConfirmacion),
					"exito":   false,
				})
				return
			}
		}
	}

	// Si no es confirmación, procesar como comando normal
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
	case CMD_MOUNT:
		handleMount(c, comando)
	case CMD_MOUNTED:
		HandleMounted(c, comando)
	case CMD_REP:
		HandleRep(c, comando)
	case CMD_MKFS:
		HandleMkfs(c, comando)
	case CMD_LOGIN:
		HandleLogin(c, comando)
	case CMD_LOGOUT:
		HandleLogout(c, comando)
	case CMD_CAT:
		HandleCat(c, comando)
	case CMD_MKGRP:
		HandleMkgrp(c, comando)
	case CMD_RMGRP:
		HandleRmgrp(c, comando)
	case CMD_MKUSR:
		HandleMkusr(c, comando)
	case CMD_RMUSR:
		HandleRmusr(c, comando)
	case CMD_CHGRP:
		HandleChgrp(c, comando)
	case CMD_MKFILE:
		HandleMkfile(c, comando)
	case CMD_MKDIR:
		HandleMkdir(c, comando)
	case CMD_COMENTARIO:
		c.JSON(http.StatusOK, gin.H{
			"mensaje": "", // Mensaje vacío para no duplicar el comentario
			"exito":   true,
		})
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
