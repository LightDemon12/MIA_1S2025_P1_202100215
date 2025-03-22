package DiskManager

// MBR representa el Master Boot Record del disco
// Estructura que se almacena al inicio del disco (primer sector)
type MBR struct {
	MbrTamanio       int64        // Tamaño total: 8 bytes, capacidad hasta ~9 exabytes
	MbrFechaCreacion [30]byte     // Fecha creación: 30 bytes fijos para string (cambio: usar 16-20 bytes si es suficiente)
	MbrDskSignature  int32        // Firma disco: 4 bytes, valor aleatorio (cambio: uint32 para solo positivos)
	DskFit           byte         // Tipo ajuste: 1 byte, valores 'B'/'F'/'W' (cambio: usar 2 bits si se necesita espacio)
	MbrPartitions    [4]Partition // Particiones: 4 × tamaño(Partition), siempre 4 primarias
	// Tamaño total: 43 bytes + 4×tamaño(Partition)
}
