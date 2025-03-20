package analizador

import (
	"MIA_P1/backend/DiskManager"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
)

// Estructura para la solicitud de creación de directorios
// EXT2CreateDirsRequest estructura para las peticiones
type EXT2CreateDirsRequest struct {
	Path      string `json:"path"`      // Ruta del archivo/directorioEXT2CreateFile
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

		// Usar OverwriteEXT2File en lugar de CreateEXT2File
		success, errMsg := DiskManager.OverwriteEXT2File(CurrentSession.PartitionID, normalizedPath, content)
		if !success {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": fmt.Sprintf("Error al sobreescribir el archivo: %s", errMsg),
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

	// Si no es sobreescritura, crear directorios recursivamente
	parentDir := filepath.Dir(normalizedPath)
	if parentDir == "." {
		parentDir = "/"
	}

	dirSegments := strings.Split(strings.TrimPrefix(parentDir, "/"), "/")
	currentPath := "/"

	fmt.Printf("Creando directorios para la ruta: %s\n", normalizedPath)

	for _, segment := range dirSegments {
		if segment == "" {
			continue
		}

		if currentPath == "/" {
			currentPath = "/" + segment
		} else {
			currentPath = currentPath + "/" + segment
		}

		fmt.Printf("Verificando directorio: %s\n", currentPath)

		exists, _ := DiskManager.FileExists(CurrentSession.PartitionID, currentPath)
		if !exists {
			fmt.Printf("Creando directorio: %s\n", currentPath)

			success, errMsg := DiskManager.CreateEXT2Directory(CurrentSession.PartitionID, currentPath)
			if !success {
				c.JSON(http.StatusOK, gin.H{
					"mensaje": fmt.Sprintf("Error al crear directorio '%s': %s", currentPath, errMsg),
					"exito":   false,
				})
				return
			}

			exists, _ = DiskManager.FileExists(CurrentSession.PartitionID, currentPath)
			if !exists {
				c.JSON(http.StatusOK, gin.H{
					"mensaje": fmt.Sprintf("Error: No se pudo verificar la creación del directorio '%s'", currentPath),
					"exito":   false,
				})
				return
			}

			fmt.Printf("Directorio creado exitosamente: %s\n", currentPath)
		} else {
			fmt.Printf("El directorio ya existe: %s\n", currentPath)
		}
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

	fmt.Printf("Creando archivo: %s\n", normalizedPath)
	success, errMsg := DiskManager.CreateEXT2File(CurrentSession.PartitionID, normalizedPath, content)
	if !success {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error al crear el archivo: %s", errMsg),
			"exito":   false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"mensaje": fmt.Sprintf("Archivo '%s' creado exitosamente", normalizedPath),
		"exito":   true,
	})
}
