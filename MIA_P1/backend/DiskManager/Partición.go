package DiskManager

// Status types
const (
	PARTITION_NOT_MOUNTED = 0
	PARTITION_MOUNTED     = 1
)

// Tipos de particiones
const (
	PARTITION_PRIMARY  = 'P'
	PARTITION_EXTENDED = 'E'
	PARTITION_LOGIC    = 'L'
)

// Fit types
const (
	FIT_FIRST = 'F'
	FIT_BEST  = 'B'
	FIT_WORST = 'W'
)

// Partition representa una partición dentro del disco
type Partition struct {
	Status      byte     // '0': no montada, '1': montada
	Type        byte     // 'P': primaria, 'E': extendida, 'L': lógica
	Fit         byte     // 'B': Best, 'F': First, 'W': Worst
	Start       int64    // Inicio de la partición en bytes
	Size        int64    // Tamaño de la partición en bytes
	Name        [16]byte // Nombre de la partición
	Correlative int32    // Correlativo de montaje (-1 si no está montada)
	Id          [4]byte  // ID de montaje (vacío hasta que se monte)
}

func NewPartition() Partition {
	return Partition{
		Status:      PARTITION_NOT_MOUNTED,
		Type:        PARTITION_PRIMARY,
		Fit:         FIT_FIRST,
		Start:       -1,
		Size:        0,
		Correlative: -1,
	}
}
