package analizador

import (
	"fmt"
	"strings"
)

type MkdirParams struct {
	Path       string
	CreateDirs bool // Parámetro -p
}

func ValidarMkdir(comando string) (*MkdirParams, []Error) {
	var errores []Error
	var path string
	var createDirs bool

	// Usar el mismo procesador de comandos que mkfile para manejar espacios
	tokens := tokenizarComando(comando)
	fmt.Printf("DEBUG: Tokens procesados: %v\n", tokens)

	// Ignorar el primer token (mkdir)
	for i := 1; i < len(tokens); i++ {
		token := strings.TrimSpace(tokens[i])

		// Ignorar tokens vacíos
		if token == "" {
			continue
		}

		// Verificar si es el parámetro -p
		if token == "-p" {
			createDirs = true
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
			case "path":
				// Preservar el path completo con espacios
				if strings.Contains(paramValue, " ") {
					// Si no tiene comillas, añadirlas
					if !strings.HasPrefix(paramValue, "\"") {
						paramValue = "\"" + paramValue + "\""
					}
				}
				path = paramValue
				fmt.Printf("DEBUG: Path procesado: '%s'\n", path)
			case "p":
				errores = append(errores, Error{
					Parametro: "p",
					Mensaje:   "El parámetro p no debe tener un valor asignado",
				})
			default:
				errores = append(errores, Error{
					Parametro: paramName,
					Mensaje:   "Parámetro no reconocido para mkdir",
				})
			}
		}
	}

	// Validar parámetros obligatorios
	if path == "" {
		errores = append(errores, Error{
			Parametro: "path",
			Mensaje:   "El parámetro path es obligatorio",
		})
	}

	if len(errores) > 0 {
		return nil, errores
	}

	return &MkdirParams{
		Path:       path,
		CreateDirs: createDirs,
	}, nil
}
