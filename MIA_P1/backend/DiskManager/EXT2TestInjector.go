package DiskManager

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// EXT2AutoInjector crea una estructura de directorios y archivos con contenido simple
func EXT2AutoInjector(id string) (bool, string) {
	fmt.Println("Iniciando inyección de archivos en partición:", id)

	// 1. Primero verificar cuánto espacio hay en el directorio raíz
	availableRootEntries := checkAvailableEntriesInRoot(id)
	fmt.Printf("Entradas disponibles en directorio raíz: %d\n", availableRootEntries)

	if availableRootEntries <= 0 {
		return false, "Error: El directorio raíz está completamente lleno, no se pueden crear más elementos"
	}

	// 2. Definir estructura a crear (en orden jerárquico)
	estructuraOrdenada := []struct {
		EsDirectorio bool
		Ruta         string
		Contenido    string
	}{
		// Primero los directorios raíz (ordenados por prioridad)
		{true, "/home", ""},
		{true, "/docs", ""},
		{true, "/etc", ""},
		{true, "/user", ""},

		// Luego archivos en la raíz
		{false, "/hola", "127.0.0.1 localhost\n192.168.1.1 router"},
		{false, "/notas", "127.0.0.1 localhost\n192.168.1.1 router"},

		// Directorios anidados
		{true, "/home/user1", ""},
		{true, "/home/user2", ""},

		// Archivos dentro de directorios
		{false, "/docs/leeme", "127.0.0.1 localhost\n192.168.1.1 router"},
		{false, "/home/user1/data", "127.0.0.1 localhost\n192.168.1.1 router"}, // Cambiado de "personal.txt" a "data.txt"
		{false, "/home/user2/config", "127.0.0.1 localhost\n192.168.1.1 router"},
		{false, "/etc/hosts", "127.0.0.1 localhost\n192.168.1.1 router"},
	}

	// 3. Crear elementos hasta que no haya más espacio
	createdItems := []string{}
	errItems := []string{}

	// Track de directorios raíz creados
	rootElementsCreated := 0

	for _, item := range estructuraOrdenada {
		// Si es un elemento en la raíz, verificar límite
		if strings.Count(item.Ruta, "/") == 1 && rootElementsCreated >= availableRootEntries {
			reason := fmt.Sprintf("No hay más espacio en el directorio raíz (límite: %d)", availableRootEntries)
			errItems = append(errItems, fmt.Sprintf("%s: %s", item.Ruta, reason))
			continue
		}

		var success bool
		var message string

		if item.EsDirectorio {
			fmt.Printf("Creando directorio: %s\n", item.Ruta)
			success, message = CreateEXT2Directory(id, item.Ruta)
		} else {
			fmt.Printf("Creando archivo: %s\n", item.Ruta)
			success, message = CreateEXT2File(id, item.Ruta, item.Contenido)
		}

		if success {
			if strings.Count(item.Ruta, "/") == 1 {
				rootElementsCreated++
			}

			if item.EsDirectorio {
				createdItems = append(createdItems, fmt.Sprintf("Directorio '%s': creado exitosamente", item.Ruta))
			} else {
				createdItems = append(createdItems, fmt.Sprintf("Archivo '%s': creado exitosamente, contenido: %d bytes",
					item.Ruta, len(item.Contenido)))
			}
		} else {
			errItems = append(errItems, fmt.Sprintf("%s: %s", item.Ruta, message))
		}
	}

	// Mensaje final
	var message strings.Builder
	message.WriteString(fmt.Sprintf("=== INYECCIÓN COMPLETADA: %d ELEMENTOS CREADOS, %d ERRORES ===\n\n",
		len(createdItems), len(errItems)))

	if len(createdItems) > 0 {
		message.WriteString("ELEMENTOS CREADOS:\n")
		for _, item := range createdItems {
			message.WriteString("• " + item + "\n")
		}
		message.WriteString("\n")
	}

	if len(errItems) > 0 {
		message.WriteString("ERRORES:\n")
		for _, item := range errItems {
			message.WriteString("• " + item + "\n")
		}
	}

	message.WriteString("\nPara visualizar la estructura:\n")
	message.WriteString("rep -id=" + id + " -path=/home/reportes/inodos.jpg -name=inode\n")
	message.WriteString("rep -id=" + id + " -path=/home/reportes/tree.jpg -name=tree\n")

	return len(createdItems) > 0, message.String()
}

// checkAvailableEntriesInRoot verifica cuántas entradas libres hay en el directorio raíz
func checkAvailableEntriesInRoot(id string) int {
	// 1. Localizar partición montada
	mountedPartition, err := findMountedPartitionById(id)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return 0
	}

	// 2. Abrir el disco
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("Error al abrir el disco: %s\n", err)
		return 0
	}
	defer file.Close()

	// 3. Obtener posición de inicio y leer el superbloque
	startByte, _, err := getPartitionDetails(file, mountedPartition)
	if err != nil {
		fmt.Printf("Error al obtener detalles de la partición: %s\n", err)
		return 0
	}

	_, err = file.Seek(startByte, 0)
	if err != nil {
		fmt.Printf("Error al posicionarse para leer el superbloque: %s\n", err)
		return 0
	}

	superblock, err := readSuperBlockFromDisc(file)
	if err != nil {
		fmt.Printf("Error al leer el superbloque: %s\n", err)
		return 0
	}

	// 4. Leer inodo raíz (inodo 2)
	rootInodePos := startByte + int64(superblock.SInodeStart) + 2*int64(superblock.SInodeSize)
	_, err = file.Seek(rootInodePos, 0)
	if err != nil {
		fmt.Printf("Error al posicionarse en inodo raíz: %s\n", err)
		return 0
	}

	rootInode, err := readInodeFromDisc(file)
	if err != nil {
		fmt.Printf("Error al leer inodo raíz: %s\n", err)
		return 0
	}

	// 5. Leer bloque del directorio raíz
	if rootInode.IBlock[0] <= 0 {
		fmt.Println("Error: El inodo raíz no tiene bloques asignados")
		return 0
	}

	rootBlockPos := startByte + int64(superblock.SBlockStart) + int64(rootInode.IBlock[0])*int64(superblock.SBlockSize)
	_, err = file.Seek(rootBlockPos, 0)
	if err != nil {
		fmt.Printf("Error al posicionarse en bloque raíz: %s\n", err)
		return 0
	}

	rootDirBlock := &DirectoryBlock{}
	err = binary.Read(file, binary.LittleEndian, rootDirBlock)
	if err != nil {
		fmt.Printf("Error al leer bloque raíz: %s\n", err)
		return 0
	}

	// 6. Contar entradas disponibles
	availableEntries := 0
	usedEntries := 0

	fmt.Println("Revisando entradas del directorio raíz:")
	for i := 0; i < B_CONTENT_COUNT; i++ {
		name := strings.TrimRight(string(rootDirBlock.BContent[i].BName[:]), "\x00")
		if rootDirBlock.BContent[i].BInodo <= 0 {
			availableEntries++
			fmt.Printf("[%d] <libre>\n", i)
		} else {
			usedEntries++
			fmt.Printf("[%d] '%s' (inodo %d)\n", i, name, rootDirBlock.BContent[i].BInodo)
		}
	}

	fmt.Printf("Total: %d entradas usadas, %d entradas disponibles\n", usedEntries, availableEntries)
	return availableEntries
}
