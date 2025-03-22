package analizador

import (
	"regexp"
	"strings"
)

// CatParams contiene los parámetros para el comando cat
type CatParams struct {
	Files []string // Lista de rutas de archivos a mostrar
}

// ValidarCat valida los parámetros del comando cat
func ValidarCat(comando string) (*CatParams, []Error) {
	var errores []Error
	var files []string

	// Expresión regular para detectar parámetros file1, file2, etc.
	fileParamRegex := regexp.MustCompile(`-file(\d+)`)

	// Dividir el comando en tokens
	tokens := strings.Split(comando, " ")

	// Ignorar el primer token (cat)
	for i := 1; i < len(tokens); i++ {
		token := strings.TrimSpace(tokens[i])

		// Ignorar tokens vacíos
		if token == "" {
			continue
		}

		// Buscar parámetros de tipo -fileN o -fileN=
		match := fileParamRegex.FindStringSubmatch(token)
		if len(match) > 0 {
			// Es un parámetro de archivo
			paramValue := ""

			// Manejar formato -fileN=ruta
			if strings.Contains(token, "=") {
				parts := strings.SplitN(token, "=", 2)
				paramValue = parts[1]

				// Eliminar comillas si existen
				if strings.HasPrefix(paramValue, "\"") && strings.HasSuffix(paramValue, "\"") {
					paramValue = paramValue[1 : len(paramValue)-1]
				}
			} else {
				// Formato -fileN ruta
				if i+1 >= len(tokens) {
					errores = append(errores, Error{
						Parametro: token,
						Mensaje:   "Falta la ruta del archivo",
					})
					continue
				}

				paramValue = tokens[i+1]
				if strings.HasPrefix(paramValue, "-") {
					errores = append(errores, Error{
						Parametro: token,
						Mensaje:   "Falta la ruta del archivo",
					})
					continue
				}

				// Eliminar comillas si existen
				if strings.HasPrefix(paramValue, "\"") && strings.HasSuffix(paramValue, "\"") {
					paramValue = paramValue[1 : len(paramValue)-1]
				}

				i++ // Saltar el siguiente token
			}

			files = append(files, paramValue)
		} else if strings.HasPrefix(token, "-") {
			errores = append(errores, Error{
				Parametro: token,
				Mensaje:   "Parámetro no reconocido para cat",
			})
		}
	}

	// Validar que al menos haya un archivo
	if len(files) == 0 {
		errores = append(errores, Error{
			Parametro: "file",
			Mensaje:   "Debe especificar al menos un archivo",
		})
	}

	return &CatParams{
		Files: files,
	}, errores
}
