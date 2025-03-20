// handleMkfile.go
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

// Permisos por defecto para archivos nuevos (664 en octal)
const DEFAULT_FILE_PERMS = "664"

type OverwriteRequest struct {
	TipoConfirmacion string `json:"tipoConfirmacion"`
	Confirmar        bool   `json:"confirmar"`
	Comando          string `json:"comando"`
	Path             string `json:"path"`
}

// HandleMkfile procesa el comando mkfile
func HandleMkfile(c *gin.Context, comando string) {
	// Verificar si es una solicitud de sobreescritura
	var overwriteReq OverwriteRequest
	if c.Request.Header.Get("Content-Type") == "application/json" {
		if err := c.ShouldBindJSON(&overwriteReq); err == nil &&
			overwriteReq.TipoConfirmacion == "sobreescribir" {

			// Validar los parámetros del comando original
			params, errores := ValidarMkfile(overwriteReq.Comando)
			if len(errores) > 0 {
				mostrarErrores(c, errores)
				return
			}

			// Preparar el contenido
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

			// Llamar a la función de sobreescritura
			success, errMsg := DiskManager.OverwriteEXT2File(
				CurrentSession.PartitionID,
				overwriteReq.Path,
				content,
			)

			if !success {
				c.JSON(http.StatusOK, gin.H{
					"mensaje": errMsg,
					"exito":   false,
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"mensaje": fmt.Sprintf("Archivo '%s' sobrescrito exitosamente", overwriteReq.Path),
				"exito":   true,
			})
			return
		}
	}

	// Continuar con el flujo normal si no es una sobreescritura
	if CurrentSession == nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": "Error: No hay una sesión activa. Debe iniciar sesión primero.",
			"exito":   false,
		})
		return
	}

	// Validar los parámetros del comando
	params, errores := ValidarMkfile(comando)
	if len(errores) > 0 {
		mostrarErrores(c, errores)
		return
	}

	// Normalizar la ruta
	filePath := normalizePath(params.Path)

	// Obtener directorio padre y nombre del archivo
	parentDir := filepath.Dir(filePath)
	if parentDir == "." {
		parentDir = "/"
	}

	// Verificar si el archivo ya existe
	fileExists, err := DiskManager.FileExists(CurrentSession.PartitionID, filePath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error al verificar existencia del archivo: %s", err),
			"exito":   false,
		})
		return
	}

	if fileExists {
		// Preguntar al frontend si se desea sobreescribir
		c.JSON(http.StatusOK, gin.H{
			"mensaje":              fmt.Sprintf("El archivo '%s' ya existe. ¿Desea sobreescribirlo?", filePath),
			"exito":                true,
			"requiereConfirmacion": true,
			"tipoConfirmacion":     "sobreescribir",
			"comando":              comando,
			"path":                 filePath,
		})
		return
	}

	// Verificar si el directorio padre existe
	parentExists, err := DiskManager.FileExists(CurrentSession.PartitionID, parentDir)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error al verificar directorio padre: %s", err),
			"exito":   false,
		})
		return
	}

	if !parentExists {
		if params.CreateDirs {
			// CAMBIO AQUÍ: En lugar de pedir confirmación, crear directorios automáticamente
			fmt.Printf("INFO: Creando directorios para la ruta: %s\n", filePath)

			// Crear directorios padres recursivamente
			dirSegments := strings.Split(strings.TrimPrefix(parentDir, "/"), "/")
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

				// Verificar si este nivel ya existe
				exists, err := DiskManager.FileExists(CurrentSession.PartitionID, currentPath)
				if err != nil {
					fmt.Printf("WARN: Error verificando existencia de '%s': %v\n", currentPath, err)
				}

				if !exists {
					fmt.Printf("INFO: Creando directorio '%s'\n", currentPath)

					// Crear solo este nivel de directorio
					success, errMsg := DiskManager.CreateEXT2Directory(CurrentSession.PartitionID, currentPath)
					if !success {
						c.JSON(http.StatusOK, gin.H{
							"mensaje": fmt.Sprintf("Error al crear directorio '%s': %s", currentPath, errMsg),
							"exito":   false,
						})
						return
					}

					fmt.Printf("INFO: Directorio '%s' creado exitosamente\n", currentPath)
				} else {
					fmt.Printf("INFO: Directorio '%s' ya existe\n", currentPath)
				}
			}
		} else {
			// Error si no existen los directorios padres y no se usa -r
			c.JSON(http.StatusOK, gin.H{
				"mensaje": fmt.Sprintf("Error: El directorio padre '%s' no existe. Use el parámetro -r para crear directorios.", parentDir),
				"exito":   false,
			})
			return
		}
	}

	// Verificar permisos de escritura en el directorio padre
	_, parentInode, err := DiskManager.GetFileInode(CurrentSession.PartitionID, parentDir)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error al obtener información del directorio padre: %s", err),
			"exito":   false,
		})
		return
	}

	// Si el usuario es root, tiene permisos administrativos
	if !CurrentSession.IsAdmin {
		// Verificar si el usuario tiene permisos de escritura
		hasWritePermission := false

		// Obtener ID de usuario y grupo
		userId := DiskManager.GetUserIdFromName(CurrentSession.PartitionID, CurrentSession.Username)
		groupId := DiskManager.GetGroupIdFromName(CurrentSession.PartitionID, CurrentSession.UserGroup)

		// Si el usuario es dueño
		if parentInode.IUid == userId {
			hasWritePermission = (parentInode.IPerm[0] & 2) != 0 // w para propietario
		} else if parentInode.IGid == groupId {
			// Si el usuario está en el grupo
			hasWritePermission = (parentInode.IPerm[1] & 2) != 0 // w para grupo
		} else {
			// Otros usuarios
			hasWritePermission = (parentInode.IPerm[2] & 2) != 0 // w para otros
		}

		if !hasWritePermission {
			c.JSON(http.StatusOK, gin.H{
				"mensaje": fmt.Sprintf("Error: No tiene permisos de escritura en el directorio '%s'", parentDir),
				"exito":   false,
			})
			return
		}
	}

	// Preparar el contenido del archivo
	var content string

	if params.Cont != "" {
		// Leer contenido del archivo local
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
		// Generar contenido según el tamaño especificado
		content = generateContent(params.Size)
	}

	// Convertir permisos a byte array
	perms := stringToPerms(DEFAULT_FILE_PERMS)

	// Crear el archivo con los permisos por defecto (664)
	err = DiskManager.EXT2CreateFile(
		CurrentSession.PartitionID,
		filePath,
		content,
		CurrentSession.Username,
		CurrentSession.UserGroup,
		perms,
	)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error al crear el archivo: %s", err),
			"exito":   false,
		})
		return
	}

	// Responder con éxito
	c.JSON(http.StatusOK, gin.H{
		"mensaje": fmt.Sprintf("Archivo '%s' creado exitosamente", filePath),
		"exito":   true,
	})
}

// normalizePath asegura que la ruta comience con / y elimina / duplicados
func normalizePath(path string) string {
	// Asegurarse que la ruta comience con /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Eliminar / duplicados
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}

	// Eliminar / al final si no es solo /
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}

	return path
}

// generateContent genera contenido numérico del tamaño especificado
func generateContent(size int) string {
	if size <= 0 {
		return ""
	}

	var content strings.Builder
	for i := 0; i < size; i++ {
		digit := i % 10
		content.WriteString(fmt.Sprintf("%d", digit))
	}

	return content.String()
}

// stringToPerms convierte un string de permisos (ej: "664") a un array de bytes [6,6,4]
func stringToPerms(perms string) []byte {
	result := make([]byte, 3)
	for i := 0; i < len(perms) && i < 3; i++ {
		if i < len(perms) {
			result[i] = perms[i] - '0'
		} else {
			result[i] = 0
		}
	}
	return result
}
