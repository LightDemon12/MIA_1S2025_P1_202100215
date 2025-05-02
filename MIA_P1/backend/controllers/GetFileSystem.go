package controllers

import (
	"MIA_P1/backend/DiskManager"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// GetFileSystem obtiene la estructura completa del sistema de archivos de una partición montada
func GetFileSystem(c *gin.Context) {
	id := c.Query("id")

	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"mensaje": "Se requiere el parámetro 'id' para identificar la partición montada",
			"exito":   false,
		})
		return
	}

	// Verificar si la partición está montada
	isMounted, err := DiskManager.IsPartitionMounted(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"mensaje": fmt.Sprintf("Error verificando partición: %v", err),
			"exito":   false,
		})
		return
	}

	if !isMounted {
		c.JSON(http.StatusBadRequest, gin.H{
			"mensaje": fmt.Sprintf("La partición con ID '%s' no está montada", id),
			"exito":   false,
		})
		return
	}

	// Obtener la estructura del sistema de archivos
	fsInfo, err := DiskManager.GetFileSystemStructure(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"mensaje": fmt.Sprintf("Error obteniendo estructura del sistema de archivos: %v", err),
			"exito":   false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"fileSystem": fsInfo,
		"exito":      true,
	})
}

// GetFileContent obtiene únicamente el contenido de un archivo específico
func GetFileContent(c *gin.Context) {
	id := c.Query("id")
	path := c.Query("path")

	if id == "" || path == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"mensaje": "Se requieren los parámetros 'id' y 'path'",
			"exito":   false,
		})
		return
	}

	// Verificar si la partición está montada
	isMounted, err := DiskManager.IsPartitionMounted(id)
	if err != nil || !isMounted {
		c.JSON(http.StatusBadRequest, gin.H{
			"mensaje": fmt.Sprintf("La partición con ID '%s' no está montada", id),
			"exito":   false,
		})
		return
	}

	// Obtener la estructura completa para buscar el archivo
	fsInfo, err := DiskManager.GetFileSystemStructure(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"mensaje": fmt.Sprintf("Error obteniendo estructura del sistema de archivos: %v", err),
			"exito":   false,
		})
		return
	}

	// Buscar el archivo en la estructura - función recursiva auxiliar
	var findFile func(node *DiskManager.FSNode, targetPath string) *DiskManager.FSNode
	findFile = func(node *DiskManager.FSNode, targetPath string) *DiskManager.FSNode {
		if node == nil {
			return nil
		}

		if node.Path == targetPath {
			return node
		}

		// Buscar en hijos si es un directorio
		if node.Type == "directory" && node.Children != nil {
			for _, child := range node.Children {
				result := findFile(child, targetPath)
				if result != nil {
					return result
				}
			}
		}

		return nil
	}

	fileNode := findFile(fsInfo.RootNode, path)

	if fileNode == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"mensaje": fmt.Sprintf("Archivo no encontrado: %s", path),
			"exito":   false,
		})
		return
	}

	if fileNode.Type != "file" {
		c.JSON(http.StatusBadRequest, gin.H{
			"mensaje": fmt.Sprintf("El path especificado no es un archivo: %s", path),
			"exito":   false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"nombre":       fileNode.Name,
		"path":         fileNode.Path,
		"tamaño":       fileNode.Size,
		"contenido":    fileNode.Content,
		"propietario":  fileNode.Owner,
		"grupo":        fileNode.Group,
		"permisos":     fileNode.Permissions,
		"creadoEn":     fileNode.CreatedAt,
		"modificadoEn": fileNode.ModifiedAt,
		"exito":        true,
	})
}

// ListDirectory obtiene el contenido de un directorio específico
func ListDirectory(c *gin.Context) {
	id := c.Query("id")
	path := c.Query("path")

	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"mensaje": "Se requiere el parámetro 'id'",
			"exito":   false,
		})
		return
	}

	// Si no se proporciona path, usar el directorio raíz
	if path == "" {
		path = "/"
	}

	// Verificar si la partición está montada
	isMounted, err := DiskManager.IsPartitionMounted(id)
	if err != nil || !isMounted {
		c.JSON(http.StatusBadRequest, gin.H{
			"mensaje": fmt.Sprintf("La partición con ID '%s' no está montada", id),
			"exito":   false,
		})
		return
	}

	// Obtener la estructura completa para buscar el directorio
	fsInfo, err := DiskManager.GetFileSystemStructure(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"mensaje": fmt.Sprintf("Error obteniendo estructura del sistema de archivos: %v", err),
			"exito":   false,
		})
		return
	}

	// Buscar el directorio en la estructura - función recursiva auxiliar
	var findDirectory func(node *DiskManager.FSNode, targetPath string) *DiskManager.FSNode
	findDirectory = func(node *DiskManager.FSNode, targetPath string) *DiskManager.FSNode {
		if node == nil {
			return nil
		}

		if node.Path == targetPath {
			return node
		}

		// Buscar en hijos solo si es un directorio
		if node.Type == "directory" && node.Children != nil {
			for _, child := range node.Children {
				result := findDirectory(child, targetPath)
				if result != nil {
					return result
				}
			}
		}

		return nil
	}

	dirNode := findDirectory(fsInfo.RootNode, path)

	if dirNode == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"mensaje": fmt.Sprintf("Directorio no encontrado: %s", path),
			"exito":   false,
		})
		return
	}

	if dirNode.Type != "directory" {
		c.JSON(http.StatusBadRequest, gin.H{
			"mensaje": fmt.Sprintf("El path especificado no es un directorio: %s", path),
			"exito":   false,
		})
		return
	}

	// Preparar respuesta simpificada con solo la información importante
	type SimplifiedFSNode struct {
		Name        string    `json:"name"`
		Type        string    `json:"type"`
		Path        string    `json:"path"`
		Size        int32     `json:"size"`
		Permissions string    `json:"permissions"`
		Owner       string    `json:"owner"`
		Group       string    `json:"group"`
		ModifiedAt  time.Time `json:"modifiedAt"`
	}

	children := make([]SimplifiedFSNode, 0)

	for _, child := range dirNode.Children {
		children = append(children, SimplifiedFSNode{
			Name:        child.Name,
			Type:        child.Type,
			Path:        child.Path,
			Size:        child.Size,
			Permissions: child.Permissions,
			Owner:       child.Owner,
			Group:       child.Group,
			ModifiedAt:  child.ModifiedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"directorio": dirNode.Path,
		"nombre":     dirNode.Name,
		"contenido":  children,
		"total":      len(children),
		"exito":      true,
	})
}
