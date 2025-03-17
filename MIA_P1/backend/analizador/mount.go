package analizador

import (
	"MIA_P1/backend/utils"
	"fmt"
	"regexp"
	"strings"
)

type MountError struct {
	Parametro string
	Mensaje   string
}

type MountParams struct {
	Path string
	Name string
}

func AnalizarMount(comando string) (MountParams, []MountError, bool) {
	params := MountParams{}
	var errores []MountError

	if !strings.HasPrefix(strings.ToLower(comando), "mount") {
		errores = append(errores, MountError{
			Parametro: "comando",
			Mensaje:   "El comando debe comenzar con 'mount'",
		})
		return params, errores, false
	}

	paramRegex := regexp.MustCompile(`-(\w+)=([^-]*(?:"[^"]*")?[^-]*)`)
	matches := paramRegex.FindAllStringSubmatch(comando, -1)

	hasPath := false
	hasName := false

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		param := strings.ToLower(match[1])
		value := strings.TrimSpace(match[2])

		switch param {
		case "path":
			if value == "" {
				continue
			}

			// Validar comillas en rutas con espacios
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = value[1 : len(value)-1]
			} else if strings.Contains(value, " ") {
				errores = append(errores, MountError{
					Parametro: "path",
					Mensaje:   "Rutas con espacios deben estar entre comillas dobles",
				})
				continue
			}

			// Verificar extensión .mia
			if !strings.HasSuffix(strings.ToLower(value), ".mia") {
				errores = append(errores, MountError{
					Parametro: "path",
					Mensaje:   "El archivo debe tener extensión .mia",
				})
				continue
			}

			// Verificar que el disco exista
			if !utils.DiskExists(value) {
				errores = append(errores, MountError{
					Parametro: "path",
					Mensaje:   fmt.Sprintf("Error: No existe un disco en la ruta: %s", value),
				})
				continue
			}

			params.Path = value
			hasPath = true

		case "name":
			if value == "" {
				errores = append(errores, MountError{
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
	if !hasPath {
		errores = append(errores, MountError{
			Parametro: "path",
			Mensaje:   "El parámetro path es obligatorio",
		})
	}
	if !hasName {
		errores = append(errores, MountError{
			Parametro: "name",
			Mensaje:   "El parámetro name es obligatorio",
		})
	}

	return params, errores, len(errores) == 0
}
