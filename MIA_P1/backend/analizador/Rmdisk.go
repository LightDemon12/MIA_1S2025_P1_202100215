package analizador

import (
	"strings"
)

type RmdiskError struct {
	Parametro string
	Mensaje   string
}

func AnalizarRmdisk(comando string) (string, []RmdiskError) {
	var errores []RmdiskError
	var path string

	// Dividir el comando preservando las comillas
	tokens := strings.Split(comando, " ")

	for i := 1; i < len(tokens); i++ {
		token := tokens[i]

		if strings.HasPrefix(token, "-path=") {
			pathValue := strings.TrimPrefix(token, "-path=")

			// Verificar si el path está entre comillas
			if !strings.HasPrefix(pathValue, "\"") {
				errores = append(errores, RmdiskError{
					Parametro: "path",
					Mensaje:   "El path debe estar entre comillas dobles cuando contiene espacios",
				})
				continue
			}

			// Si el path está entre comillas, puede contener espacios
			if strings.HasPrefix(pathValue, "\"") {
				path = pathValue
				// Buscar la comilla de cierre
				for i++; i < len(tokens); i++ {
					path += " " + tokens[i]
					if strings.HasSuffix(tokens[i], "\"") {
						break
					}
				}
				path = strings.Trim(path, "\"")
			}
			continue
		}
	}

	// Verificar parámetro obligatorio
	if path == "" {
		errores = append(errores, RmdiskError{
			Parametro: "path",
			Mensaje:   "El parámetro path es obligatorio",
		})
	}

	return path, errores
}
