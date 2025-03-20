// validarMkusr.go
package analizador

import (
	"strings"
)

// MkusrParams contiene los parámetros para el comando mkusr
type MkusrParams struct {
	User  string
	Pass  string
	Group string
}

// ValidarMkusr valida los parámetros del comando mkusr
func ValidarMkusr(comando string) (*MkusrParams, []Error) {
	var errores []Error
	var user, pass, group string

	// Dividir el comando en tokens
	tokens := strings.Split(comando, " ")

	// Ignorar el primer token (mkusr)
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
			case "pass":
				pass = paramValue
			case "grp":
				group = paramValue
			default:
				errores = append(errores, Error{
					Parametro: paramName,
					Mensaje:   "Parámetro no reconocido para mkusr",
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
			case "pass":
				pass = paramValue
			case "grp":
				group = paramValue
			default:
				errores = append(errores, Error{
					Parametro: paramName,
					Mensaje:   "Parámetro no reconocido para mkusr",
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
	} else if len(user) > 10 {
		errores = append(errores, Error{
			Parametro: "user",
			Mensaje:   "El nombre de usuario no puede exceder los 10 caracteres",
		})
	}

	if pass == "" {
		errores = append(errores, Error{
			Parametro: "pass",
			Mensaje:   "El parámetro pass es obligatorio",
		})
	} else if len(pass) > 10 {
		errores = append(errores, Error{
			Parametro: "pass",
			Mensaje:   "La contraseña no puede exceder los 10 caracteres",
		})
	}

	if group == "" {
		errores = append(errores, Error{
			Parametro: "grp",
			Mensaje:   "El parámetro grp es obligatorio",
		})
	} else if len(group) > 10 {
		errores = append(errores, Error{
			Parametro: "grp",
			Mensaje:   "El nombre del grupo no puede exceder los 10 caracteres",
		})
	}

	if len(errores) > 0 {
		return nil, errores
	}

	return &MkusrParams{
		User:  user,
		Pass:  pass,
		Group: group,
	}, nil
}
