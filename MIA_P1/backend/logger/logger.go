package logger

import (
	"MIA_P1/backend/DiskManager"
	"encoding/binary"
	"fmt"
	"os"
)

// LogMBR imprime la información del MBR para propósitos de debugging
func LogMBR(path string) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("Error abriendo archivo: %v\n", err)
		return
	}
	defer file.Close()

	var mbr DiskManager.MBR
	if err := binary.Read(file, binary.LittleEndian, &mbr); err != nil {
		fmt.Printf("Error leyendo MBR: %v\n", err)
		return
	}

	fmt.Printf("\n=== MBR INFO ===\n")
	fmt.Printf("Tamaño: %d bytes\n", mbr.MbrTamanio)
	fmt.Printf("Fecha: %s\n", string(mbr.MbrFechaCreacion[:]))
	fmt.Printf("Signature: %d\n", mbr.MbrDskSignature)
	fmt.Printf("Fit: %c\n", mbr.DskFit)

	fmt.Printf("\n=== PARTICIONES ===\n")
	for i, part := range mbr.MbrPartitions {
		fmt.Printf("\nPartición %d:\n", i+1)
		fmt.Printf("Status: %c\n", part.Status)
		fmt.Printf("Type: %c\n", part.Type)
		fmt.Printf("Fit: %c\n", part.Fit)
		fmt.Printf("Start: %d\n", part.Start)
		fmt.Printf("Size: %d\n", part.Size)
		fmt.Printf("Name: %s\n", string(part.Name[:]))
	}
}
