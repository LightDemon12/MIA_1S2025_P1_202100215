package DiskManager

// MBR representa el Master Boot Record del disco
type MBR struct {
	MbrTamanio       int64        // Tamaño total del disco en bytes
	MbrFechaCreacion [30]byte     // Fecha y hora de creación del disco como string
	MbrDskSignature  int32        // Número random que identifica al disco
	DskFit           byte         // Tipo de ajuste (B: Best, F: First, W: Worst)
	MbrPartitions    [4]Partition // Arreglo de 4 particiones
}

// Partition representa una partición dentro del disco
