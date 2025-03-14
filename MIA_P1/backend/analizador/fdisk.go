package analizador

import (
	"MIA_P1/backend/utils"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type FdiskError struct {
	Parametro string
	Mensaje   string
}

func AnalizarFdisk(comando string) (utils.PartitionConfig, []FdiskError, bool) {
	params := utils.NewPartitionConfig()
	var errores []FdiskError

	if !strings.HasPrefix(strings.ToLower(comando), "fdisk") {
		errores = append(errores, FdiskError{
			Parametro: "comando",
			Mensaje:   "El comando debe comenzar con 'fdisk'",
		})
		return params, errores, false
	}

	paramRegex := regexp.MustCompile(`-(\w+)=([^-]*(?:"[^"]*")?[^-]*)`)
	matches := paramRegex.FindAllStringSubmatch(comando, -1)

	hasSize := false
	hasPath := false
	hasName := false

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		param := strings.ToLower(match[1])
		value := strings.TrimSpace(match[2])

		switch param {
		case "size":
			size, err := strconv.Atoi(value)
			if err != nil || size <= 0 {
				errores = append(errores, FdiskError{
					Parametro: "size",
					Mensaje:   "El tamaño debe ser un número positivo mayor que cero",
				})
			} else {
				params.Size = size
				hasSize = true
			}

		case "unit":
			valueUnit := strings.ToUpper(value)
			if valueUnit != "B" && valueUnit != "K" && valueUnit != "M" {
				errores = append(errores, FdiskError{
					Parametro: "unit",
					Mensaje:   "El valor de unit debe ser B, K o M",
				})
			} else {
				params.Unit = valueUnit
			}

		case "type":
			valueType := strings.ToUpper(value)
			if valueType != "P" && valueType != "E" && valueType != "L" {
				errores = append(errores, FdiskError{
					Parametro: "type",
					Mensaje:   "El valor de type debe ser P, E o L",
				})
			} else {
				params.Type = valueType
			}

		case "fit":
			valueFit := strings.ToUpper(value)
			if valueFit != "BF" && valueFit != "FF" && valueFit != "WF" {
				errores = append(errores, FdiskError{
					Parametro: "fit",
					Mensaje:   "El valor de fit debe ser BF, FF o WF",
				})
			} else {
				params.Fit = valueFit
			}

		case "path":
			if value == "" {
				continue
			}

			// Validar comillas en rutas con espacios
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = value[1 : len(value)-1]
			} else if strings.Contains(value, " ") {
				errores = append(errores, FdiskError{
					Parametro: "path",
					Mensaje:   "Rutas con espacios deben estar entre comillas dobles",
				})
				continue
			}

			// Verificar extensión .mia
			if !strings.HasSuffix(strings.ToLower(value), ".mia") {
				errores = append(errores, FdiskError{
					Parametro: "path",
					Mensaje:   "El archivo debe tener extensión .mia",
				})
				continue
			}

			// Verificar que el disco exista
			if !utils.DiskExists(value) {
				errores = append(errores, FdiskError{
					Parametro: "path",
					Mensaje:   fmt.Sprintf("Error: No existe un disco en la ruta: %s", value),
				})
				continue
			}

			params.Path = value
			hasPath = true

		case "name":
			if value == "" {
				errores = append(errores, FdiskError{
					Parametro: "name",
					Mensaje:   "El nombre de la partición no puede estar vacío",
				})
				continue
			}
			params.Name = value
			hasName = true
		}
	}

	// Verificar parámetros obligatorios
	if !hasSize {
		errores = append(errores, FdiskError{
			Parametro: "size",
			Mensaje:   "El parámetro size es obligatorio",
		})
	}
	if !hasPath {
		errores = append(errores, FdiskError{
			Parametro: "path",
			Mensaje:   "El parámetro path es obligatorio",
		})
	}
	if !hasName {
		errores = append(errores, FdiskError{
			Parametro: "name",
			Mensaje:   "El parámetro name es obligatorio",
		})
	}

	return params, errores, false
}
