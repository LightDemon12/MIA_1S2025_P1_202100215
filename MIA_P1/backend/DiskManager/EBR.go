package DiskManager

type EBR struct {
	Status byte     // Estado de la partición lógica
	Fit    byte     // Tipo de ajuste
	Start  int64    // Inicio de la partición lógica
	Size   int64    // Tamaño de la partición lógica
	Next   int64    // Apuntador al siguiente EBR (-1 si no hay siguiente)
	Name   [16]byte // Nombre de la partición lógica
}

func NewEBR() *EBR {
	return &EBR{
		Status: PARTITION_NOT_MOUNTED,
		Fit:    FIT_FIRST,
		Start:  -1,
		Size:   0,
		Next:   -1,
	}
}
