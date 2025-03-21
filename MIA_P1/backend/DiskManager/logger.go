package DiskManager

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"time"
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

// LogEXT2 muestra la información de una partición formateada con EXT2
func LogEXT2(id string) {
	// 1. Encontrar la partición montada
	mountedPartition, err := FindMountedPartitionById(id)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	// 2. Abrir el disco
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDONLY, 0666)
	if err != nil {
		fmt.Printf("Error al abrir disco: %s\n", err)
		return
	}
	defer file.Close()

	// 3. Obtener detalles de la partición
	startByte, size, err := GetPartitionDetails(file, mountedPartition)
	if err != nil {
		fmt.Printf("Error al obtener detalles de la partición: %s\n", err)
		return
	}

	fmt.Printf("=== INFORMACIÓN EXT2 DE PARTICIÓN %s ===\n", id)
	fmt.Printf("Nombre: %s\n", mountedPartition.PartitionName)
	fmt.Printf("Tipo: %c\n", mountedPartition.PartitionType)
	fmt.Printf("Inicio: %d bytes\n", startByte)
	fmt.Printf("Tamaño: %d bytes\n\n", size)

	// 4. Leer el SuperBloque
	_, err = file.Seek(startByte, 0)
	if err != nil {
		fmt.Printf("Error al posicionarse para leer SuperBloque: %s\n", err)
		return
	}

	sb, err := ReadSuperBlockFromDisc(file)
	if err != nil {
		fmt.Printf("Error al leer SuperBloque: %s\n", err)
		return
	}

	// 5. Mostrar información del SuperBloque
	fmt.Println("=== SUPERBLOQUE ===")
	fmt.Printf("Tipo de sistema: %d (EXT2)\n", sb.SFilesystemType)
	fmt.Printf("Número mágico: 0x%X\n", sb.SMagic)
	fmt.Printf("Total inodos: %d\n", sb.SInodesCount)
	fmt.Printf("Total bloques: %d\n", sb.SBlocksCount)
	fmt.Printf("Inodos libres: %d\n", sb.SFreeInodesCount)
	fmt.Printf("Bloques libres: %d\n", sb.SFreeBlocksCount)
	fmt.Printf("Uso inodos: %.2f%%\n", float64(sb.SInodesCount-sb.SFreeInodesCount)/float64(sb.SInodesCount)*100)
	fmt.Printf("Uso bloques: %.2f%%\n", float64(sb.SBlocksCount-sb.SFreeBlocksCount)/float64(sb.SBlocksCount)*100)
	fmt.Printf("Tamaño inodo: %d bytes\n", sb.SInodeSize)
	fmt.Printf("Tamaño bloque: %d bytes\n", sb.SBlockSize)
	fmt.Printf("Último montaje: %s\n", sb.SMtime.Format(time.RFC1123))
	if !sb.SUmtime.IsZero() {
		fmt.Printf("Último desmontaje: %s\n", sb.SUmtime.Format(time.RFC1123))
	} else {
		fmt.Printf("Último desmontaje: Nunca\n")
	}
	fmt.Printf("Contador montajes: %d\n", sb.SMntCount)

	// Posiciones clave
	fmt.Printf("Inicio bitmap inodos: %d\n", sb.SBmInodeStart)
	fmt.Printf("Inicio bitmap bloques: %d\n", sb.SBmBlockStart)
	fmt.Printf("Inicio tabla inodos: %d\n", sb.SInodeStart)
	fmt.Printf("Inicio bloques datos: %d\n\n", sb.SBlockStart)

	// 6. Leer y mostrar bitmap de inodos (primeros bytes)
	_, err = file.Seek(startByte+int64(sb.SBmInodeStart), 0)
	if err != nil {
		fmt.Printf("Error al posicionarse para leer bitmap de inodos: %s\n", err)
		return
	}

	// Leer algunos bytes del bitmap de inodos para visualización
	inodeBitmapSample := make([]byte, 8) // Primeros 8 bytes = 64 inodos
	_, err = file.Read(inodeBitmapSample)
	if err != nil {
		fmt.Printf("Error al leer bitmap de inodos: %s\n", err)
	} else {
		fmt.Println("=== BITMAP DE INODOS (MUESTRA) ===")
		fmt.Printf("Hex: ")
		for _, b := range inodeBitmapSample {
			fmt.Printf("%02X ", b)
		}
		fmt.Printf("\nBin: ")
		for _, b := range inodeBitmapSample {
			fmt.Printf("%08b ", b)
		}
		fmt.Println("\n")
	}

	// 7. Leer y mostrar bitmap de bloques (primeros bytes)
	_, err = file.Seek(startByte+int64(sb.SBmBlockStart), 0)
	if err != nil {
		fmt.Printf("Error al posicionarse para leer bitmap de bloques: %s\n", err)
		return
	}

	// Leer algunos bytes del bitmap de bloques para visualización
	blockBitmapSample := make([]byte, 8) // Primeros 8 bytes = 64 bloques
	_, err = file.Read(blockBitmapSample)
	if err != nil {
		fmt.Printf("Error al leer bitmap de bloques: %s\n", err)
	} else {
		fmt.Println("=== BITMAP DE BLOQUES (MUESTRA) ===")
		fmt.Printf("Hex: ")
		for _, b := range blockBitmapSample {
			fmt.Printf("%02X ", b)
		}
		fmt.Printf("\nBin: ")
		for _, b := range blockBitmapSample {
			fmt.Printf("%08b ", b)
		}
		fmt.Println("\n")
	}

	// 8. Leer y mostrar el inodo raíz (inodo 2)
	inodePos := startByte + int64(sb.SInodeStart) + int64(2)*int64(sb.SInodeSize)
	_, err = file.Seek(inodePos, 0)
	if err != nil {
		fmt.Printf("Error al posicionarse para leer inodo raíz: %s\n", err)
		return
	}

	rootInode, err := readInodeFromDisc(file)
	if err != nil {
		fmt.Printf("Error al leer inodo raíz: %s\n", err)
		return
	}

	fmt.Println("=== INODO RAÍZ (INODO 2) ===")
	fmt.Printf("Tipo: %d (0=Carpeta, 1=Archivo)\n", rootInode.IType)
	fmt.Printf("Tamaño: %d bytes\n", rootInode.ISize)
	fmt.Printf("UID: %d\n", rootInode.IUid)
	fmt.Printf("GID: %d\n", rootInode.IGid)
	fmt.Printf("Permisos: %v\n", rootInode.IPerm)
	fmt.Printf("Creación: %s\n", rootInode.ICtime.Format(time.RFC1123))
	fmt.Printf("Modificación: %s\n", rootInode.IMtime.Format(time.RFC1123))
	fmt.Printf("Acceso: %s\n", rootInode.IAtime.Format(time.RFC1123))

	fmt.Println("Bloques directos:")
	for i, blockPtr := range rootInode.IBlock {
		if i >= 12 {
			break // Solo mostrar los directos
		}
		if blockPtr != -1 {
			fmt.Printf("  Bloque[%d]: %d\n", i, blockPtr)
		}
	}

	// 9. Leer y mostrar el bloque de directorio raíz
	if rootInode.IBlock[0] != -1 {
		blockPos := startByte + int64(sb.SBlockStart) + int64(rootInode.IBlock[0])*int64(sb.SBlockSize)
		_, err = file.Seek(blockPos, 0)
		if err != nil {
			fmt.Printf("Error al posicionarse para leer bloque de directorio: %s\n", err)
			return
		}

		dirBlock := &DirectoryBlock{}
		if err := binary.Read(file, binary.LittleEndian, dirBlock); err != nil {
			fmt.Printf("Error al leer bloque de directorio: %s\n", err)
			return
		}

		// Corrección para la parte que busca users.txt en LogEXT2
		fmt.Println("\n=== CONTENIDO DEL DIRECTORIO RAÍZ ===")
		var foundUsersTxt bool = false
		for _, entry := range dirBlock.BContent {
			name := strings.TrimRight(string(entry.BName[:]), "\x00")
			if name != "" {
				fmt.Printf("  %-12s -> Inodo: %d\n", name, entry.BInodo)

				// Si encontramos users.txt, mostramos su contenido
				if name == "users.txt" {
					foundUsersTxt = true
					usersTxtInodeNum := entry.BInodo

					// Leer el inodo
					inodePos := startByte + int64(sb.SInodeStart) + int64(usersTxtInodeNum)*int64(sb.SInodeSize)
					_, err = file.Seek(inodePos, 0)
					if err != nil {
						fmt.Printf("Error al posicionarse para leer inodo de users.txt: %s\n", err)
						continue
					}

					usersTxtInode, err := readInodeFromDisc(file)
					if err != nil {
						fmt.Printf("Error al leer inodo de users.txt: %s\n", err)
						continue
					}

					fmt.Printf("\n=== INODO DE USERS.TXT (INODO %d) ===\n", usersTxtInodeNum)
					fmt.Printf("Tipo: %d (0=Carpeta, 1=Archivo)\n", usersTxtInode.IType)
					fmt.Printf("Tamaño: %d bytes\n", usersTxtInode.ISize)

					// Leer contenido del archivo
					if usersTxtInode.IBlock[0] != -1 {
						blockPos := startByte + int64(sb.SBlockStart) + int64(usersTxtInode.IBlock[0])*int64(sb.SBlockSize)
						_, err = file.Seek(blockPos, 0)
						if err != nil {
							fmt.Printf("Error al posicionarse para leer bloque de users.txt: %s\n", err)
							continue
						}

						contentBuf := make([]byte, usersTxtInode.ISize)
						_, err = file.Read(contentBuf)
						if err != nil {
							fmt.Printf("Error al leer contenido de users.txt: %s\n", err)
							continue
						}

						fmt.Println("\n=== CONTENIDO DE USERS.TXT ===")
						fmt.Println(string(contentBuf))
					}
				}
			}
		}

		if !foundUsersTxt {
			fmt.Println("\nNo se encontró el archivo users.txt en el directorio raíz")
		}
	}
}
