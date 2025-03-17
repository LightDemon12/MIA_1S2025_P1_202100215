package DiskManager

import (
	"encoding/binary"
	"fmt"
	"os"
)

// LogMBR imprime la información del MBR para propósitos de debugging
func LogMBR(diskPath string) {
	file, err := os.OpenFile(diskPath, os.O_RDONLY, 0666)
	if err != nil {
		fmt.Printf("Error al abrir disco para verificación: %v\n", err)
		return
	}
	defer file.Close()

	mbr := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, mbr); err != nil {
		fmt.Printf("Error al leer MBR para verificación: %v\n", err)
		return
	}

	fmt.Println("=== VERIFICACIÓN MBR ===")
	fmt.Printf("Tamaño: %d bytes\n", mbr.MbrTamanio)
	fmt.Printf("Fecha: %s\n", string(mbr.MbrFechaCreacion[:]))
	fmt.Printf("Signature: %d\n", mbr.MbrDskSignature)
	fmt.Printf("Fit: %c\n\n", mbr.DskFit)

	for i, p := range mbr.MbrPartitions {
		fmt.Printf("Partición %d:\n", i+1)
		fmt.Printf("Status: %d\n", p.Status)
		fmt.Printf("Type: %c\n", p.Type)
		fmt.Printf("Fit: %c\n", p.Fit)
		fmt.Printf("Start: %d\n", p.Start)
		fmt.Printf("Size: %d\n", p.Size)
		fmt.Printf("Name: %s\n\n", string(p.Name[:]))
	}
}
