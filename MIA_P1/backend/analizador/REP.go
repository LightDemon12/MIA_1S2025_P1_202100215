package analizador

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type RepError struct {
	Parametro string
	Mensaje   string
}

type RepParams struct {
	Name       string
	Path       string
	ID         string
	PathFileLS string // Opcional
}

// Constantes para los tipos de reportes válidos
var validReportTypes = map[string]bool{
	"mbr":      true,
	"disk":     true,
	"inode":    true,
	"block":    true,
	"bm_inode": true,
	"bm_block": true,
	"tree":     true,
	"sb":       true,
	"file":     true,
	"ls":       true,
}

func AnalizarRep(comando string) (RepParams, []RepError, bool, bool, string) {
	params := RepParams{}
	var errores []RepError

	if !strings.HasPrefix(strings.ToLower(comando), "rep") {
		errores = append(errores, RepError{
			Parametro: "comando",
			Mensaje:   "El comando debe comenzar con 'rep'",
		})
		return params, errores, false, false, ""
	}

	// Expresión regular para extraer parámetros
	paramRegex := regexp.MustCompile(`-(\w+)=([^-]*(?:"[^"]*")?[^-]*)`)
	matches := paramRegex.FindAllStringSubmatch(comando, -1)

	hasName := false
	hasPath := false
	hasID := false

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		param := strings.ToLower(match[1])
		value := strings.TrimSpace(match[2])

		switch param {
		case "name":
			if value == "" {
				errores = append(errores, RepError{
					Parametro: "name",
					Mensaje:   "El nombre del reporte no puede estar vacío",
				})
				continue
			}

			// Verificar que sea un tipo de reporte válido
			if !validReportTypes[strings.ToLower(value)] {
				errores = append(errores, RepError{
					Parametro: "name",
					Mensaje:   fmt.Sprintf("Tipo de reporte no válido: %s", value),
				})
				continue
			}

			params.Name = strings.ToLower(value)
			hasName = true

		case "path":
			if value == "" {
				errores = append(errores, RepError{
					Parametro: "path",
					Mensaje:   "La ruta del reporte no puede estar vacía",
				})
				continue
			}

			// Manejar comillas en rutas
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = value[1 : len(value)-1]
			} else if strings.Contains(value, " ") {
				errores = append(errores, RepError{
					Parametro: "path",
					Mensaje:   "Rutas con espacios deben estar entre comillas dobles",
				})
				continue
			}

			// Verificar extensión del archivo de salida
			if !strings.HasSuffix(strings.ToLower(value), ".png") &&
				!strings.HasSuffix(strings.ToLower(value), ".jpg") &&
				!strings.HasSuffix(strings.ToLower(value), ".pdf") {
				value += ".png" // Agregar extensión por defecto
			}

			params.Path = value
			hasPath = true

		case "id":
			if value == "" {
				errores = append(errores, RepError{
					Parametro: "id",
					Mensaje:   "El ID de la partición no puede estar vacío",
				})
				continue
			}

			// La verificación de si el ID existe se hará en la fase de ejecución
			params.ID = value
			hasID = true

		case "path_file_ls":
			if value == "" {
				errores = append(errores, RepError{
					Parametro: "path_file_ls",
					Mensaje:   "La ruta del archivo o carpeta no puede estar vacía",
				})
				continue
			}

			// Manejar comillas en rutas
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = value[1 : len(value)-1]
			} else if strings.Contains(value, " ") {
				errores = append(errores, RepError{
					Parametro: "path_file_ls",
					Mensaje:   "Rutas con espacios deben estar entre comillas dobles",
				})
				continue
			}

			// Si el reporte no es 'file' o 'ls', mostrar advertencia
			if params.Name != "file" && params.Name != "ls" {
				fmt.Printf("Advertencia: El parámetro path_file_ls solo es válido para reportes 'file' y 'ls'\n")
			}

			params.PathFileLS = value
		}
	}

	// Verificar parámetros obligatorios
	if !hasName {
		errores = append(errores, RepError{
			Parametro: "name",
			Mensaje:   "El parámetro name es obligatorio",
		})
	}
	if !hasPath {
		errores = append(errores, RepError{
			Parametro: "path",
			Mensaje:   "El parámetro path es obligatorio",
		})
	}
	if !hasID {
		errores = append(errores, RepError{
			Parametro: "id",
			Mensaje:   "El parámetro id es obligatorio",
		})
	}

	// Verificar si path_file_ls es obligatorio para reportes file y ls
	if (params.Name == "file" || params.Name == "ls") && params.PathFileLS == "" {
		fmt.Printf("Advertencia: El parámetro path_file_ls es recomendado para reportes '%s'\n", params.Name)
	}

	// CAMBIO: Verificar si se necesita confirmación para crear directorio
	if hasPath && params.Path != "" {
		dir := filepath.Dir(params.Path)
		if dir != "" && dir != "." {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				// Retornar que se necesita confirmación para crear el directorio
				fmt.Printf("Directorio no existe, requiere confirmación: %s\n", dir)
				return params, errores, len(errores) == 0, true, dir
			}
		}
	}

	// No se necesita confirmación
	return params, errores, len(errores) == 0, false, ""
}
