package analizador

import (
	"MIA_P1/backend/DiskManager"
	"MIA_P1/backend/utils"
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
	params, errores, requiereConfirmacion, dirPath := AnalizarMkdisk(comando)

	if requiereConfirmacion {
		c.JSON(http.StatusOK, gin.H{
			"mensaje":              fmt.Sprintf("El directorio no existe: %s\n¿Desea crearlo?", dirPath),
			"requiereConfirmacion": true,
			"dirPath":              dirPath,
			"comando":              comando,
		})
		return
	}

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
	// Crear el disco físicamente
	if err := DiskManager.CreateDisk(params); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"mensaje": fmt.Sprintf("Error al crear el disco: %s", err),
			"exito":   false,
		})
		return
	}

	// Agregar el disco a la lista global
	utils.DiscosList[params.Path] = params

	// Crear mensaje de éxito con información del disco
	sizeStr := fmt.Sprintf("%d%s", params.Size, params.Unit)
	mensaje := fmt.Sprintf("Disco creado exitosamente:\nNombre: %s\nTamaño: %s",
		params.Name, sizeStr)

	c.JSON(http.StatusOK, gin.H{
		"mensaje":    mensaje,
		"nombre":     params.Name,
		"tamanio":    sizeStr,
		"parametros": params,
		"discos":     utils.DiscosList,
		"exito":      true,
	})
}

func CrearDirectorio(c *gin.Context) {
	var request struct {
		Path    string `json:"path"`
		Comando string `json:"comando"`
	}

	if err := c.BindJSON(&request); err != nil {
		fmt.Printf("Error al leer request: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
		return
	}

	fmt.Printf("Intentando crear directorio: %s\n", request.Path)
	if err := utils.CrearDirectorio(request.Path); err != nil {
		fmt.Printf("Error al crear directorio: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"mensaje": fmt.Sprintf("Error al crear directorio: %s", err),
			"exito":   false,
		})
		return
	}

	// Reanalizar el comando original después de crear el directorio
	params, errores, requireConfirmation, _ := AnalizarMkdisk(request.Comando)
	if len(errores) > 0 || requireConfirmation {
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

	// Verificar nuevamente si el disco ya existe
	if utils.DiskExists(params.Path) {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error: Ya existe un disco con la ruta: %s", params.Path),
			"exito":   false,
		})
		return
	}

	// Agregar el disco a la lista global
	utils.DiscosList[params.Path] = params

	// Crear mensaje de éxito
	sizeStr := fmt.Sprintf("%d%s", params.Size, params.Unit)
	mensaje := fmt.Sprintf("Directorio creado y disco creado exitosamente:\nNombre: %s\nTamaño: %s",
		params.Name, sizeStr)

	c.JSON(http.StatusOK, gin.H{
		"mensaje":    mensaje,
		"nombre":     params.Name,
		"tamanio":    sizeStr,
		"parametros": params,
		"discos":     utils.DiscosList,
		"exito":      true,
	})
}
