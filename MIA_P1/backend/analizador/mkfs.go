package analizador

import (
	"MIA_P1/backend/DiskManager"
	"github.com/gin-gonic/gin"
	"net/http"
	"regexp"
	"strings"
)

// MkfsParams contiene los parámetros para el comando MKFS
type MkfsParams struct {
	Id   string
	Type string
}

// HandleMkfs procesa el comando MKFS
func HandleMkfs(c *gin.Context, comando string) {
	var params MkfsParams
	var errores []Error

	// Expresiones regulares para extraer parámetros
	idRegex := regexp.MustCompile(`(?i)-id=([^\s]+)`)
	typeRegex := regexp.MustCompile(`(?i)-type=([^\s]+)`)

	// Extraer ID (obligatorio)
	idMatches := idRegex.FindStringSubmatch(comando)
	if len(idMatches) > 1 {
		params.Id = idMatches[1]
		params.Id = strings.Trim(params.Id, "\"")
	} else {
		errores = append(errores, Error{
			Parametro: "id",
			Mensaje:   "El parámetro id es obligatorio",
		})
	}

	// Extraer Type (opcional, default: "full")
	typeMatches := typeRegex.FindStringSubmatch(comando)
	if len(typeMatches) > 1 {
		params.Type = strings.ToLower(typeMatches[1])
		params.Type = strings.Trim(params.Type, "\"")

		// Validar que el tipo sea "full"
		if params.Type != "full" {
			errores = append(errores, Error{
				Parametro: "type",
				Mensaje:   "El tipo de formateo debe ser 'full'",
			})
		}
	} else {
		// Si no se especifica, se toma como "full"
		params.Type = "full"
	}

	// Si hay errores, mostrarlos
	if len(errores) > 0 {
		mostrarErrores(c, errores)
		return
	}

	success, mensaje := DiskManager.FormatearParticion(params.Id, params.Type)

	if success {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": mensaje,
			"exito":   true,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": mensaje,
			"exito":   false,
		})
	}
}
