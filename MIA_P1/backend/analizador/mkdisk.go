package analizador

import (
	"MIA_P1/backend/utils"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Error struct {
	Parametro string
	Mensaje   string
}

func AnalizarMkdisk(comando string) (utils.DiskConfig, []Error, bool, string) {
	params := utils.DiskConfig{
		Fit:  "FF",
		Unit: "M",
	}
	var errores []Error

	if !strings.HasPrefix(strings.ToLower(comando), "mkdisk") {
		errores = append(errores, Error{
			Parametro: "comando",
			Mensaje:   "El comando debe comenzar con 'mkdisk'",
		})
		return params, errores, false, ""
	}

	paramRegex := regexp.MustCompile(`-(\w+)=([^-]*(?:"[^"]*")?[^-]*)`)
	matches := paramRegex.FindAllStringSubmatch(comando, -1)

	hasSize := false
	hasPath := false

	// Lista de parámetros válidos para mkdisk
	validParams := map[string]bool{
		"size": true,
		"fit":  true,
		"unit": true,
		"path": true,
	}

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		param := strings.ToLower(match[1])
		value := strings.TrimSpace(match[2])

		// Verificar si el parámetro es válido para mkdisk
		if _, exists := validParams[param]; !exists {
			errores = append(errores, Error{
				Parametro: param,
				Mensaje:   fmt.Sprintf("Parámetro '%s' no válido para el comando mkdisk", param),
			})
			continue
		}

		switch param {
		case "size":
			// Resto del código existente...
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
			valueFit := strings.ToUpper(value)
			if valueFit != "BF" && valueFit != "FF" && valueFit != "WF" {
				errores = append(errores, Error{
					Parametro: "fit",
					Mensaje:   "El valor de fit debe ser BF, FF o WF",
				})
			} else {
				params.Fit = valueFit
			}

		case "unit":
			valueUnit := strings.ToUpper(value)
			if valueUnit != "K" && valueUnit != "M" {
				errores = append(errores, Error{
					Parametro: "unit",
					Mensaje:   "El valor de unit debe ser K o M",
				})
			} else {
				params.Unit = valueUnit
			}

		case "path":
			if value == "" {
				continue
			}

			// Si el valor está entre comillas, eliminarlas
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = value[1 : len(value)-1]
			} else if strings.Contains(value, " ") {
				errores = append(errores, Error{
					Parametro: "path",
					Mensaje:   "Rutas con espacios deben estar entre comillas dobles",
				})
				continue
			}

			// Verificar extensión .mia
			if !strings.HasSuffix(strings.ToLower(value), ".mia") {
				errores = append(errores, Error{
					Parametro: "path",
					Mensaje:   "El archivo debe tener extensión .mia",
				})
				continue
			}

			// Verificar si el disco ya existe en la lista
			if utils.DiskExists(value) {
				errores = append(errores, Error{
					Parametro: "path",
					Mensaje:   fmt.Sprintf("Error: Ya existe un disco con la ruta: %s", value),
				})
				continue
			}

			// Verificar existencia de la ruta
			if rutaValida, mensaje, puedeCrear := utils.ValidarRuta(value); !rutaValida {
				if puedeCrear {
					return params, errores, true, filepath.Dir(value)
				}
				errores = append(errores, Error{
					Parametro: "path",
					Mensaje:   mensaje,
				})
				continue
			}

			params.Path = value
			params.Name, params.Extension = utils.ExtractDiskInfo(value)
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

	return params, errores, false, ""
}
