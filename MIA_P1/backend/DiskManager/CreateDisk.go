package DiskManager

import (
	"MIA_P1/backend/utils"
	"fmt"
	"os"
)

const BUFFER_SIZE = 1024

// CreateDisk crea un archivo binario que simula un disco duro
func CreateDisk(diskConfig utils.DiskConfig) error {
	// Calcular el tamaño exacto en bytes
	var totalBytes int64
	if diskConfig.Unit == "K" {
		totalBytes = int64(diskConfig.Size) * 1000 // Usar 1000 para KB exactos
	} else { // "M" por defecto
		totalBytes = int64(diskConfig.Size) * 1000 * 1000 // Usar 1000*1000 para MB exactos
	}

	// Crear el archivo
	file, err := os.Create(diskConfig.Path)
	if err != nil {
		return fmt.Errorf("error creando archivo de disco: %v", err)
	}
	defer file.Close()

	// Establecer el tamaño exacto del archivo
	if err := file.Truncate(totalBytes); err != nil {
		return fmt.Errorf("error estableciendo tamaño del disco: %v", err)
	}

	// Escribir ceros en el archivo
	buffer := make([]byte, BUFFER_SIZE)
	currentPosition := int64(0)

	for currentPosition < totalBytes {
		writeSize := BUFFER_SIZE
		if remainingBytes := totalBytes - currentPosition; remainingBytes < int64(BUFFER_SIZE) {
			writeSize = int(remainingBytes)
		}

		if _, err := file.Write(buffer[:writeSize]); err != nil {
			return fmt.Errorf("error escribiendo datos al disco: %v", err)
		}

		currentPosition += int64(writeSize)
	}

	return nil
}
