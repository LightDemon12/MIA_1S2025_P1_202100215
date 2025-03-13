package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DiskConfig estructura para la configuración de discos
type DiskConfig struct {
	Size      int
	Fit       string
	Unit      string
	Path      string
	Name      string
	Extension string
}

// Lista global para mantener registro de discos
var DiscosList = make(map[string]DiskConfig)

// ValidarRuta verifica si existe una ruta y si es un directorio válido
func ValidarRuta(path string) (bool, string, bool) {
	cleanPath := strings.Trim(path, "\"")
	dir := filepath.Dir(cleanPath)

	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Sprintf("La ruta no existe: %s", dir), true
		}
		return false, fmt.Sprintf("Error al verificar la ruta: %s", err), false
	}

	if !info.IsDir() {
		return false, fmt.Sprintf("La ruta no es un directorio válido: %s", dir), false
	}

	return true, "", false
}

// CrearDirectorio crea un directorio y sus padres si no existen
func CrearDirectorio(path string) error {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return fmt.Errorf("error creando directorio: %v", err)
	}
	return nil
}

// ExtractDiskInfo extrae el nombre y extensión de un path
func ExtractDiskInfo(path string) (diskName, extension string) {
	lastSlash := strings.LastIndex(path, "/")
	fullName := path
	if lastSlash != -1 {
		fullName = path[lastSlash+1:]
	}

	lastDot := strings.LastIndex(fullName, ".")
	if lastDot != -1 {
		diskName = fullName[:lastDot]
		extension = fullName[lastDot:]
	}

	return diskName, extension
}

// DiskExists verifica si un disco ya existe en la lista
func DiskExists(path string) bool {
	_, exists := DiscosList[path]
	return exists
}
