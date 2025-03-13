package analizador

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type DiskConfig struct {
	Size      int
	Fit       string
	Unit      string
	Path      string
	Name      string // Nombre del disco (extraído del path)
	Extension string // Extensión del archivo (.mia)
}

type Error struct {
	Parametro string
	Mensaje   string
}

// Lista global para mantener registro de discos
var DiscosList = make(map[string]DiskConfig)

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

func DiskExists(path string) bool {
	// Verificar en la lista de discos
	_, exists := DiscosList[path]
	return exists
}

// Funciones y métodos específicos para mkdisk
func AnalizarMkdisk(comando string) (DiskConfig, []Error) {
	params := DiskConfig{
		Fit:  "FF",
		Unit: "M",
	}
	var errores []Error

	if !strings.HasPrefix(strings.ToLower(comando), "mkdisk") {
		errores = append(errores, Error{
			Parametro: "comando",
			Mensaje:   "El comando debe comenzar con 'mkdisk'",
		})
		return params, errores
	}

	paramRegex := regexp.MustCompile(`-(\w+)=([^ ]+)`)
	matches := paramRegex.FindAllStringSubmatch(comando, -1)

	hasSize := false
	hasPath := false

	for _, match := range matches {
		param := strings.ToLower(match[1])
		value := match[2]

		switch param {
		case "size":
			size, err := strconv.Atoi(value)
			if err != nil || size <= 0 {
				errores = append(errores, Error{
					Parametro: "size",
					Mensaje:   "El tamaño debe ser un número positivo mayor que cero",
				})
			} else {
				params.Size = size
				hasSize = true
			}

		case "fit":
			if value != "BF" && value != "FF" && value != "WF" {
				errores = append(errores, Error{
					Parametro: "fit",
					Mensaje:   "El valor de fit debe ser BF, FF o WF",
				})
			} else {
				params.Fit = value
			}

		case "unit":
			if value != "K" && value != "M" {
				errores = append(errores, Error{
					Parametro: "unit",
					Mensaje:   "El valor de unit debe ser K o M",
				})
			} else {
				params.Unit = value
			}

		case "path":
			if strings.Contains(value, " ") {
				if !strings.HasPrefix(value, "\"") || !strings.HasSuffix(value, "\"") {
					errores = append(errores, Error{
						Parametro: "path",
						Mensaje:   "Rutas con espacios deben estar entre comillas dobles",
					})
					continue
				}
				value = value[1 : len(value)-1]
			}

			if !strings.HasSuffix(value, ".mia") {
				errores = append(errores, Error{
					Parametro: "path",
					Mensaje:   "El archivo debe tener extensión .mia",
				})
			}

			// Verificar en la lista de discos
			if DiskExists(value) {
				errores = append(errores, Error{
					Parametro: "path",
					Mensaje:   fmt.Sprintf("Error: Ya existe un disco con la ruta: %s", value),
				})
				return params, errores
			}

			params.Path = value
			params.Name, params.Extension = ExtractDiskInfo(value)
			hasPath = true
		}
	}

	if !hasSize {
		errores = append(errores, Error{
			Parametro: "size",
			Mensaje:   "El parámetro size es obligatorio",
		})
	}
	if !hasPath {
		errores = append(errores, Error{
			Parametro: "path",
			Mensaje:   "El parámetro path es obligatorio",
		})
	}

	return params, errores
}
