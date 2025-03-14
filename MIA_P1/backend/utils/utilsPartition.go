package utils

type PartitionConfig struct {
	Size int    // Tamaño de la partición
	Path string // Ruta del disco
	Name string // Nombre de la partición
	Type string // Tipo de partición (P, E, L)
	Fit  string // Tipo de ajuste (BF, FF, WF)
	Unit string // Unidad de medida (B, K, M)
}

func NewPartitionConfig() PartitionConfig {
	return PartitionConfig{
		Type: "P",  // Default: Primaria
		Fit:  "FF", // Default: First Fit
		Unit: "K",  // Default: Kilobytes
	}
}
