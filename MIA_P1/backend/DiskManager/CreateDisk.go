package DiskManager

import (
	"MIA_P1/backend/utils"
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"time"
)

const BUFFER_SIZE = 1024

// CreateDisk crea un archivo binario que simula un disco duro

func CreateDisk(diskConfig utils.DiskConfig) error {
	// Calcular el tamaño exacto en bytes
	var totalBytes int64
	if diskConfig.Unit == "K" {
		totalBytes = int64(diskConfig.Size) * 1000 // Kilobytes a bytes (1K = 1000 bytes)
	} else {
		totalBytes = int64(diskConfig.Size) * 1000 * 1000 // Megabytes a bytes (1M = 1000000 bytes)
	}

	// Crear el archivo
	file, err := os.Create(diskConfig.Path)
	if err != nil {
		return fmt.Errorf("error creando archivo de disco: %v", err)
	}
	defer file.Close()

	// Crear e inicializar el MBR
	mbr := MBR{
		MbrTamanio:      totalBytes,
		MbrDskSignature: rand.Int31(),
		DskFit:          getDiskFit(diskConfig.Fit),
	}

	// Convertir la fecha actual a string y copiarla al array de bytes
	timeStr := time.Now().Format("2006-01-02 15:04:05")
	copy(mbr.MbrFechaCreacion[:], timeStr)

	// Inicializar particiones
	for i := range mbr.MbrPartitions {
		mbr.MbrPartitions[i].Status = '0'
		mbr.MbrPartitions[i].Type = '0'
		mbr.MbrPartitions[i].Fit = 'F'
	}

	// Escribir el MBR al inicio del archivo
	if err := binary.Write(file, binary.LittleEndian, &mbr); err != nil {
		return fmt.Errorf("error escribiendo MBR: %v", err)
	}

	// Establecer el tamaño total del disco
	if err := file.Truncate(totalBytes); err != nil {
		return fmt.Errorf("error estableciendo tamaño del disco: %v", err)
	}

	// Llenar el resto del disco con ceros (después del MBR)
	remainingBytes := totalBytes - int64(binary.Size(mbr))
	if remainingBytes > 0 {
		zeroBuffer := make([]byte, BUFFER_SIZE)
		for remainingBytes > 0 {
			writeSize := BUFFER_SIZE
			if remainingBytes < int64(BUFFER_SIZE) {
				writeSize = int(remainingBytes)
			}
			if _, err := file.Write(zeroBuffer[:writeSize]); err != nil {
				return fmt.Errorf("error escribiendo datos al disco: %v", err)
			}
			remainingBytes -= int64(writeSize)
		}
	}
	// Al final, si la creación fue exitosa, registrar el disco
	RegisterDisk(DiskInfo{
		Path: diskConfig.Path,
		Name: diskConfig.Name,
		Size: diskConfig.Size,
		Unit: diskConfig.Unit,
	})
	return nil
}

// Función auxiliar para obtener el tipo de ajuste
func getDiskFit(fit string) byte {
	switch fit {
	case "BF":
		return 'B'
	case "WF":
		return 'W'
	default:
		return 'F'
	}
}
