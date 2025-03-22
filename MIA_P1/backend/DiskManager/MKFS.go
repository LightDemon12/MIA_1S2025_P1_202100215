// mkfs.go
package DiskManager

import (
	"MIA_P1/backend/utils"
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"time"
)

// FormatearParticion formatea una partición con el sistema de archivos EXT2
func FormatearParticion(id, formatType string) (bool, string) {
	// 1. Verificar que exista el ID de la partición montada
	mountedPartition, err := FindMountedPartitionById(id)
	if err != nil {
		return false, fmt.Sprintf("Error: %s", err)
	}

	// 2. Verificar que el disco existe físicamente
	file, err := os.OpenFile(mountedPartition.DiskPath, os.O_RDWR, 0666)
	if err != nil {
		return false, fmt.Sprintf("Error al abrir el disco: %s", err)
	}
	defer file.Close()

	// 3. Buscar los detalles de la partición en el disco
	startByte, size, err := GetPartitionDetails(file, mountedPartition)
	if err != nil {
		return false, fmt.Sprintf("Error al obtener detalles de la partición: %s", err)
	}

	fmt.Printf("Inicializando partición con ceros desde %d, tamaño %d bytes...\n", startByte, size)
	zeroBuffer := make([]byte, 8192) // Buffer de 8KB para eficiencia
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para inicializar partición: %s", err)
	}

	remaining := size
	for remaining > 0 {
		writeSize := int64(len(zeroBuffer))
		if remaining < writeSize {
			writeSize = remaining
		}

		_, err = file.Write(zeroBuffer[:writeSize])
		if err != nil {
			return false, fmt.Sprintf("Error al inicializar partición: %s", err)
		}

		remaining -= writeSize
	}

	// Regresar al inicio de la partición
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse después de inicializar: %s", err)
	}

	// 4. Calcular el tamaño de las estructuras EXT2 para esta partición
	extInfo := CalculateEXT2Format(size)

	// 5. Verificar que el formato sea válido para esta partición
	if !ValidateEXT2Format(extInfo) {
		return false, "Error: La partición es demasiado pequeña para formatear con EXT2"
	}

	// 6. Crear e inicializar las estructuras

	// 6.1 Crear el Superbloque
	superbloque := NewSuperBlock(
		int32(extInfo.InodeCount),
		int32(extInfo.BlockCount),
		int32(extInfo.InodeSize),
		int32(extInfo.BlockSize),
		int32(extInfo.SuperBlockSize),
		int32(extInfo.SuperBlockSize+extInfo.InodeBitmapSize),
		int32(extInfo.SuperBlockSize+extInfo.InodeBitmapSize+extInfo.BlockBitmapSize),
		int32(extInfo.SuperBlockSize+extInfo.InodeBitmapSize+extInfo.BlockBitmapSize+extInfo.InodeTableSize),
	)

	// Ajustar contadores de uso
	superbloque.SFreeBlocksCount = superbloque.SBlocksCount - 2 // Para directorio raíz y users.txt
	superbloque.SFreeInodesCount = superbloque.SInodesCount - 2 // Para inodo raíz y users.txt

	// 6.2 Crear los Bitmaps
	bitmapMgr := NewBitmapManager(extInfo.InodeCount, extInfo.BlockCount)

	// 6.3 Reservar los primeros inodos y bloques
	bitmapMgr.ReserveInitialBlocks(EXT2_RESERVED_INODES, EXT2_RESERVED_INODES)

	// 6.4 Crear el directorio raíz (inodo 2)
	rootInode := NewInode(0, 0, INODE_FOLDER) // UID 0, GID 0, tipo Carpeta
	// Establecer permisos razonables
	rootInode.IPerm[0] = 7 // rwx para propietario
	rootInode.IPerm[1] = 5 // r-x para grupo
	rootInode.IPerm[2] = 5 // r-x para otros
	rootInode.ISize = 64   // Tamaño típico de directorio

	// 6.5 Crear el primer bloque de directorio para el directorio raíz
	rootDirBlock := NewDirectoryBlock()
	rootDirBlock.InitializeAsDirectory(2, 2, "/") // Ahora incluye el nombre

	// 6.6 Asignar el bloque al inodo raíz
	firstRootBlock := bitmapMgr.AllocateBlock()
	rootInode.AddDirectBlock(int32(firstRootBlock))
	// Antes de escribir los bitmaps
	fmt.Println("Bitmap de inodos antes de escribir:")
	for i := 0; i < 2; i++ {
		fmt.Printf("%08b ", bitmapMgr.InodeBitmap[i])
	}
	fmt.Println()

	// Justo después de llamar a MarkInodeAsUsed
	bitmapMgr.MarkInodeAsUsed(2) // Inodo raíz
	fmt.Printf("Después de marcar inodo 2: %08b\n", bitmapMgr.InodeBitmap[0])
	bitmapMgr.MarkBlockAsUsed(firstRootBlock)

	// 6.7 Crear el archivo users.txt en la raíz
	fmt.Printf("Valor de INODE_FILE: %d\n", INODE_FILE)
	usersInode := NewInode(0, 0, INODE_FILE) // Usar la constante INODE_FILE
	fmt.Printf("Tipo de usersInode después de creación: %d\n", usersInode.IType)

	// Verificar qué valor tiene INODE_FILE
	fmt.Printf("VERIFICACIÓN: Valor de INODE_FILE: %d\n", INODE_FILE)

	// Establecer permisos razonables
	usersInode.IPerm[0] = 6 // rw- para propietario
	usersInode.IPerm[1] = 4 // r-- para grupo
	usersInode.IPerm[2] = 4 // r-- para otros

	// 6.8 Contenido inicial del archivo users.txt
	usersContent := "1, G, root\n1, U, root, root, 123\n"
	usersInode.ISize = int32(len(usersContent))

	// 6.9 Crear el bloque de archivo para users.txt
	usersBlock := NewFileBlock()
	bytesWritten := usersBlock.WriteContent([]byte(usersContent))
	usersInode.ISize = int32(bytesWritten)

	// 6.10 Asignar el bloque al inodo del archivo
	firstUsersBlock := bitmapMgr.AllocateBlock()
	usersInode.AddDirectBlock(int32(firstUsersBlock))

	// 6.11 Añadir la entrada para users.txt en el directorio raíz
	usersInodeNum := bitmapMgr.AllocateInode()
	if !rootDirBlock.AddEntry("users.txt", int32(usersInodeNum)) {
		fmt.Println("Error: No se pudo añadir users.txt al directorio")
	}
	fmt.Println("Verificando entradas del directorio raíz:")
	rootDirBlock.PrintEntries()
	// Marcar explícitamente el inodo y bloque de users.txt como usados
	bitmapMgr.MarkInodeAsUsed(usersInodeNum)
	bitmapMgr.MarkBlockAsUsed(firstUsersBlock)

	// 7. Escribir todas las estructuras en el disco

	// 7.1 Posicionarse al inicio de la partición
	_, err = file.Seek(startByte, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse en el disco: %s", err)
	}

	// 7.2 Escribir el Superbloque
	err = writeStructToDisc(file, superbloque)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir el superbloque: %s", err)
	}

	// 7.3 Escribir el Bitmap de Inodos - CORREGIDO: posicionarse explícitamente
	bmInodePos := startByte + int64(superbloque.SBmInodeStart)
	fmt.Printf("Escribiendo bitmap de inodos en posición: %d\n", bmInodePos)
	_, err = file.Seek(bmInodePos, 0) // Posicionarse explícitamente
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para escribir bitmap de inodos: %s", err)
	}

	_, err = file.Write(bitmapMgr.InodeBitmap)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir el bitmap de inodos: %s", err)
	}

	// 7.4 Escribir el Bitmap de Bloques - CORREGIDO: posicionarse explícitamente
	bmBlockPos := startByte + int64(superbloque.SBmBlockStart)
	fmt.Printf("Escribiendo bitmap de bloques en posición: %d\n", bmBlockPos)
	_, err = file.Seek(bmBlockPos, 0) // Posicionarse explícitamente
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para escribir bitmap de bloques: %s", err)
	}

	_, err = file.Write(bitmapMgr.BlockBitmap)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir el bitmap de bloques: %s", err)
	}

	calculatedSize := calculateInodeSize()
	if calculatedSize != INODE_SIZE {
		fmt.Printf("ADVERTENCIA: Tamaño calculado de Inode (%d) no coincide con el declarado (%d)\n",
			calculatedSize, INODE_SIZE)
	}

	// 2. Ajustar el ciclo de escritura de inodos - algo está mal aquí
	// 7.5 Escribir los Inodos
	inodeTablePos := startByte + int64(superbloque.SInodeStart)
	fmt.Printf("Posición inicial de tabla de inodos: %d\n", inodeTablePos)
	_, err = file.Seek(inodeTablePos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para escribir inodos: %s", err)
	}

	// Modificación del ciclo para incluir el inodo de users.txt
	var maxInodeToWrite int = EXT2_RESERVED_INODES
	if usersInodeNum >= EXT2_RESERVED_INODES {
		maxInodeToWrite = usersInodeNum + 1 // Incluir el inodo de users.txt
		fmt.Printf("Ampliando escritura de inodos hasta %d para incluir users.txt\n", maxInodeToWrite)
	}

	for i := 0; i < maxInodeToWrite; i++ {
		var currentPos int64
		currentPos, err = file.Seek(0, os.SEEK_CUR)
		if err != nil {
			return false, fmt.Sprintf("Error al obtener posición: %s", err)
		}

		fmt.Printf("Escribiendo inodo %d en posición %d\n", i, currentPos)

		if i == 2 {
			// Inodo raíz (#2)
			debugInode("Inodo raíz antes de escribir", rootInode)
			err = writeInodeToDisc(file, rootInode)
		} else if i == usersInodeNum {
			fmt.Printf("Escribiendo users.txt (inodo %d) con tipo %d\n", i, usersInode.IType)
			// Verificar valores
			if usersInode.ISize != int32(len(usersContent)) {
				fmt.Printf("CORRIGIENDO: Tamaño de users.txt de %d a %d\n", usersInode.ISize, len(usersContent))
				usersInode.ISize = int32(len(usersContent))
			}
			debugInode("Inodo users.txt antes de escribir", usersInode)
			err = writeInodeToDisc(file, usersInode)
		} else {
			// Inodo vacío para los demás
			emptyInode := NewInode(0, 0, 0)
			err = writeInodeToDisc(file, emptyInode)
		}

		if err != nil {
			return false, fmt.Sprintf("Error al escribir inodo %d: %s", i, err)
		}

		// Verificar posición después de escribir
		var newPos int64
		newPos, err = file.Seek(0, os.SEEK_CUR)
		if err != nil {
			return false, fmt.Sprintf("Error al obtener nueva posición: %s", err)
		}

		fmt.Printf("Nueva posición después de escribir inodo %d: %d (avanzó %d bytes)\n",
			i, newPos, newPos-currentPos)
	}

	// 7.6 Escribir los Bloques de Datos
	// Primero el bloque del directorio raíz
	rootBlockPos := startByte + int64(superbloque.SBlockStart) + int64(firstRootBlock)*int64(superbloque.SBlockSize)
	_, err = file.Seek(rootBlockPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para escribir el directorio raíz: %s", err)
	}

	err = writeStructToDisc(file, rootDirBlock)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir el directorio raíz: %s", err)
	}

	// Luego el bloque del archivo users.txt
	usersBlockPos := startByte + int64(superbloque.SBlockStart) + int64(firstUsersBlock)*int64(superbloque.SBlockSize)
	_, err = file.Seek(usersBlockPos, 0)
	if err != nil {
		return false, fmt.Sprintf("Error al posicionarse para escribir users.txt: %s", err)
	}

	fmt.Printf("Escribiendo contenido de users.txt en el bloque %d, posición %d\n",
		firstUsersBlock, usersBlockPos)
	fmt.Printf("Contenido a escribir: %s (tamaño: %d)\n", usersContent, len(usersContent))

	err = writeStructToDisc(file, usersBlock, superbloque.SBlockSize)
	if err != nil {
		return false, fmt.Sprintf("Error al escribir el archivo users.txt: %s", err)
	}

	// 8. Actualizar el estado de la partición en la estructura interna
	for _, mp := range utils.MountedPartitions {
		if mp.ID == id {
			break
		}
	}

	// Mostrar el log automáticamente después del formateo exitoso
	fmt.Println("\n======= INFORMACIÓN DEL FORMATEO EXT2 =======")
	LogEXT2(id) // Llamar a la función LogEXT2 que ya tenemos
	fmt.Println("=============================================\n")

	return true, fmt.Sprintf("Partición %s formateada exitosamente con sistema EXT2", id)
}

// readInodeFromDisc lee un Inode desde disco, manejando time.Time correctamente
func readInodeFromDisc(file *os.File) (*Inode, error) {
	inode := &Inode{}

	pos, _ := file.Seek(0, os.SEEK_CUR)
	fmt.Printf("Leyendo inodo desde posición: %d\n", pos)

	// Leer campos en el mismo orden exacto que writeInodeToDisc
	binary.Read(file, binary.LittleEndian, &inode.IUid)
	binary.Read(file, binary.LittleEndian, &inode.IGid)
	binary.Read(file, binary.LittleEndian, &inode.ISize)
	binary.Read(file, binary.LittleEndian, &inode.IPerm)

	// Leer timestamps
	var aTime, cTime, mTime int64
	binary.Read(file, binary.LittleEndian, &aTime)
	binary.Read(file, binary.LittleEndian, &cTime)
	binary.Read(file, binary.LittleEndian, &mTime)

	// Convertir timestamps
	inode.IAtime = time.Unix(aTime, 0)
	inode.ICtime = time.Unix(cTime, 0)
	inode.IMtime = time.Unix(mTime, 0)

	// Leer bloques directos e indirectos
	binary.Read(file, binary.LittleEndian, &inode.IBlock)

	// IMPORTANTE: El tipo debe leerse en la posición correcta
	binary.Read(file, binary.LittleEndian, &inode.IType)
	fmt.Printf("Tipo de inodo leído: %d\n", inode.IType)

	// Leer padding
	binary.Read(file, binary.LittleEndian, &inode.IPadding)

	// Ignorar bytes de padding adicionales que se agregaron en writeInodeToDisc
	if INODE_SIZE > 104 {
		padding := make([]byte, INODE_SIZE-104)
		binary.Read(file, binary.LittleEndian, &padding)
	}

	return inode, nil
}

// Métodos auxiliares para el BitmapManager
func (bm *BitmapManager) MarkInodeAsUsed(inodeNum int) {
	if inodeNum < 0 || inodeNum >= len(bm.InodeBitmap)*8 {
		return // Fuera de rango
	}

	byteIndex := inodeNum / 8
	bitIndex := inodeNum % 8
	bm.InodeBitmap[byteIndex] |= (1 << bitIndex)
}

func (bm *BitmapManager) MarkBlockAsUsed(blockNum int) {
	if blockNum < 0 || blockNum >= len(bm.BlockBitmap)*8 {
		return // Fuera de rango
	}

	byteIndex := blockNum / 8
	bitIndex := blockNum % 8
	bm.BlockBitmap[byteIndex] |= (1 << bitIndex)
}

// findMountedPartitionById busca una partición montada por su ID
func FindMountedPartitionById(id string) (*utils.MountedPartition, error) {
	for i, mp := range utils.MountedPartitions {
		if mp.ID == id {
			return &utils.MountedPartitions[i], nil
		}
	}
	return nil, fmt.Errorf("partición con ID %s no encontrada", id)
}

// getPartitionDetails obtiene el inicio y tamaño de una partición montada
func GetPartitionDetails(file *os.File, mp *utils.MountedPartition) (int64, int64, error) {
	if mp.PartitionType == PARTITION_PRIMARY {
		// Leer el MBR
		mbr := &MBR{}
		if _, err := file.Seek(0, 0); err != nil {
			return 0, 0, err
		}

		if err := binary.Read(file, binary.LittleEndian, mbr); err != nil {
			return 0, 0, err
		}

		// Buscar la partición por nombre
		for _, p := range mbr.MbrPartitions {
			pName := strings.TrimRight(string(p.Name[:]), " \x00")
			if pName == mp.PartitionName {
				return p.Start, p.Size, nil
			}
		}
	} else if mp.PartitionType == PARTITION_LOGIC {
		// Leer el MBR para encontrar la extendida
		mbr := &MBR{}
		if _, err := file.Seek(0, 0); err != nil {
			return 0, 0, err
		}

		if err := binary.Read(file, binary.LittleEndian, mbr); err != nil {
			return 0, 0, err
		}

		// Buscar la partición extendida
		var extendedPartition *Partition
		for i, p := range mbr.MbrPartitions {
			if p.Type == PARTITION_EXTENDED && p.Size > 0 {
				extendedPartition = &mbr.MbrPartitions[i]
				break
			}
		}

		if extendedPartition == nil {
			return 0, 0, fmt.Errorf("no se encontró la partición extendida que contiene la lógica")
		}

		// Navegar por los EBRs
		currentPos := extendedPartition.Start
		for currentPos != -1 {
			if _, err := file.Seek(currentPos, 0); err != nil {
				break
			}

			ebr := &EBR{}
			if err := binary.Read(file, binary.LittleEndian, ebr); err != nil {
				break
			}

			ebrName := strings.TrimRight(string(ebr.Name[:]), " \x00")
			if ebrName == mp.PartitionName && ebr.Size > 0 {
				return currentPos + int64(binary.Size(ebr)), ebr.Size, nil
			}

			// Avanzar al siguiente EBR
			if ebr.Next == -1 {
				break
			}
			currentPos = ebr.Next
		}
	}

	return 0, 0, fmt.Errorf("no se encontró la partición %s en el disco", mp.PartitionName)
}

// Implementar writeDirectoryBlockToDisc para asegurar correcta serialización
func writeDirectoryBlockToDisc(file *os.File, dirBlock *DirectoryBlock) error {
	// Crear un buffer con tamaño exacto
	buf := new(bytes.Buffer)

	// Escribir cada entrada manualmente
	for i := 0; i < B_CONTENT_COUNT; i++ {
		// Escribir el nombre (array fijo)
		if _, err := buf.Write(dirBlock.BContent[i].BName[:]); err != nil {
			return err
		}

		// Escribir el número de inodo
		if err := binary.Write(buf, binary.LittleEndian, dirBlock.BContent[i].BInodo); err != nil {
			return err
		}
	}

	// Escribir el buffer completo al archivo
	_, err := file.Write(buf.Bytes())
	return err
}

func writeStructToDisc(file *os.File, data interface{}, blockSize ...int32) error {
	switch v := data.(type) {
	case *SuperBlock:
		return writeSuperBlockToDisc(file, v)
	case *Inode:
		return writeInodeToDisc(file, v)
	case *DirectoryBlock:
		return writeDirectoryBlockToDisc(file, v)
	case *FileBlock:
		// Crear buffer del tamaño de bloque
		size := int32(64) // Tamaño predeterminado
		if len(blockSize) > 0 {
			size = blockSize[0]
		}

		blockBuffer := make([]byte, size)
		// Copiar contenido al inicio del buffer
		copy(blockBuffer, v.BContent[:])
		// El resto del buffer ya está inicializado a ceros
		_, err := file.Write(blockBuffer)
		return err
	default:
		return binary.Write(file, binary.LittleEndian, data)
	}
}

// writeSuperBlockToDisc escribe un SuperBlock en el disco, manejando time.Time correctamente
func writeSuperBlockToDisc(file *os.File, sb *SuperBlock) error {
	// Crear un buffer para contener el superbloque serializado
	buf := new(bytes.Buffer)

	// Escribir campos simples primero
	if err := binary.Write(buf, binary.LittleEndian, sb.SFilesystemType); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, sb.SInodesCount); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, sb.SBlocksCount); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, sb.SFreeBlocksCount); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, sb.SFreeInodesCount); err != nil {
		return err
	}

	// Convertir time.Time a int64 (Unix timestamp)
	mTimeUnix := sb.SMtime.Unix()
	if err := binary.Write(buf, binary.LittleEndian, mTimeUnix); err != nil {
		return err
	}

	uTimeUnix := sb.SUmtime.Unix()
	if err := binary.Write(buf, binary.LittleEndian, uTimeUnix); err != nil {
		return err
	}

	// Continuar con el resto de campos simples
	if err := binary.Write(buf, binary.LittleEndian, sb.SMntCount); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, sb.SMagic); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, sb.SInodeSize); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, sb.SBlockSize); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, sb.SFirstIno); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, sb.SFirstBlo); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, sb.SBmInodeStart); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, sb.SBmBlockStart); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, sb.SInodeStart); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, sb.SBlockStart); err != nil {
		return err
	}

	// Escribir el padding
	if err := binary.Write(buf, binary.LittleEndian, sb.SPadding); err != nil {
		return err
	}

	// Escribir el buffer completo al archivo
	_, err := file.Write(buf.Bytes())
	return err
}

// Función similar para leer un SuperBlock desde disco
func ReadSuperBlockFromDisc(file *os.File) (*SuperBlock, error) {
	sb := &SuperBlock{}

	// Leer campos simples primero
	if err := binary.Read(file, binary.LittleEndian, &sb.SFilesystemType); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &sb.SInodesCount); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &sb.SBlocksCount); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &sb.SFreeBlocksCount); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &sb.SFreeInodesCount); err != nil {
		return nil, err
	}

	// Leer timestamps como int64
	var mTimeUnix, uTimeUnix int64
	if err := binary.Read(file, binary.LittleEndian, &mTimeUnix); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &uTimeUnix); err != nil {
		return nil, err
	}

	// Convertir a time.Time
	sb.SMtime = time.Unix(mTimeUnix, 0)
	sb.SUmtime = time.Unix(uTimeUnix, 0)

	// Continuar con el resto de campos simples
	if err := binary.Read(file, binary.LittleEndian, &sb.SMntCount); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &sb.SMagic); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &sb.SInodeSize); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &sb.SBlockSize); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &sb.SFirstIno); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &sb.SFirstBlo); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &sb.SBmInodeStart); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &sb.SBmBlockStart); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &sb.SInodeStart); err != nil {
		return nil, err
	}
	if err := binary.Read(file, binary.LittleEndian, &sb.SBlockStart); err != nil {
		return nil, err
	}

	// Leer el padding
	if err := binary.Read(file, binary.LittleEndian, &sb.SPadding); err != nil {
		return nil, err
	}

	return sb, nil
}

const (
	INODE_SIZE = 160 // Tamaño total exacto
)

// 1. Implementa un método para calcular el tamaño real de la estructura Inode
func calculateInodeSize() int {
	var inode Inode
	size := binary.Size(inode.IUid)     // 4 bytes (int32)
	size += binary.Size(inode.IGid)     // 4 bytes (int32)
	size += binary.Size(inode.ISize)    // 4 bytes (int32)
	size += binary.Size(inode.IPerm)    // 3 bytes ([3]byte)
	size += 24                          // 3 timestamps de 8 bytes (int64)
	size += binary.Size(inode.IBlock)   // 60 bytes (15*int32)
	size += binary.Size(inode.IType)    // 1 byte (byte)
	size += binary.Size(inode.IPadding) // 60 bytes ([60]byte)

	fmt.Printf("Tamaño calculado de Inode: %d bytes\n", size)
	return size
}

// 2. Función para escribir un inodo byte a byte
func writeInodeToDisc(file *os.File, inode *Inode) error {
	// Imprimir el inodo para depuración
	debugInode("Escribiendo inodo", inode)

	// Obtener la posición actual
	pos, _ := file.Seek(0, os.SEEK_CUR)
	fmt.Printf("Escribiendo inodo en posición: %d\n", pos)

	// Crear buffer con tamaño exacto
	buf := new(bytes.Buffer)

	// Escribir campos en orden preciso
	if err := binary.Write(buf, binary.LittleEndian, inode.IUid); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, inode.IGid); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, inode.ISize); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, inode.IPerm); err != nil {
		return err
	}

	// Timestamps
	if err := binary.Write(buf, binary.LittleEndian, inode.IAtime.Unix()); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, inode.ICtime.Unix()); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, inode.IMtime.Unix()); err != nil {
		return err
	}

	// Bloques directos e indirectos
	if err := binary.Write(buf, binary.LittleEndian, inode.IBlock); err != nil {
		return err
	}

	// Tipo
	if err := binary.Write(buf, binary.LittleEndian, inode.IType); err != nil {
		return err
	}

	// Padding
	if err := binary.Write(buf, binary.LittleEndian, inode.IPadding); err != nil {
		return err
	}

	// Verificar tamaño
	serializedSize := buf.Len()
	fmt.Printf("Tamaño serializado: %d bytes\n", serializedSize)

	if serializedSize != INODE_SIZE {
		fmt.Printf("ADVERTENCIA: El tamaño serializado (%d) no coincide con INODE_SIZE (%d)\n",
			serializedSize, INODE_SIZE)

		// Si es menor, añadir padding
		if serializedSize < INODE_SIZE {
			padding := make([]byte, INODE_SIZE-serializedSize)
			buf.Write(padding)
			fmt.Printf("Añadidos %d bytes de padding\n", INODE_SIZE-serializedSize)
		}
	}

	// Escribir el buffer completo
	bytesWritten, err := file.Write(buf.Bytes())
	if err != nil {
		return err
	}

	fmt.Printf("Bytes escritos: %d\n", bytesWritten)
	return nil
}

// 4. Función de depuración para imprimir detalles del inodo
func debugInode(prefix string, inode *Inode) {
	fmt.Printf("=== %s ===\n", prefix)
	fmt.Printf("UID: %d\n", inode.IUid)
	fmt.Printf("GID: %d\n", inode.IGid)
	fmt.Printf("Size: %d\n", inode.ISize)
	fmt.Printf("Permisos: %v\n", inode.IPerm)
	fmt.Printf("Tipo: %d\n", inode.IType)

	fmt.Printf("Bloques directos: ")
	for i := 0; i < 5; i++ {
		if i < len(inode.IBlock) && inode.IBlock[i] != -1 {
			fmt.Printf("%d ", inode.IBlock[i])
		}
	}
	fmt.Println()
}
