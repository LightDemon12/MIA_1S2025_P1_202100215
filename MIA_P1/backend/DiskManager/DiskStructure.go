package DiskManager

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"strings"
)

// DiskAnalysis contiene toda la información analizada del disco
type DiskAnalysis struct {
	Path      string                `json:"path"`
	Name      string                `json:"name"`
	TotalSize int64                 `json:"totalSize"`
	Signature int32                 `json:"signature"`
	CreatedAt string                `json:"createdAt"`
	Fit       byte                  `json:"fit"`
	Sections  []DiskAnalysisSection `json:"sections"`
}

// DiskAnalysisSection representa una sección del disco (MBR, partición, espacio libre)
type DiskAnalysisSection struct {
	Start       int64                 `json:"start"`
	Size        int64                 `json:"size"`
	Percentage  float64               `json:"percentage"`
	SectionType string                `json:"sectionType"` // "MBR", "PRIMARY", "EXTENDED", "EBR", "LOGICAL", "FREE"
	Name        string                `json:"name,omitempty"`
	Fit         byte                  `json:"fit,omitempty"`
	Status      byte                  `json:"status,omitempty"`
	Index       int                   `json:"index,omitempty"`
	SubSections []DiskAnalysisSection `json:"subSections,omitempty"` // Para particiones lógicas dentro de extendidas
}

// AnalyzeAllDisks analiza todos los discos disponibles en la lista de registro
func AnalyzeAllDisks() ([]DiskAnalysis, error) {
	disks := GetAllDisks()
	results := make([]DiskAnalysis, 0, len(disks))

	for _, disk := range disks {
		analysis, err := AnalyzeDiskStructure(disk.Path)
		if err != nil {
			// Log el error pero continúa con el siguiente disco
			fmt.Printf("Error analizando disco %s: %v\n", disk.Path, err)
			continue
		}
		results = append(results, *analysis)
	}

	return results, nil
}

// AnalyzeDiskStructure analiza la estructura completa de un disco sin generar un reporte gráfico
func AnalyzeDiskStructure(diskPath string) (*DiskAnalysis, error) {
	// 1. Abrir el disco
	file, err := os.OpenFile(diskPath, os.O_RDONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	// 2. Leer el MBR
	mbr := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, mbr); err != nil {
		return nil, fmt.Errorf("error leyendo MBR: %v", err)
	}

	// Crear estructura para almacenar la información
	diskAnalysis := &DiskAnalysis{
		Path:      diskPath,
		Name:      getFileNameFromPath(diskPath),
		TotalSize: mbr.MbrTamanio,
		Signature: mbr.MbrDskSignature,
		CreatedAt: string(bytes.Trim(mbr.MbrFechaCreacion[:], "\x00")),
		Fit:       mbr.DskFit,
		Sections:  []DiskAnalysisSection{},
	}

	// 3. Recolectar información de las secciones del disco
	var sections []DiskAnalysisSection
	mbrSize := int64(binary.Size(mbr))
	totalSize := mbr.MbrTamanio

	// Agregar MBR
	sections = append(sections, DiskAnalysisSection{
		Start:       0,
		Size:        mbrSize,
		Percentage:  float64(mbrSize) / float64(totalSize) * 100,
		SectionType: "MBR",
	})

	// Recolectar particiones
	var partitions []DiskAnalysisSection
	for i, p := range mbr.MbrPartitions {
		if p.Size > 0 {
			partName := strings.TrimRight(string(p.Name[:]), " \x00")
			sectionType := ""
			switch p.Type {
			case PARTITION_PRIMARY:
				sectionType = "PRIMARY"
			case PARTITION_EXTENDED:
				sectionType = "EXTENDED"

				// Procesar particiones lógicas para la extendida
				var logicals []DiskAnalysisSection
				currentPos := p.Start

				for currentPos != -1 {
					if _, err := file.Seek(int64(currentPos), 0); err != nil {
						break
					}

					ebr := &EBR{}
					if err := binary.Read(file, binary.LittleEndian, ebr); err != nil {
						break
					}

					ebrSize := int64(binary.Size(ebr))

					// Agregar EBR
					logicals = append(logicals, DiskAnalysisSection{
						Start:       int64(currentPos),
						Size:        ebrSize,
						Percentage:  float64(ebrSize) / float64(totalSize) * 100,
						SectionType: "EBR",
						Index:       len(logicals) + 1,
					})

					// Agregar partición lógica si tiene tamaño
					if ebr.Size > 0 {
						logicName := strings.TrimRight(string(ebr.Name[:]), " \x00")
						logicals = append(logicals, DiskAnalysisSection{
							Start:       int64(currentPos) + ebrSize,
							Size:        int64(ebr.Size),
							Percentage:  float64(ebr.Size) / float64(totalSize) * 100,
							SectionType: "LOGICAL",
							Name:        logicName,
							Fit:         ebr.Fit,
							Status:      ebr.Status,
							Index:       len(logicals) + 1,
						})
					}

					if ebr.Next <= 0 {
						break
					}
					currentPos = ebr.Next
				}

				// Agregar subsecciones a la partición extendida
				partitions = append(partitions, DiskAnalysisSection{
					Start:       int64(p.Start),
					Size:        int64(p.Size),
					Percentage:  float64(p.Size) / float64(totalSize) * 100,
					SectionType: sectionType,
					Name:        partName,
					Fit:         p.Fit,
					Status:      p.Status,
					Index:       i + 1,
					SubSections: logicals,
				})
				continue
			}

			partitions = append(partitions, DiskAnalysisSection{
				Start:       int64(p.Start),
				Size:        int64(p.Size),
				Percentage:  float64(p.Size) / float64(totalSize) * 100,
				SectionType: sectionType,
				Name:        partName,
				Fit:         p.Fit,
				Status:      p.Status,
				Index:       i + 1,
			})
		}
	}

	// Ordenar particiones por posición
	sort.Slice(partitions, func(i, j int) bool {
		return partitions[i].Start < partitions[j].Start
	})

	// Agregar particiones a las secciones
	sections = append(sections, partitions...)

	// Ordenar todas las secciones por posición
	sort.Slice(sections, func(i, j int) bool {
		return sections[i].Start < sections[j].Start
	})

	// 4. Agregar espacios libres
	var allSections []DiskAnalysisSection
	lastEnd := int64(0)

	for _, section := range sections {
		// Si hay un espacio antes de esta sección
		if section.Start > lastEnd {
			freeSize := section.Start - lastEnd
			allSections = append(allSections, DiskAnalysisSection{
				Start:       lastEnd,
				Size:        freeSize,
				Percentage:  float64(freeSize) / float64(totalSize) * 100,
				SectionType: "FREE",
			})
		}

		// Agregar la sección actual
		allSections = append(allSections, section)

		// Actualizar última posición
		sectionEnd := section.Start + section.Size
		if sectionEnd > lastEnd {
			lastEnd = sectionEnd
		}
	}

	// Verificar espacio libre al final
	if lastEnd < totalSize {
		freeSize := totalSize - lastEnd
		allSections = append(allSections, DiskAnalysisSection{
			Start:       lastEnd,
			Size:        freeSize,
			Percentage:  float64(freeSize) / float64(totalSize) * 100,
			SectionType: "FREE",
		})
	}

	// Ordenar nuevamente
	sort.Slice(allSections, func(i, j int) bool {
		return allSections[i].Start < allSections[j].Start
	})

	// Guardar las secciones en la estructura del disco
	diskAnalysis.Sections = allSections

	return diskAnalysis, nil
}

// Función auxiliar para obtener el nombre de archivo de una ruta
func getFileNameFromPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}
	fileName := parts[len(parts)-1]
	return strings.TrimSuffix(fileName, ".mia")
}
