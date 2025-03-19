package DiskManager

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ValidateEXT2Path(id string, path string) (bool, string, error) {
	// 1. Verificar la partición montada
	mountedPartition, err := findMountedPartitionById(id)
	if err != nil {
		return false, "", fmt.Errorf("Error: %s", err)
	}

	// 2. Abrir el disco
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return false, "", fmt.Errorf("Error al abrir el disco: %s", err)
	}
	defer file.Close()

	// 3. Obtener detalles de la partición
	startByte, _, err := getPartitionDetails(file, mountedPartition)
	if err != nil {
		return false, "", fmt.Errorf("Error al obtener detalles de la partición: %s", err)
	}

	// 4. Leer el superbloque
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, "", fmt.Errorf("Error al posicionarse para leer superbloque: %s", err)
	}

	superblock, err := readSuperBlockFromDisc(file)
	if err != nil {
		return false, "", fmt.Errorf("Error al leer el superbloque: %s", err)
	}

	// 5. Normalizar la ruta
	path = filepath.Clean(path)
	if path == "." {
		path = "/"
	}

	// 6. Dividir la ruta en componentes
	components := strings.Split(path, "/")
	// Eliminar componentes vacíos
	var cleanedComponents []string
	for _, comp := range components {
		if comp != "" {
			cleanedComponents = append(cleanedComponents, comp)
		}
	}

	// 7. Comenzar desde el directorio raíz (inodo 2)
	currentInodeNum := int32(2)
	currentPath := "/"

	// Para depuración
	fmt.Printf("Validando ruta: %s con %d componentes\n", path, len(cleanedComponents))

	// 8. Recorrer cada componente de la ruta
	for i, component := range cleanedComponents {
		fmt.Printf("Verificando componente: '%s'\n", component)

		// Leer el inodo actual
		inodePos := startByte + int64(superblock.SInodeStart) + int64(currentInodeNum)*int64(superblock.SInodeSize)
		_, err = file.Seek(inodePos, 0)
		if err != nil {
			return false, "", fmt.Errorf("Error al posicionarse para leer inodo %d: %s", currentInodeNum, err)
		}

		currentInode, err := readInodeFromDisc(file)
		if err != nil {
			return false, "", fmt.Errorf("Error al leer inodo %d: %s", currentInodeNum, err)
		}

		// Verificar que el inodo actual sea un directorio
		if currentInode.IType != INODE_FOLDER {
			return false, "", fmt.Errorf("Error: %s no es un directorio", currentPath)
		}

		// Buscar la entrada del componente en el directorio actual
		found := false
		var nextInodeNum int32 = -1

		// Examinar los bloques directos del directorio
		for blockIdx := 0; blockIdx < 12; blockIdx++ {
			if currentInode.IBlock[blockIdx] <= 0 {
				continue // Bloque no asignado
			}

			// Leer el bloque del directorio
			blockPos := startByte + int64(superblock.SBlockStart) +
				int64(currentInode.IBlock[blockIdx])*int64(superblock.SBlockSize)

			_, err = file.Seek(blockPos, 0)
			if err != nil {
				continue
			}

			// Leer el bloque de directorio
			dirBlock := &DirectoryBlock{}
			err = binary.Read(file, binary.LittleEndian, dirBlock)
			if err != nil {
				fmt.Printf("Error al leer bloque de directorio: %s\n", err)
				continue
			}

			// Buscar la entrada en este bloque
			for entryIdx := 0; entryIdx < B_CONTENT_COUNT; entryIdx++ {
				entry := &dirBlock.BContent[entryIdx]

				if entry.BInodo <= 0 {
					continue // Entrada vacía
				}

				entryName := strings.TrimRight(string(entry.BName[:]), "\x00")
				fmt.Printf("Entrada encontrada: '%s' -> inodo %d\n", entryName, entry.BInodo)

				if entryName == component {
					found = true
					nextInodeNum = entry.BInodo
					break
				}
			}

			if found {
				break
			}
		}

		if !found {
			return false, "", fmt.Errorf("Error: No se encontró el componente '%s' en la ruta %s",
				component, currentPath)
		}

		// Actualizar para el siguiente componente
		currentInodeNum = nextInodeNum
		if currentPath == "/" {
			currentPath = "/" + component
		} else {
			currentPath = currentPath + "/" + component
		}

		// Si estamos en el último componente, determinar su tipo
		if i == len(cleanedComponents)-1 {
			// Leer el inodo final
			inodePos = startByte + int64(superblock.SInodeStart) + int64(currentInodeNum)*int64(superblock.SInodeSize)
			_, err = file.Seek(inodePos, 0)
			if err != nil {
				return false, "", fmt.Errorf("Error al leer inodo final: %s", err)
			}

			finalInode, err := readInodeFromDisc(file)
			if err != nil {
				return false, "", fmt.Errorf("Error al leer inodo final: %s", err)
			}

			pathType := "archivo"
			if finalInode.IType == INODE_FOLDER {
				pathType = "directorio"
			}

			return true, pathType, nil
		}
	}

	// Si llegamos aquí con una ruta vacía, estamos en el directorio raíz
	if len(cleanedComponents) == 0 {
		return true, "directorio", nil
	}

	return false, "", fmt.Errorf("Error inesperado al validar ruta")
}
