package DiskManager

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// BasicPartitionInfo contiene la información básica de una partición y su estado de montaje
type BasicPartitionInfo struct {
	DiskPath   string  `json:"diskPath"`
	DiskName   string  `json:"diskName"`
	Name       string  `json:"name"`
	Type       byte    `json:"type"`     // 'P', 'E', 'L'
	TypeName   string  `json:"typeName"` // "Primaria", "Extendida", "Lógica"
	Size       int64   `json:"size"`
	Start      int64   `json:"start"`
	Status     byte    `json:"status"`
	Fit        byte    `json:"fit"`
	IsMounted  bool    `json:"isMounted"`
	MountID    string  `json:"mountId,omitempty"`
	Percentage float64 `json:"percentage"`
}

// GetAllPartitionsInfo obtiene información básica de todas las particiones
func GetAllPartitionsInfo() ([]BasicPartitionInfo, error) {
	disks := GetAllDisks()
	var allPartitions []BasicPartitionInfo

	for _, disk := range disks {
		partitions, err := GetDiskPartitionsInfo(disk.Path)
		if err != nil {
			fmt.Printf("Error analizando particiones del disco %s: %v\n", disk.Path, err)
			continue
		}
		allPartitions = append(allPartitions, partitions...)
	}

	return allPartitions, nil
}

// GetDiskPartitionsInfo obtiene información básica de las particiones de un disco
func GetDiskPartitionsInfo(diskPath string) ([]BasicPartitionInfo, error) {
	// Abrir el archivo del disco
	file, err := os.OpenFile(diskPath, os.O_RDONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	// Leer el MBR
	mbr := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, mbr); err != nil {
		return nil, fmt.Errorf("error leyendo MBR: %v", err)
	}

	diskName := getFileNameFromPath(diskPath)
	var partitions []BasicPartitionInfo
	totalSize := mbr.MbrTamanio

	// Procesar particiones primarias y extendidas
	for _, p := range mbr.MbrPartitions {
		if p.Size > 0 {
			partName := strings.TrimRight(string(p.Name[:]), " \x00")

			// Determinar tipo de partición en texto
			typeName := ""
			switch p.Type {
			case PARTITION_PRIMARY:
				typeName = "Primaria"
			case PARTITION_EXTENDED:
				typeName = "Extendida"
			}

			// Verificar si está montada consultando el registro existente
			mountID, isMounted := getPartitionMountID(diskPath, partName)

			partInfo := BasicPartitionInfo{
				DiskPath:   diskPath,
				DiskName:   diskName,
				Name:       partName,
				Type:       p.Type,
				TypeName:   typeName,
				Size:       int64(p.Size),
				Start:      int64(p.Start),
				Status:     p.Status,
				Fit:        p.Fit,
				IsMounted:  isMounted,
				MountID:    mountID,
				Percentage: float64(p.Size) / float64(totalSize) * 100,
			}

			partitions = append(partitions, partInfo)

			// Si es extendida, procesar particiones lógicas
			if p.Type == PARTITION_EXTENDED {
				logicals, err := getLogicalPartitionsInfo(file, p.Start, diskPath, diskName, totalSize)
				if err != nil {
					fmt.Printf("Error leyendo particiones lógicas: %v\n", err)
				} else {
					partitions = append(partitions, logicals...)
				}
			}
		}
	}

	return partitions, nil
}

// getLogicalPartitionsInfo obtiene información básica de las particiones lógicas
func getLogicalPartitionsInfo(file *os.File, startPosition int64, diskPath, diskName string, totalSize int64) ([]BasicPartitionInfo, error) {
	var logicals []BasicPartitionInfo
	currentPos := startPosition

	for currentPos != -1 {
		if _, err := file.Seek(currentPos, 0); err != nil {
			return logicals, nil
		}

		ebr := &EBR{}
		if err := binary.Read(file, binary.LittleEndian, ebr); err != nil {
			return logicals, nil
		}

		if ebr.Size > 0 {
			partName := strings.TrimRight(string(ebr.Name[:]), " \x00")

			// Verificar si está montada
			mountID, isMounted := getPartitionMountID(diskPath, partName)

			partInfo := BasicPartitionInfo{
				DiskPath:   diskPath,
				DiskName:   diskName,
				Name:       partName,
				Type:       PARTITION_LOGIC,
				TypeName:   "Lógica",
				Size:       int64(ebr.Size),
				Start:      int64(ebr.Start),
				Status:     ebr.Status,
				Fit:        ebr.Fit,
				IsMounted:  isMounted,
				MountID:    mountID,
				Percentage: float64(ebr.Size) / float64(totalSize) * 100,
			}

			logicals = append(logicals, partInfo)
		}

		currentPos = ebr.Next
	}

	return logicals, nil
}

// getPartitionMountID verifica si una partición está montada usando tu implementación existente
func getPartitionMountID(diskPath string, partitionName string) (string, bool) {
	mountedPartitions := GetMountedPartitions()

	for _, mp := range mountedPartitions {
		if mp.DiskPath == diskPath && mp.PartitionName == partitionName {
			return mp.ID, true
		}
	}

	return "", false
}

// GetPartitionInfoByName busca información básica de una partición por nombre
func GetPartitionInfoByName(diskPath, partitionName string) (*BasicPartitionInfo, error) {
	partitions, err := GetDiskPartitionsInfo(diskPath)
	if err != nil {
		return nil, fmt.Errorf("error obteniendo particiones: %v", err)
	}

	for _, part := range partitions {
		if part.Name == partitionName {
			return &part, nil
		}
	}

	return nil, fmt.Errorf("partición '%s' no encontrada en el disco %s", partitionName, diskPath)
}
