// validarChgrp.go
package analizador

import (
	"strings"
)

// ChgrpParams contiene los parámetros para el comando chgrp
type ChgrpParams struct {
	User  string
	Group string
}

// ValidarChgrp valida los parámetros del comando chgrp
func ValidarChgrp(comando string) (*ChgrpParams, []Error) {
	var errores []Error
	var user, group string

	// Dividir el comando en tokens
	tokens := strings.Split(comando, " ")

	// Ignorar el primer token (chgrp)
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
			case "user":
				user = paramValue
			case "grp":
				group = paramValue
			default:
				errores = append(errores, Error{
					Parametro: paramName,
					Mensaje:   "Parámetro no reconocido para chgrp",
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
			case "user":
				user = paramValue
			case "grp":
				group = paramValue
			default:
				errores = append(errores, Error{
					Parametro: paramName,
					Mensaje:   "Parámetro no reconocido para chgrp",
				})
			}
		}
	}

	// Validar parámetros obligatorios
	if user == "" {
		errores = append(errores, Error{
			Parametro: "user",
			Mensaje:   "El parámetro user es obligatorio",
		})
	}

	if group == "" {
		errores = append(errores, Error{
			Parametro: "grp",
			Mensaje:   "El parámetro grp es obligatorio",
		})
	}

	if len(errores) > 0 {
		return nil, errores
	}

	return &ChgrpParams{
		User:  user,
		Group: group,
	}, nil
}
