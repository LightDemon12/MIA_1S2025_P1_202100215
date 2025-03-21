package analizador

import (
	"MIA_P1/backend/DiskManager"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"path/filepath"
	"strings"
)

func HandleMkdir(c *gin.Context, comando string) {
	if CurrentSession == nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": "Error: No hay una sesión activa. Debe iniciar sesión primero.",
			"exito":   false,
		})
		return
	}

	// Validar los parámetros del comando
	params, errores := ValidarMkdir(comando)
	if len(errores) > 0 {
		mostrarErrores(c, errores)
		return
	}

	// Normalizar la ruta y obtener directorio padre
	dirPath := normalizePath(params.Path)
	parentDir := filepath.Dir(dirPath)
	if parentDir == "." {
		parentDir = "/"
	}

	fmt.Printf("DEBUG: Creando directorio '%s' en padre '%s'\n", dirPath, parentDir)

	// Verificar si el directorio ya existe
	exists, err := fileExists(CurrentSession.PartitionID, dirPath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error al verificar existencia del directorio: %s", err),
			"exito":   false,
		})
		return
	}

	if exists {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("El directorio '%s' ya existe", dirPath),
			"exito":   false,
		})
		return
	}

	// Verificar si el directorio padre existe
	parentExists, err := fileExists(CurrentSession.PartitionID, parentDir)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error al verificar directorio padre: %s", err),
			"exito":   false,
		})
		return
	}

	if !parentExists {
		if params.CreateDirs {
			fmt.Printf("INFO: Creando directorios padres para: %s\n", dirPath)

			// Crear directorios padres recursivamente
			dirSegments := strings.Split(strings.TrimPrefix(dirPath, "/"), "/")
			currentPath := "/"

			for _, segment := range dirSegments {
				if segment == "" {
					continue
				}

				// Construir la ruta para este nivel
				if currentPath == "/" {
					currentPath = "/" + segment
				} else {
					currentPath = currentPath + "/" + segment
				}

				fmt.Printf("DEBUG: Verificando directorio: '%s'\n", currentPath)

				// Verificar si este nivel ya existe
				exists, err := fileExists(CurrentSession.PartitionID, currentPath)
				if err != nil {
					fmt.Printf("WARN: Error verificando existencia de '%s': %v\n", currentPath, err)
				}

				if !exists {
					fmt.Printf("INFO: Creando directorio '%s'\n", currentPath)

					// CAMBIO IMPORTANTE: Usar permisos 664 para directorios
					perms := stringToPerms("664")
					err := DiskManager.CreateEXT2Directory(
						CurrentSession.PartitionID,
						currentPath,
						CurrentSession.Username,
						CurrentSession.UserGroup,
						perms,
					)

					if err != nil {
						c.JSON(http.StatusOK, gin.H{
							"mensaje": fmt.Sprintf("Error al crear directorio '%s': %s", currentPath, err),
							"exito":   false,
						})
						return
					}

					// Verificar que el directorio se creó correctamente
					exists, err = fileExists(CurrentSession.PartitionID, currentPath)
					if !exists {
						c.JSON(http.StatusOK, gin.H{
							"mensaje": fmt.Sprintf("Error crítico: el directorio '%s' no se creó correctamente", currentPath),
							"exito":   false,
						})
						return
					}

					fmt.Printf("INFO: Directorio '%s' creado exitosamente\n", currentPath)
				}
			}
		} else {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": fmt.Sprintf("Error: El directorio padre '%s' no existe. Use el parámetro -p para crear directorios.", parentDir),
				"exito":   false,
			})
			return
		}
	} else {
		// Si el padre existe, crear solo el directorio final
		fmt.Printf("INFO: Creando directorio final '%s'\n", dirPath)
		perms := stringToPerms("664")
		err := DiskManager.CreateEXT2Directory(
			CurrentSession.PartitionID,
			dirPath,
			CurrentSession.Username,
			CurrentSession.UserGroup,
			perms,
		)

		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": fmt.Sprintf("Error al crear directorio '%s': %s", dirPath, err),
				"exito":   false,
			})
			return
		}

		// Verificar que el directorio se creó correctamente
		exists, err = fileExists(CurrentSession.PartitionID, dirPath)
		if !exists {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": fmt.Sprintf("Error crítico: el directorio '%s' no se creó correctamente", dirPath),
				"exito":   false,
			})
			return
		}
	}

	// Responder con éxito
	c.JSON(http.StatusOK, gin.H{
		"mensaje": fmt.Sprintf("Directorio '%s' creado exitosamente", dirPath),
		"exito":   true,
	})
}
