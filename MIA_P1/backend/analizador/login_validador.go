package analizador

import (
	"strings"
)

// LoginParams contiene los parámetros para el comando login
type LoginParams struct {
	User string
	Pass string
	ID   string
}

// ValidarLogin valida los parámetros del comando login
func ValidarLogin(comando string) (*LoginParams, []Error) {
	var errores []Error
	var user, pass, id string

	// Dividir el comando en tokens
	tokens := strings.Split(comando, " ")

	// Ignorar el primer token (login)
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

			switch paramName {
			case "user":
				user = paramValue
			case "pass":
				pass = paramValue
			case "id":
				id = paramValue
			default:
				errores = append(errores, Error{
					Parametro: paramName,
					Mensaje:   "Parámetro no reconocido para login",
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
			i++ // Avanzar para saltarse el valor

			switch paramName {
			case "user":
				user = paramValue
			case "pass":
				pass = paramValue
			case "id":
				id = paramValue
			default:
				errores = append(errores, Error{
					Parametro: paramName,
					Mensaje:   "Parámetro no reconocido para login",
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

	if pass == "" {
		errores = append(errores, Error{
			Parametro: "pass",
			Mensaje:   "El parámetro pass es obligatorio",
		})
	}

	if id == "" {
		errores = append(errores, Error{
			Parametro: "id",
			Mensaje:   "El parámetro id es obligatorio",
		})
	}

	if len(errores) > 0 {
		return nil, errores
	}

	return &LoginParams{
		User: user,
		Pass: pass,
		ID:   id,
	}, errores
}
