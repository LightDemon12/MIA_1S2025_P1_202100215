package DiskManager

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// EXT2UpdateFileOwnerAndPermissions actualiza el propietario, grupo y permisos de un archivo o directorio
func EXT2UpdateFileOwnerAndPermissions(partitionID, path, owner, group string, perms []byte) error {
	// 1. Encontrar el inodo del archivo/directorio
	mountedPartition, err := FindMountedPartitionById(partitionID)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("error al abrir el disco: %s", err)
	}
	defer file.Close()

	startByte, _, err := GetPartitionDetails(file, mountedPartition)
	if err != nil {
		return fmt.Errorf("error al obtener detalles de la partición: %s", err)
	}

	superblock := &SuperBlock{}
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para leer superbloque: %s", err)
	}

	superblock, err = ReadSuperBlockFromDisc(file)
	if err != nil {
		return fmt.Errorf("error al leer el superbloque: %s", err)
	}

	// Encontrar el inodo por ruta
	inodeNum, inode, err := FindInodeByPath(file, startByte, superblock, path)
	if err != nil {
		return fmt.Errorf("no se encontró el archivo/directorio: %s", err)
	}

	// Buscar IDs para el usuario y grupo
	ownerId := getUserIdFromName(partitionID, owner)
	groupId := getGroupIdFromName(partitionID, group)

	// Actualizar el inodo
	inode.IUid = ownerId
	inode.IGid = groupId

	// Actualizar permisos si se proporcionaron
	if perms != nil && len(perms) >= 3 {
		inode.IPerm[0] = perms[0]
		inode.IPerm[1] = perms[1]
		inode.IPerm[2] = perms[2]
	}

	// Escribir el inodo actualizado
	inodePos := startByte + int64(superblock.SInodeStart) + int64(inodeNum)*int64(superblock.SInodeSize)
	_, err = file.Seek(inodePos, 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para escribir inodo: %s", err)
	}

	err = writeInodeToDisc(file, inode)
	if err != nil {
		return fmt.Errorf("error al escribir inodo: %s", err)
	}

	return nil
}

// FileExists verifica si un archivo o directorio existe
func FileExists(id string, path string) (bool, error) {
	// 1. Verificar la partición montada
	mountedPartition, err := FindMountedPartitionById(id)
	if err != nil {
		return false, err
	}

	// 2. Abrir el disco
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return false, err
	}
	defer file.Close()

	// 3. Obtener la posición de inicio de la partición
	startByte, _, err := GetPartitionDetails(file, mountedPartition)
	if err != nil {
		return false, err
	}

	// 4. Leer el superbloque
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, err
	}

	superblock, err := ReadSuperBlockFromDisc(file)
	if err != nil {
		return false, err
	}

	// 5. Intentar encontrar el inodo
	_, _, err = FindInodeByPath(file, startByte, superblock, path)
	if err != nil {
		return false, nil // No existe, pero no es un error
	}

	return true, nil
}

// Obtener ID de usuario a partir del nombre
func getUserIdFromName(partitionID, username string) int32 {
	// Leer el archivo users.txt para encontrar el ID del usuario
	content, err := EXT2FileOperation(partitionID, "/users.txt", FILE_READ, "")
	if err != nil {
		// Si hay error, asumimos usuario desconocido (0)
		return 0
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		parts := strings.Split(trimmedLine, ",")
		if len(parts) >= 5 && strings.TrimSpace(parts[1]) == "U" {
			if strings.TrimSpace(parts[2]) == username {
				id, err := strconv.Atoi(strings.TrimSpace(parts[0]))
				if err == nil && id > 0 {
					return int32(id)
				}
			}
		}
	}

	return 0 // ID desconocido
}

// Obtener ID de grupo a partir del nombre
func getGroupIdFromName(partitionID, groupname string) int32 {
	// Leer el archivo users.txt para encontrar el ID del grupo
	content, err := EXT2FileOperation(partitionID, "/users.txt", FILE_READ, "")
	if err != nil {
		// Si hay error, asumimos grupo desconocido (0)
		return 0
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		parts := strings.Split(trimmedLine, ",")
		if len(parts) >= 3 && strings.TrimSpace(parts[1]) == "G" {
			if strings.TrimSpace(parts[2]) == groupname {
				id, err := strconv.Atoi(strings.TrimSpace(parts[0]))
				if err == nil && id > 0 {
					return int32(id)
				}
			}
		}
	}

	return 0 // ID desconocido
}
