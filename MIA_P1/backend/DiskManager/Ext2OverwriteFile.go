package DiskManager

import (
	"fmt"
	"os"
	"time"
)

// OverwriteEXT2File sobrescribe el contenido de un archivo existente en el sistema de archivos EXT2
// Implementación segura para evitar corrupción de otros archivos
// OverwriteEXT2File sobrescribe el contenido de un archivo existente en el sistema de archivos EXT2
// Implementación segura para evitar corrupción del disco
func OverwriteEXT2File(id, path, newContent string) error {
	fmt.Printf("OverwriteEXT2File: Sobrescribiendo archivo '%s'\n", path)

	// PROTECCIÓN CRÍTICA INICIAL
	if path == "/users.txt" || path == "users.txt" {
		return fmt.Errorf("no se permite sobrescribir directamente users.txt por seguridad")
	}

	// 1. Verificar la partición montada
	mountedPartition, err := FindMountedPartitionById(id)
	if err != nil {
		return fmt.Errorf("partición no encontrada: %v", err)
	}

	// 2. Abrir el disco en modo exclusivo para evitar interferencias
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("error al abrir disco: %v", err)
	}
	// Asegurarnos de que el archivo se cierre al finalizar, pase lo que pase
	defer file.Close()

	// 3. Obtener la posición de inicio de la partición
	startByte, _, err := GetPartitionDetails(file, mountedPartition)
	if err != nil {
		return fmt.Errorf("error obteniendo detalles de partición: %v", err)
	}

	// 4. Leer el superbloque
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para leer superbloque: %v", err)
	}

	superblock, err := ReadSuperBlockFromDisc(file)
	if err != nil {
		return fmt.Errorf("error al leer superbloque: %v", err)
	}

	// 5. PROTECCIÓN CRÍTICA: Hacer respaldo completo de archivos sensibles antes de cualquier operación
	backups := make(map[int][]byte) // Mapa de inodo -> contenido completo

	// Lista de inodos críticos a proteger
	criticalInodes := []int{2, 3} // Raíz y users.txt

	// Hacer respaldo de cada inodo crítico y sus bloques
	for _, inodeNum := range criticalInodes {
		// Leer el inodo
		inodePos := startByte + int64(superblock.SInodeStart) + int64(inodeNum)*int64(superblock.SInodeSize)
		_, err = file.Seek(inodePos, 0)
		if err != nil {
			continue // Si no podemos leer este inodo, simplemente continuamos
		}

		inode, err := readInodeFromDisc(file)
		if err != nil {
			continue
		}

		// Hacer una copia binaria exacta del inodo para restaurarlo después si es necesario
		inodeBackup := make([]byte, superblock.SInodeSize)
		_, err = file.Seek(inodePos, 0)
		if err == nil {
			_, err = file.Read(inodeBackup)
			if err == nil {
				// Guardar esta copia exacta
				backups[inodeNum] = inodeBackup

				// También respaldar todos los bloques de datos
				for i := 0; i < 12; i++ {
					if inode.IBlock[i] <= 0 {
						continue
					}

					blockPos := startByte + int64(superblock.SBlockStart) +
						int64(inode.IBlock[i])*int64(superblock.SBlockSize)

					blockBackup := make([]byte, superblock.SBlockSize)
					_, err = file.Seek(blockPos, 0)
					if err == nil {
						_, err = file.Read(blockBackup)
						if err == nil {
							// Guardar este bloque con un identificador único
							backupKey := -(inodeNum*1000 + i) // Clave negativa para diferenciar de inodos
							backups[backupKey] = blockBackup
						}
					}
				}
			}
		}
	}

	// Configurar la restauración al finalizar, pase lo que pase
	defer func() {
		// Verificar si necesitamos restaurar (comprobar users.txt)
		usersInodeNum := 3
		usersInodePos := startByte + int64(superblock.SInodeStart) + int64(usersInodeNum)*int64(superblock.SInodeSize)

		// Verificar el inodo de users.txt
		_, err := file.Seek(usersInodePos, 0)
		if err == nil {
			// Si tenemos un respaldo del inodo de users.txt
			if inodeBackup, exists := backups[usersInodeNum]; exists {
				// Restaurar exactamente como estaba originalmente
				_, err = file.Seek(usersInodePos, 0)
				if err == nil {
					_, err = file.Write(inodeBackup)
					if err != nil {
						fmt.Printf("ERROR: No se pudo restaurar inodo %d: %v\n", usersInodeNum, err)
					} else {
						fmt.Printf("INFO: Restaurado inodo %d (preventivo)\n", usersInodeNum)
					}
				}

				// También restaurar sus bloques
				for i := 0; i < 12; i++ {
					backupKey := -(usersInodeNum*1000 + i)
					if blockBackup, exists := backups[backupKey]; exists {
						// Leer el inodo para obtener el número de bloque
						_, err = file.Seek(usersInodePos, 0)
						if err == nil {
							inode, err := readInodeFromDisc(file)
							if err == nil && inode.IBlock[i] > 0 {
								blockPos := startByte + int64(superblock.SBlockStart) +
									int64(inode.IBlock[i])*int64(superblock.SBlockSize)

								_, err = file.Seek(blockPos, 0)
								if err == nil {
									_, err = file.Write(blockBackup)
									if err != nil {
										fmt.Printf("ERROR: No se pudo restaurar bloque %d del inodo %d: %v\n",
											i, usersInodeNum, err)
									} else {
										fmt.Printf("INFO: Restaurado bloque %d del inodo %d (preventivo)\n",
											i, usersInodeNum)
									}
								}
							}
						}
					}
				}
			}
		}
	}()

	// 6. Buscar el archivo a sobrescribir
	fileInodeNum, fileInode, err := FindInodeByPath(file, startByte, superblock, path)
	if err != nil {
		return fmt.Errorf("archivo no encontrado: %v", err)
	}

	// 7. Verificar que sea un archivo (no un directorio)
	if fileInode.IType != INODE_FILE {
		return fmt.Errorf("la ruta '%s' no es un archivo", path)
	}

	// 8. PROTECCIÓN CRÍTICA: Verificar que no sea un archivo crítico del sistema
	for _, criticalInode := range criticalInodes {
		if fileInodeNum == criticalInode {
			return fmt.Errorf("no se permite sobrescribir archivos críticos del sistema (inodo %d)", criticalInode)
		}
	}

	// 9. Obtener los bloques que usa actualmente el archivo
	currentBlocks := make([]int32, 0)
	for i := 0; i < 12; i++ { // Solo bloques directos por ahora
		if fileInode.IBlock[i] > 0 {
			currentBlocks = append(currentBlocks, fileInode.IBlock[i])
		}
	}

	// 10. Identificar bloques críticos para evitarlos
	criticalBlocks := identifyCriticalBlocks(file, startByte, superblock)

	// 11. Preparar el nuevo contenido
	contentBytes := []byte(newContent)
	contentSize := len(contentBytes)
	blockSize := int(superblock.SBlockSize)

	// 12. Determinar cuántos bloques se necesitan
	requiredBlocks := (contentSize + blockSize - 1) / blockSize
	if requiredBlocks == 0 {
		requiredBlocks = 1 // Al menos un bloque siempre
	}

	// 13. Verificar si los bloques actuales son suficientes
	additionalBlocksNeeded := requiredBlocks - len(currentBlocks)
	if additionalBlocksNeeded > 0 {
		// Necesitamos cargar el bitmap de bloques
		blockBitmap, err := loadBlockBitmap(file, startByte, superblock)
		if err != nil {
			return fmt.Errorf("error cargando bitmap de bloques: %v", err)
		}

		// Buscar bloques adicionales
		newBlocks := make([]int, 0, additionalBlocksNeeded)
		for i := 0; i < additionalBlocksNeeded; i++ {
			freeBlockNum := findSafeBlockNum(blockBitmap, int(superblock.SBlocksCount), criticalBlocks)
			if freeBlockNum < 0 {
				return fmt.Errorf("no hay suficientes bloques libres disponibles")
			}

			// Marcar el bloque como usado en el bitmap
			blockBitmap[freeBlockNum/8] |= (1 << (freeBlockNum % 8))
			newBlocks = append(newBlocks, freeBlockNum)
		}

		// Actualizar el bitmap de bloques
		_, err = file.Seek(startByte+int64(superblock.SBmBlockStart), 0)
		if err != nil {
			return fmt.Errorf("error al posicionarse para actualizar bitmap de bloques: %v", err)
		}

		_, err = file.Write(blockBitmap)
		if err != nil {
			return fmt.Errorf("error al actualizar bitmap de bloques: %v", err)
		}

		// Añadir los nuevos bloques al inodo
		for _, blockNum := range newBlocks {
			// Usar el siguiente bloque disponible en el inodo
			for j := 0; j < 12; j++ {
				if fileInode.IBlock[j] <= 0 {
					fileInode.IBlock[j] = int32(blockNum)
					break
				}
			}
		}

		// Actualizar el contador de bloques libres en el superbloque
		superblock.SFreeBlocksCount -= int32(additionalBlocksNeeded)
		_, err = file.Seek(startByte, 0)
		if err != nil {
			return fmt.Errorf("error al posicionarse para actualizar superbloque: %v", err)
		}

		err = writeSuperBlockToDisc(file, superblock)
		if err != nil {
			return fmt.Errorf("error al actualizar superbloque: %v", err)
		}
	} else if additionalBlocksNeeded < 0 {
		// Tenemos más bloques de los necesarios, marcar los sobrantes como libres
		// Cargar el bitmap de bloques
		blockBitmap, err := loadBlockBitmap(file, startByte, superblock)
		if err != nil {
			return fmt.Errorf("error cargando bitmap de bloques: %v", err)
		}

		// Liberar los bloques sobrantes (desde el final)
		for i := requiredBlocks; i < len(currentBlocks); i++ {
			blockNum := currentBlocks[i]

			// Marcar el bloque como libre en el bitmap
			blockBitmap[blockNum/8] &= ^(1 << (blockNum % 8))

			// Quitar la referencia en el inodo
			for j := 0; j < 12; j++ {
				if fileInode.IBlock[j] == blockNum {
					fileInode.IBlock[j] = -1
					break
				}
			}
		}

		// Actualizar el bitmap de bloques
		_, err = file.Seek(startByte+int64(superblock.SBmBlockStart), 0)
		if err != nil {
			return fmt.Errorf("error al posicionarse para actualizar bitmap de bloques: %v", err)
		}

		_, err = file.Write(blockBitmap)
		if err != nil {
			return fmt.Errorf("error al actualizar bitmap de bloques: %v", err)
		}

		// Actualizar el contador de bloques libres en el superbloque
		superblock.SFreeBlocksCount += int32(-additionalBlocksNeeded)
		_, err = file.Seek(startByte, 0)
		if err != nil {
			return fmt.Errorf("error al posicionarse para actualizar superbloque: %v", err)
		}

		err = writeSuperBlockToDisc(file, superblock)
		if err != nil {
			return fmt.Errorf("error al actualizar superbloque: %v", err)
		}
	}

	// 14. Escribir el contenido a los bloques
	remainingContent := contentBytes

	for i := 0; i < 12 && i < requiredBlocks; i++ {
		if fileInode.IBlock[i] <= 0 {
			continue
		}

		// Verificar que este bloque no sea crítico
		if criticalBlocks[fileInode.IBlock[i]] {
			return fmt.Errorf("error crítico: el bloque %d está marcado como crítico", fileInode.IBlock[i])
		}

		// Preparar el buffer para este bloque
		blockBuffer := make([]byte, blockSize)
		bytesToWrite := len(remainingContent)
		if bytesToWrite > blockSize {
			bytesToWrite = blockSize
		}

		// Copiar solo lo que quepa en este bloque
		copy(blockBuffer[:bytesToWrite], remainingContent[:bytesToWrite])

		// Rellenar el resto del bloque con un patrón seguro
		for j := bytesToWrite; j < blockSize; j++ {
			blockBuffer[j] = byte('-')
		}

		// Escribir el bloque
		blockPos := startByte + int64(superblock.SBlockStart) + int64(fileInode.IBlock[i])*int64(blockSize)
		_, err = file.Seek(blockPos, 0)
		if err != nil {
			return fmt.Errorf("error al posicionarse para escribir bloque %d: %v", i, err)
		}

		_, err = file.Write(blockBuffer)
		if err != nil {
			return fmt.Errorf("error al escribir bloque %d: %v", i, err)
		}

		// Actualizar el contenido restante
		if len(remainingContent) > blockSize {
			remainingContent = remainingContent[blockSize:]
		} else {
			remainingContent = nil
		}
	}

	// 15. Actualizar los metadatos del inodo
	fileInode.ISize = int32(contentSize)
	fileInode.IMtime = time.Now()

	// 16. Escribir el inodo actualizado
	inodePos := startByte + int64(superblock.SInodeStart) + int64(fileInodeNum)*int64(superblock.SInodeSize)
	_, err = file.Seek(inodePos, 0)
	if err != nil {
		return fmt.Errorf("error al posicionarse para actualizar inodo: %v", err)
	}

	err = writeInodeToDisc(file, fileInode)
	if err != nil {
		return fmt.Errorf("error al actualizar inodo: %v", err)
	}

	// 17. Sincrónizar cambios al disco para garantizar persistencia
	err = file.Sync()
	if err != nil {
		return fmt.Errorf("error al sincronizar cambios con el disco: %v", err)
	}

	fmt.Printf("Archivo '%s' sobrescrito exitosamente (tamaño: %d bytes)\n", path, contentSize)
	return nil
}
