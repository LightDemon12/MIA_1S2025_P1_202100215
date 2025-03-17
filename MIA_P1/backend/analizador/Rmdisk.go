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

			// Si el path comienza con comillas, buscar hasta el cierre
			if strings.HasPrefix(pathValue, "\"") {
				path = pathValue
				// Buscar la comilla de cierre
				for i++; i < len(tokens) && !strings.HasSuffix(path, "\""); i++ {
					path += " " + tokens[i]
				}
				// Eliminar las comillas
				path = strings.Trim(path, "\"")
			} else {
				// Si no tiene comillas, usar el valor directamente
				path = pathValue
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
