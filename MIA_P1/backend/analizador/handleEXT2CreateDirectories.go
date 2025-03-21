package analizador

import (
	"MIA_P1/backend/DiskManager"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"path/filepath"
)

// Estructura para la solicitud de creación de directorios
// EXT2CreateDirsRequest estructura para las peticiones
type EXT2CreateDirsRequest struct {
	Path      string `json:"path"`      // Ruta del archivo/directorio
	Command   string `json:"command"`   // Comando original
	Confirm   bool   `json:"confirm"`   // Confirmación del usuario
	Overwrite bool   `json:"overwrite"` // Flag para sobreescritura
}

func HandleEXT2CreateDirectories(c *gin.Context) {
	var req EXT2CreateDirsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"mensaje": "Datos inválidos",
			"exito":   false,
		})
		return
	}

	// Debug log
	fmt.Printf("Recibida solicitud EXT2: path=%s, overwrite=%v\n", req.Path, req.Overwrite)

	if !req.Confirm {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": "Operación cancelada por el usuario",
			"exito":   true,
		})
		return
	}

	if CurrentSession == nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": "Error: No hay una sesión activa",
			"exito":   false,
		})
		return
	}

	params, errores := ValidarMkfile(req.Command)
	if len(errores) > 0 {
		mostrarErrores(c, errores)
		return
	}

	normalizedPath := normalizePath(req.Path)

	// Si es una sobreescritura, manejarla directamente
	if req.Overwrite {
		fmt.Printf("Procesando sobreescritura para: %s\n", normalizedPath)

		var content string
		if params.Cont != "" {
			contentBytes, err := ioutil.ReadFile(params.Cont)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"mensaje": fmt.Sprintf("Error al leer el archivo: %s", err),
					"exito":   false,
				})
				return
			}
			content = string(contentBytes)
		} else if params.Size >= 0 {
			content = generateContent(params.Size)
		}

		// Usar OverwriteEXT2File con la nueva firma que devuelve un error
		err := DiskManager.OverwriteEXT2File(CurrentSession.PartitionID, normalizedPath, content)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": fmt.Sprintf("Error al sobreescribir el archivo: %s", err.Error()),
				"exito":   false,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Archivo '%s' sobreescrito exitosamente", normalizedPath),
			"exito":   true,
		})
		return
	}

	// Si no es sobreescritura, crear directorios padres recursivamente
	parentDir := filepath.Dir(normalizedPath)
	if parentDir == "." {
		parentDir = "/"
	}

	// Permisos por defecto para directorios: 755
	dirPerms := []byte{7, 5, 5}

	// Usar la nueva función CreateEXT2DirectoryRecursive para crear todos los directorios padres
	fmt.Printf("Creando directorios padres para: %s\n", normalizedPath)
	err := DiskManager.CreateEXT2DirectoryRecursive(
		CurrentSession.PartitionID,
		parentDir,
		CurrentSession.Username,
		CurrentSession.UserGroup,
		dirPerms,
	)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error al crear directorios padres: %s", err.Error()),
			"exito":   false,
		})
		return
	}

	// Crear el archivo final
	var content string
	if params.Cont != "" {
		contentBytes, err := ioutil.ReadFile(params.Cont)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": fmt.Sprintf("Error al leer el archivo '%s': %s", params.Cont, err),
				"exito":   false,
			})
			return
		}
		content = string(contentBytes)
	} else if params.Size >= 0 {
		content = generateContent(params.Size)
	}

	// Permisos por defecto para archivos: 664
	filePerms := []byte{6, 6, 4}

	fmt.Printf("Creando archivo: %s\n", normalizedPath)
	// Llamar CreateEXT2File con la nueva firma
	err = DiskManager.CreateEXT2File(
		CurrentSession.PartitionID,
		normalizedPath,
		content,
		CurrentSession.Username,
		CurrentSession.UserGroup,
		filePerms,
	)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error al crear el archivo: %s", err.Error()),
			"exito":   false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"mensaje": fmt.Sprintf("Archivo '%s' creado exitosamente", normalizedPath),
		"exito":   true,
	})
}
