// validarMkfile.go
package analizador

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// MkfileParams contiene los parámetros para el comando mkfile
type MkfileParams struct {
	Path       string
	CreateDirs bool // Parámetro -r
	Size       int  // -1 significa que no se proporcionó
	Cont       string
}

// ValidarMkfile valida los parámetros del comando mkfile
func ValidarMkfile(comando string) (*MkfileParams, []Error) {
	var errores []Error
	var path string
	var createDirs bool
	var size = -1
	var cont string

	// Dividir el comando en tokens
	tokens := strings.Split(comando, " ")

	// Ignorar el primer token (mkfile)
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
			case "path":
				path = paramValue
			case "size":
				val, err := strconv.Atoi(paramValue)
				if err != nil {
					errores = append(errores, Error{
						Parametro: "size",
						Mensaje:   "El valor debe ser un número entero",
					})
				} else if val < 0 {
					errores = append(errores, Error{
						Parametro: "size",
						Mensaje:   "El tamaño no puede ser negativo",
					})
				} else {
					size = val
				}
			case "cont":
				cont = paramValue
				// Verificar si el archivo existe en el disco local
				if _, err := os.Stat(cont); os.IsNotExist(err) {
					errores = append(errores, Error{
						Parametro: "cont",
						Mensaje:   fmt.Sprintf("El archivo '%s' no existe en el disco local", cont),
					})
				}
			case "r":
				errores = append(errores, Error{
					Parametro: "r",
					Mensaje:   "El parámetro r no debe tener un valor asignado",
				})
			default:
				errores = append(errores, Error{
					Parametro: paramName,
					Mensaje:   "Parámetro no reconocido para mkfile",
				})
			}
		} else if strings.HasPrefix(token, "-") {
			// Formato -param o -param valor
			paramName := strings.ToLower(strings.TrimPrefix(token, "-"))

			if paramName == "r" {
				createDirs = true
				continue
			}

			// Verificar que hay un valor después para los otros parámetros
			if i+1 >= len(tokens) {
				errores = append(errores, Error{
					Parametro: paramName,
					Mensaje:   "Falta valor para el parámetro",
				})
				continue
			}

			paramValue := strings.TrimSpace(tokens[i+1])
			if strings.HasPrefix(paramValue, "-") && !strings.HasPrefix(paramValue, "-\"") {
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
			case "path":
				path = paramValue
			case "size":
				val, err := strconv.Atoi(paramValue)
				if err != nil {
					errores = append(errores, Error{
						Parametro: "size",
						Mensaje:   "El valor debe ser un número entero",
					})
				} else if val < 0 {
					errores = append(errores, Error{
						Parametro: "size",
						Mensaje:   "El tamaño no puede ser negativo",
					})
				} else {
					size = val
				}
			case "cont":
				cont = paramValue
				// Verificar si el archivo existe en el disco local
				if _, err := os.Stat(cont); os.IsNotExist(err) {
					errores = append(errores, Error{
						Parametro: "cont",
						Mensaje:   fmt.Sprintf("El archivo '%s' no existe en el disco local", cont),
					})
				}
			default:
				errores = append(errores, Error{
					Parametro: paramName,
					Mensaje:   "Parámetro no reconocido para mkfile",
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

	return &MkfileParams{
		Path:       path,
		CreateDirs: createDirs,
		Size:       size,
		Cont:       cont,
	}, nil
}
