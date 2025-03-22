package DiskManager

// EBR representa el Extended Boot Record, utilizado para manejar particiones lógicas
// dentro de una partición extendida. Cada partición lógica tiene su propio EBR
// que contiene metadatos sobre esa partición.
type EBR struct {
	Status byte     // Estado de la partición lógica (activa/inactiva/montada)
	Fit    byte     // Tipo de ajuste para asignación de espacio (First/Best/Worst)
	Start  int64    // Posición de inicio de la partición lógica en bytes desde el comienzo del disco
	Size   int64    // Tamaño total de la partición lógica en bytes
	Next   int64    // Posición del siguiente EBR en la cadena, o -1 si es el último
	Name   [16]byte // Identificador único de la partición lógica (16 bytes fijos)
}

// NewEBR crea una nueva instancia de EBR con valores predeterminados seguros.
// - Status inicializado como no montado
// - Fit configurado como primer ajuste (First Fit)
// - Start en -1 indicando que no tiene posición asignada
// - Size en 0 indicando que no tiene espacio asignado
// - Next en -1 indicando que no hay siguiente EBR en la cadena
func NewEBR() *EBR {
	return &EBR{
		Status: PARTITION_NOT_MOUNTED,
		Fit:    FIT_FIRST,
		Start:  -1,
		Size:   0,
		Next:   -1,
	}
}
