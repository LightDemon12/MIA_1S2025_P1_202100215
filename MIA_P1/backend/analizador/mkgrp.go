// validarMkgrp.go
package analizador

import (
	"strings"
)

// MkgrpParams contiene los parámetros para el comando mkgrp
type MkgrpParams struct {
	Name string
}

// ValidarMkgrp valida los parámetros del comando mkgrp
func ValidarMkgrp(comando string) (*MkgrpParams, []Error) {
	var errores []Error
	var name string

	// Dividir el comando en tokens
	tokens := strings.Split(comando, " ")

	// Ignorar el primer token (mkgrp)
	for i := 1; i < len(tokens); i++ {
		token := strings.TrimSpace(tokens[i])

		// Ignorar tokens vacíos
		if token == "" {
			continue
		}

		// Verificar si el parámetro usa el formato -param=valor
		if strings.HasPrefix(token, "-") && strings.Contains(token, "=") {
			parts := strings.SplitN(token, "=", 2)
			paramName := strings.ToLower(strings.TrimPrefix(parts[0], "-"))
			paramValue := parts[1]

			// Eliminar comillas si existen
			if strings.HasPrefix(paramValue, "\"") && strings.HasSuffix(paramValue, "\"") {
				paramValue = paramValue[1 : len(paramValue)-1]
			}

			switch paramName {
			case "name":
				name = paramValue
			default:
				errores = append(errores, Error{
					Parametro: paramName,
					Mensaje:   "Parámetro no reconocido para mkgrp",
				})
			}
		} else if strings.HasPrefix(token, "-") {
			// Formato -param valor
			paramName := strings.ToLower(strings.TrimPrefix(token, "-"))

			// Verificar que hay un valor después
			if i+1 >= len(tokens) {
				errores = append(errores, Error{
					Parametro: paramName,
					Mensaje:   "Falta valor para el parámetro",
				})
				continue
			}

			paramValue := strings.TrimSpace(tokens[i+1])
			if strings.HasPrefix(paramValue, "-") {
				errores = append(errores, Error{
					Parametro: paramName,
					Mensaje:   "Falta valor para el parámetro",
				})
				continue
			}

			// Eliminar comillas si existen
			if strings.HasPrefix(paramValue, "\"") && strings.HasSuffix(paramValue, "\"") {
				paramValue = paramValue[1 : len(paramValue)-1]
			}

			i++ // Avanzar para saltarse el valor

			switch paramName {
			case "name":
				name = paramValue
			default:
				errores = append(errores, Error{
					Parametro: paramName,
					Mensaje:   "Parámetro no reconocido para mkgrp",
				})
			}
		}
	}

	// Validar parámetros obligatorios
	if name == "" {
		errores = append(errores, Error{
			Parametro: "name",
			Mensaje:   "El parámetro name es obligatorio",
		})
	}

	if len(errores) > 0 {
		return nil, errores
	}

	return &MkgrpParams{
		Name: name,
	}, nil
}
