package DiskManager

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

type PartitionManager struct {
	diskPath  string
	mbr       *MBR
	validator *PartitionValidator
	fit       *PartitionFit
}

func NewPartitionManager(diskPath string) (*PartitionManager, error) {
	file, err := os.OpenFile(diskPath, os.O_RDWR, 0666)
	if err != nil {
		return nil, fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	mbr := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, mbr); err != nil {
		return nil, fmt.Errorf("error leyendo MBR: %v", err)
	}

	return &PartitionManager{
		diskPath:  diskPath,
		mbr:       mbr,
		validator: NewPartitionValidator(mbr, diskPath),
		fit:       NewPartitionFit(mbr, diskPath),
	}, nil
}

func (pm *PartitionManager) CreatePartition(partition *Partition, unit string) error {
	// 1. Convertir tamaño a bytes
	partition.Size = pm.convertToBytes(partition.Size, unit)

	// 2. Leer MBR actualizado del disco
	file, err := os.OpenFile(pm.diskPath, os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	// Leer MBR actual
	currentMBR := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, currentMBR); err != nil {
		return fmt.Errorf("error leyendo MBR: %v", err)
	}

	// 3. Validar reglas de particiones
	primarias := 0
	extendidas := 0

	// Mostrar estado actual
	fmt.Printf("Debug: Particiones actuales en disco:\n")
	for i, p := range currentMBR.MbrPartitions {
		if p.Size > 0 {
			fmt.Printf("  Partición %d: Type=%c, Name=%s\n",
				i+1, p.Type, strings.TrimRight(string(p.Name[:]), " "))

			if p.Type == PARTITION_PRIMARY {
				primarias++
			} else if p.Type == PARTITION_EXTENDED {
				extendidas++
			}
		}
	}

	// 4. Validaciones
	fmt.Printf("Debug: Conteo final - Primarias: %d, Extendidas: %d\n", primarias, extendidas)

	// Verificar si es una partición lógica
	if partition.Type == PARTITION_LOGIC { // CORREGIDO: PARTITION_LOGIC en lugar de PARTITION_LOGICAL
		// Para particiones lógicas, verificar que exista una partición extendida
		if extendidas == 0 {
			return fmt.Errorf("no se puede crear una partición lógica sin una partición extendida")
		}

		// Las particiones lógicas se manejan en otro método, no aplica el límite de 4
		return pm.CreateLogicalPartition(partition, unit)
	}

	// Solo aplica el límite para primarias y extendidas
	if primarias+extendidas >= 4 {
		return fmt.Errorf("no se pueden crear más particiones primarias o extendidas: límite máximo alcanzado (4)")
	}

	// Validar partición extendida única
	if partition.Type == PARTITION_EXTENDED && extendidas > 0 {
		return fmt.Errorf("ya existe una partición extendida en el disco")
	}

	// Validar nombre único
	partitionName := strings.TrimRight(string(partition.Name[:]), " ")
	for _, p := range currentMBR.MbrPartitions {
		if p.Size > 0 {
			existingName := strings.TrimRight(string(p.Name[:]), " ")
			if existingName == partitionName {
				return fmt.Errorf("ya existe una partición con el nombre '%s'", partitionName)
			}
		}
	}

	// 5. Buscar espacio según el algoritmo de ajuste
	if err := pm.fit.FindPartitionSpace(partition); err != nil {
		return err
	}

	// 6. Encontrar slot libre
	slotIndex := -1
	for i, p := range currentMBR.MbrPartitions {
		if p.Size == 0 {
			slotIndex = i
			break
		}
	}

	if slotIndex == -1 {
		return fmt.Errorf("no hay slots libres en el MBR")
	}

	// 7. Actualizar MBR con la nueva partición
	currentMBR.MbrPartitions[slotIndex] = *partition

	// 8. Volver a principio y escribir MBR actualizado
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("error posicionando cursor para escribir MBR: %v", err)
	}

	if err := binary.Write(file, binary.LittleEndian, currentMBR); err != nil {
		return fmt.Errorf("error escribiendo MBR: %v", err)
	}

	// 9. Escribir espacio para la partición
	zeros := make([]byte, partition.Size)
	if _, err := file.Seek(partition.Start, 0); err != nil {
		return fmt.Errorf("error posicionando cursor para partición: %v", err)
	}

	if _, err := file.Write(zeros); err != nil {
		return fmt.Errorf("error escribiendo espacio para partición: %v", err)
	}

	// 10. Log
	LogMBR(pm.diskPath)

	return nil
}

func (pm *PartitionManager) calculateNextStartPosition() int64 {
	// Leer MBR actual directamente del disco
	file, err := os.OpenFile(pm.diskPath, os.O_RDONLY, 0666)
	if err != nil {
		fmt.Printf("Error abriendo disco: %v\n", err)
		return int64(binary.Size(pm.mbr))
	}
	defer file.Close()

	diskMBR := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, diskMBR); err != nil {
		fmt.Printf("Error leyendo MBR: %v\n", err)
		return int64(binary.Size(pm.mbr))
	}

	// Posición inicial después del MBR
	mbrSize := int64(binary.Size(diskMBR))
	lastEndPosition := mbrSize

	// Mostrar todas las particiones del disco
	fmt.Printf("Debug: Particiones leídas directamente del disco:\n")
	for i, p := range diskMBR.MbrPartitions {
		if p.Size > 0 {
			fmt.Printf("  Partición %d: Start=%d, Size=%d, End=%d, Name=%s\n",
				i+1, p.Start, p.Size, p.Start+p.Size, string(p.Name[:]))

			endPos := p.Start + p.Size
			if endPos > lastEndPosition {
				lastEndPosition = endPos
			}
		}
	}

	fmt.Printf("Debug: Última posición calculada: %d\n", lastEndPosition)
	return lastEndPosition
}
func (pm *PartitionManager) convertToBytes(size int64, unit string) int64 {
	switch unit {
	case "B":
		return size
	case "K":
		return size * 1024
	case "M":
		return size * 1024 * 1024
	default:
		return size * 1024 // Default  KB
	}
}

func (pm *PartitionManager) updateMBR(partition *Partition) error {
	file, err := os.OpenFile(pm.diskPath, os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	if err := binary.Write(file, binary.LittleEndian, pm.mbr); err != nil {
		return fmt.Errorf("error escribiendo MBR: %v", err)
	}

	return nil
}

func (pm *PartitionManager) writePartitionToDisk(partition *Partition) error {
	file, err := os.OpenFile(pm.diskPath, os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	// Escribir MBR completo con todas las particiones
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("error posicionando cursor para MBR: %v", err)
	}

	if err := binary.Write(file, binary.LittleEndian, pm.mbr); err != nil {
		return fmt.Errorf("error escribiendo MBR: %v", err)
	}

	// Reservar espacio para la partición
	zeros := make([]byte, partition.Size)
	if _, err := file.Seek(partition.Start, 0); err != nil {
		return fmt.Errorf("error posicionando cursor para partición: %v", err)
	}

	if _, err := file.Write(zeros); err != nil {
		return fmt.Errorf("error escribiendo espacio para partición: %v", err)
	}

	// Manejar partición extendida
	if partition.Type == PARTITION_EXTENDED {
		ebr := NewEBR()
		ebr.Start = partition.Start

		if _, err := file.Seek(partition.Start, 0); err != nil {
			return fmt.Errorf("error posicionando cursor para EBR: %v", err)
		}

		if err := binary.Write(file, binary.LittleEndian, ebr); err != nil {
			return fmt.Errorf("error escribiendo EBR: %v", err)
		}
	}

	return nil
}

func (pm *PartitionManager) findFreePartitionSlot() int {
	for i, p := range pm.mbr.MbrPartitions {
		if p.Status == PARTITION_NOT_MOUNTED {
			return i
		}
	}
	return -1
}

// CreateLogicalPartition crea una partición lógica dentro de una partición extendida
func (pm *PartitionManager) CreateLogicalPartition(partition *Partition, unit string) error {
	// 1. Convertir tamaño a bytes
	partition.Size = pm.convertToBytes(partition.Size, unit)

	fmt.Printf("Debug: Creando partición lógica de tamaño %d bytes\n", partition.Size)

	// 2. Validar la partición
	if err := pm.validator.ValidateNewPartition(partition); err != nil {
		return fmt.Errorf("validación fallida: %v", err)
	}

	// 3. Buscar la partición extendida
	extendedPartition, err := pm.getExtendedPartition()
	if err != nil {
		return fmt.Errorf("error buscando partición extendida: %v", err)
	}

	fmt.Printf("Debug: Partición extendida encontrada en Start=%d, Size=%d\n",
		extendedPartition.Start, extendedPartition.Size)

	// 4. Verificar nombre único
	if err := pm.validateUniqueName(partition.Name[:]); err != nil {
		return fmt.Errorf("error de validación: %v", err)
	}

	// 5. Crear el EBR para la partición lógica
	ebr := NewEBR()
	ebr.Status = partition.Status
	ebr.Fit = partition.Fit
	ebr.Size = partition.Size
	copy(ebr.Name[:], partition.Name[:])

	fmt.Printf("Debug: Creado EBR con Fit=%c, Size=%d, Name=%s\n",
		ebr.Fit, ebr.Size, string(ebr.Name[:]))

	// 6. Encontrar espacio para la partición lógica
	if err := pm.findSpaceForLogicalPartition(extendedPartition, ebr); err != nil {
		return fmt.Errorf("error buscando espacio: %v", err)
	}

	// 7. Escribir el EBR en el disco
	if err := pm.writeEBRToDisk(ebr); err != nil {
		return fmt.Errorf("error escribiendo EBR: %v", err)
	}

	fmt.Printf("Debug: Partición lógica '%s' creada con éxito en posición %d y tamaño %d\n",
		strings.TrimRight(string(partition.Name[:]), " "), ebr.Start, ebr.Size)

	return nil
}

func (pm *PartitionManager) getExtendedPartition() (*Partition, error) {
	file, err := os.OpenFile(pm.diskPath, os.O_RDONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	currentMBR := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, currentMBR); err != nil {
		return nil, fmt.Errorf("error leyendo MBR: %v", err)
	}

	fmt.Printf("Debug: PARTITION_EXTENDED = %c (%d)\n", PARTITION_EXTENDED, PARTITION_EXTENDED)
	for i, p := range currentMBR.MbrPartitions {
		fmt.Printf("Debug: Partición %d: Type=%c (%d), Size=%d\n",
			i+1, p.Type, p.Type, p.Size)
		if p.Type == PARTITION_EXTENDED && p.Size > 0 {
			fmt.Printf("Debug: Partición extendida encontrada: Start=%d, Size=%d\n",
				p.Start, p.Size)
			return &p, nil
		}
	}

	return nil, fmt.Errorf("no existe una partición extendida en el disco")
}

// validateUniqueName verifica que el nombre de la partición no se repita
func (pm *PartitionManager) validateUniqueName(name []byte) error {
	// Verificar nombres en particiones primarias y extendidas
	file, err := os.OpenFile(pm.diskPath, os.O_RDONLY, 0666)
	if err != nil {
		return fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	currentMBR := &MBR{}
	if err := binary.Read(file, binary.LittleEndian, currentMBR); err != nil {
		return fmt.Errorf("error leyendo MBR: %v", err)
	}

	newName := strings.TrimRight(string(name), " ")

	// Verificar nombres en MBR
	for _, p := range currentMBR.MbrPartitions {
		if p.Size > 0 {
			existingName := strings.TrimRight(string(p.Name[:]), " ")
			if existingName == newName {
				return fmt.Errorf("ya existe una partición con el nombre '%s'", newName)
			}
		}
	}

	// Verificar nombres en particiones lógicas (EBRs)
	extendedPartition, err := pm.getExtendedPartition()
	if err == nil { // Si hay una partición extendida
		ebrs, err := pm.getAllEBRs(extendedPartition)
		if err == nil {
			for _, ebr := range ebrs {
				if ebr.Size > 0 {
					ebrName := strings.TrimRight(string(ebr.Name[:]), " ")
					if ebrName == newName {
						return fmt.Errorf("ya existe una partición lógica con el nombre '%s'", newName)
					}
				}
			}
		}
	}

	return nil
}

// getAllEBRs obtiene todos los EBRs dentro de una partición extendida
func (pm *PartitionManager) getAllEBRs(extendedPartition *Partition) ([]*EBR, error) {
	var ebrs []*EBR
	file, err := os.OpenFile(pm.diskPath, os.O_RDONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	// Iniciar desde el primer EBR en la partición extendida
	currentPos := extendedPartition.Start
	for currentPos != -1 {
		if _, err := file.Seek(currentPos, 0); err != nil {
			return ebrs, nil
		}

		ebr := &EBR{}
		if err := binary.Read(file, binary.LittleEndian, ebr); err != nil {
			return ebrs, nil
		}

		ebrs = append(ebrs, ebr)
		currentPos = ebr.Next
	}

	return ebrs, nil
}

// findSpaceForLogicalPartition busca espacio para una nueva partición lógica
func (pm *PartitionManager) findSpaceForLogicalPartition(extendedPartition *Partition, ebr *EBR) error {
	// Abrir el archivo de disco
	file, err := os.OpenFile(pm.diskPath, os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	// Verificar que la partición extendida tiene suficiente espacio para el EBR inicial
	ebrSize := int64(binary.Size(ebr))

	// Comprobar si hay suficiente espacio para la partición lógica
	if ebrSize+ebr.Size > extendedPartition.Size {
		return fmt.Errorf("la partición lógica es demasiado grande para la partición extendida")
	}

	fmt.Printf("Debug: Partición extendida: Start=%d, Size=%d\n",
		extendedPartition.Start, extendedPartition.Size)
	fmt.Printf("Debug: Tamaño de EBR: %d, Tamaño total requerido: %d\n",
		ebrSize, ebrSize+ebr.Size)

	// Posicionarse al inicio de la partición extendida
	if _, err := file.Seek(extendedPartition.Start, 0); err != nil {
		return fmt.Errorf("error posicionando cursor: %v", err)
	}

	// Leer el primer EBR
	firstEBR := &EBR{}
	err = binary.Read(file, binary.LittleEndian, firstEBR)

	// Si es la primera partición lógica o no hay un EBR válido, crear uno al inicio
	if err != nil || firstEBR.Size == 0 {
		fmt.Printf("Debug: Creando primera partición lógica en la partición extendida\n")
		ebr.Start = extendedPartition.Start
		ebr.Next = -1
		return nil
	}

	fmt.Printf("Debug: Buscando espacio en la cadena de EBRs existentes\n")

	// Navegar por la lista enlazada para encontrar el último EBR válido
	currentEBR := firstEBR
	var lastValidEBR *EBR = nil
	var lastPosition int64 = extendedPartition.Start

	for {
		// Si este EBR es válido, actualizamos lastValidEBR
		if currentEBR.Size > 0 {
			lastValidEBR = currentEBR
			lastPosition = currentEBR.Start

			fmt.Printf("Debug: EBR válido encontrado en %d, Size=%d, Next=%d\n",
				currentEBR.Start, currentEBR.Size, currentEBR.Next)
		}

		// Si no hay siguiente EBR, salimos del bucle
		if currentEBR.Next == -1 {
			break
		}

		// Leemos el siguiente EBR
		nextPos := currentEBR.Next
		if nextPos < extendedPartition.Start ||
			nextPos >= extendedPartition.Start+extendedPartition.Size {
			// El puntero Next está fuera de la partición extendida
			fmt.Printf("Debug: Error: puntero Next (%d) fuera de la partición extendida\n", nextPos)
			break
		}

		if _, err := file.Seek(nextPos, 0); err != nil {
			fmt.Printf("Debug: Error posicionando cursor: %v\n", err)
			break
		}

		nextEBR := &EBR{}
		if err := binary.Read(file, binary.LittleEndian, nextEBR); err != nil {
			fmt.Printf("Debug: Error leyendo siguiente EBR: %v\n", err)
			break
		}

		currentEBR = nextEBR
	}

	// Calcular la nueva posición después del último EBR válido
	var newStart int64
	if lastValidEBR != nil {
		newStart = lastValidEBR.Start + lastValidEBR.Size
		fmt.Printf("Debug: Última posición válida: %d, nueva posición: %d\n",
			lastPosition, newStart)
	} else {
		// Si no encontramos un EBR válido, usamos el inicio de la partición extendida
		newStart = extendedPartition.Start
		fmt.Printf("Debug: No se encontraron EBRs válidos, usando inicio de la extendida: %d\n",
			newStart)
	}

	// Verificar si hay espacio suficiente en la partición extendida
	if newStart+ebrSize+ebr.Size > extendedPartition.Start+extendedPartition.Size {
		return fmt.Errorf("no hay espacio suficiente en la partición extendida")
	}

	// Actualizar el último EBR para que apunte al nuevo
	if lastValidEBR != nil {
		lastValidEBR.Next = newStart

		// Escribir el último EBR actualizado
		if _, err := file.Seek(lastValidEBR.Start, 0); err != nil {
			return fmt.Errorf("error posicionando cursor: %v", err)
		}

		if err := binary.Write(file, binary.LittleEndian, lastValidEBR); err != nil {
			return fmt.Errorf("error actualizando último EBR: %v", err)
		}

		fmt.Printf("Debug: Actualizado EBR en posición %d para apuntar a %d\n",
			lastValidEBR.Start, newStart)
	}

	// Asignar posición al nuevo EBR
	ebr.Start = newStart
	ebr.Next = -1

	fmt.Printf("Debug: Nuevo EBR será creado en Start=%d con Size=%d\n",
		ebr.Start, ebr.Size)

	return nil
}

// writeEBRToDisk escribe un EBR en el disco
func (pm *PartitionManager) writeEBRToDisk(ebr *EBR) error {
	file, err := os.OpenFile(pm.diskPath, os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	fmt.Printf("Debug: Escribiendo EBR en posición %d\n", ebr.Start)

	// Posicionar en el inicio del EBR
	if _, err := file.Seek(ebr.Start, 0); err != nil {
		return fmt.Errorf("error posicionando cursor para EBR: %v", err)
	}

	// Escribir el EBR
	if err := binary.Write(file, binary.LittleEndian, ebr); err != nil {
		return fmt.Errorf("error escribiendo EBR: %v", err)
	}

	// Reservar espacio para los datos de la partición lógica
	ebrSize := int64(binary.Size(ebr))
	dataSize := ebr.Size - ebrSize

	fmt.Printf("Debug: Tamaño EBR=%d, Tamaño datos=%d\n", ebrSize, dataSize)

	if dataSize > 0 {
		zeros := make([]byte, dataSize)
		if _, err := file.Write(zeros); err != nil {
			return fmt.Errorf("error inicializando espacio para partición lógica: %v", err)
		}
		fmt.Printf("Debug: Inicializado espacio de datos: %d bytes\n", dataSize)
	}

	return nil
}
