// handleMkfile.go
package analizador

import (
	"MIA_P1/backend/DiskManager"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

			// Llamar a la función de sobreescritura con la nueva firma
			err := DiskManager.OverwriteEXT2File(
				CurrentSession.PartitionID,
				overwriteReq.Path,
				content,
			)

			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"mensaje": err.Error(),
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
	parentDir, _ := getParentAndBasePath(filePath)
	if parentDir == "." {
		parentDir = "/"
	}

	// Verificar si el archivo ya existe
	exists, err := fileExists(CurrentSession.PartitionID, filePath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": fmt.Sprintf("Error al verificar existencia del archivo: %s", err),
			"exito":   false,
		})
		return
	}

	if exists {
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
				exists, err := fileExists(CurrentSession.PartitionID, currentPath)
				if err != nil {
					fmt.Printf("WARN: Error verificando existencia de '%s': %v\n", currentPath, err)
				}

				if !exists {
					fmt.Printf("INFO: Creando directorio '%s'\n", currentPath)

					// Crear solo este nivel de directorio usando la nueva firma
					perms := stringToPerms("755") // Permisos por defecto para directorios
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
	_, parentInode, err := getFileInode(CurrentSession.PartitionID, parentDir)
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
		userId := getUserIdFromName(CurrentSession.PartitionID, CurrentSession.Username)
		groupId := getGroupIdFromName(CurrentSession.PartitionID, CurrentSession.UserGroup)

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

	// Crear el archivo con los permisos por defecto (664) - usando la nueva firma
	err = DiskManager.CreateEXT2File(
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
	// Eliminar comillas si existen al inicio y final
	path = strings.Trim(path, "\"")

	// Asegurarse que la ruta comience con /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Dividir la ruta en segmentos preservando los espacios
	var cleanedPath string
	if strings.Contains(path, " ") {
		// Si hay espacios, tratar la ruta con más cuidado
		parts := strings.Split(path, "/")
		cleanParts := make([]string, 0)

		for _, part := range parts {
			if part != "" {
				// Preservar el segmento completo, incluyendo espacios
				cleanParts = append(cleanParts, part)
			}
		}

		cleanedPath = "/" + strings.Join(cleanParts, "/")
	} else {
		// Para rutas sin espacios, usar el método simple
		for strings.Contains(path, "//") {
			path = strings.ReplaceAll(path, "//", "/")
		}
		cleanedPath = path
	}

	// Eliminar / al final si no es solo /
	if len(cleanedPath) > 1 && strings.HasSuffix(cleanedPath, "/") {
		cleanedPath = cleanedPath[:len(cleanedPath)-1]
	}

	fmt.Printf("DEBUG: Ruta normalizada: '%s'\n", cleanedPath)
	return cleanedPath
}

// Modificar la parte donde se obtiene el directorio padre
func getParentAndBasePath(path string) (string, string) {
	// Si la ruta contiene espacios, necesitamos ser más cuidadosos
	if strings.Contains(path, " ") {
		lastSlash := strings.LastIndex(path, "/")
		if lastSlash <= 0 {
			return "/", path
		}
		return path[:lastSlash], path[lastSlash+1:]
	}

	// Para rutas sin espacios, usar filepath.Dir y filepath.Base
	return filepath.Dir(path), filepath.Base(path)
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

// Funciones auxiliares para compatibilidad con las nuevas implementaciones:

// fileExists verifica si un archivo o directorio existe
func fileExists(id string, path string) (bool, error) {
	_, _, err := getFileInode(id, path)
	if err != nil {
		// Si el error dice "no encontrado", simplemente no existe
		if strings.Contains(err.Error(), "no encontrado") ||
			strings.Contains(err.Error(), "no se encontró") {
			return false, nil
		}
		// Si es otro tipo de error, reportarlo
		return false, err
	}
	return true, nil
}

// getFileInode obtiene información sobre un archivo o directorio
func getFileInode(id string, path string) (int, *DiskManager.Inode, error) {
	// Abrir la partición
	mountedPartition, err := DiskManager.FindMountedPartitionById(id)
	if err != nil {
		return -1, nil, fmt.Errorf("partición no encontrada: %v", err)
	}

	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return -1, nil, fmt.Errorf("error al abrir disco: %v", err)
	}
	defer file.Close()

	startByte, _, err := DiskManager.GetPartitionDetails(file, mountedPartition)
	if err != nil {
		return -1, nil, fmt.Errorf("error obteniendo detalles de partición: %v", err)
	}

	_, err = file.Seek(startByte, 0)
	if err != nil {
		return -1, nil, fmt.Errorf("error al posicionarse: %v", err)
	}

	superblock, err := DiskManager.ReadSuperBlockFromDisc(file)
	if err != nil {
		return -1, nil, fmt.Errorf("error al leer superbloque: %v", err)
	}

	// Utilizar la implementación segura de findInodeByPath
	inodeNum, inode, err := DiskManager.FindInodeByPath(file, startByte, superblock, path)
	if err != nil {
		return -1, nil, err
	}

	return inodeNum, inode, nil
}

// getUserIdFromName obtiene el ID de un usuario a partir de su nombre
func getUserIdFromName(id string, username string) int32 {
	// Implementar función simplificada para obtener ID de usuario
	// Leer /users.txt y buscar el usuario
	content, err := DiskManager.EXT2FileOperation(id, "/users.txt", DiskManager.FILE_READ, "")
	if err != nil {
		return 1 // Default a root si hay error
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}

		// Buscar líneas de usuario: ID, U, nombre, grupo, pass
		if len(parts) >= 4 && parts[1] == "U" && parts[2] == username {
			// El primer elemento es el ID
			id, err := strconv.Atoi(parts[0])
			if err != nil {
				return 1 // Default a root si hay error
			}
			return int32(id)
		}
	}

	return 1 // Default a root si no se encuentra
}

// getGroupIdFromName obtiene el ID de un grupo a partir de su nombre
func getGroupIdFromName(id string, groupname string) int32 {
	// Implementar función simplificada para obtener ID de grupo
	// Leer /users.txt y buscar el grupo
	content, err := DiskManager.EXT2FileOperation(id, "/users.txt", DiskManager.FILE_READ, "")
	if err != nil {
		return 1 // Default a root si hay error
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}

		// Buscar líneas de grupo: ID, G, nombre
		if len(parts) >= 3 && parts[1] == "G" && parts[2] == groupname {
			// El primer elemento es el ID
			id, err := strconv.Atoi(parts[0])
			if err != nil {
				return 1 // Default a root si hay error
			}
			return int32(id)
		}
	}

	return 1 // Default a root si no se encuentra
}
