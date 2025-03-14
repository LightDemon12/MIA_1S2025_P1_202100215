package analizador

import "strings"

// CommandType representa los tipos de comandos disponibles
type CommandType string

const (
	CMD_MKDISK CommandType = "mkdisk"
	CMD_RMDISK CommandType = "rmdisk"
	CMD_FDISK  CommandType = "fdisk"
	// Agregar más comandos aquí
)

// IdentificarComando determina el tipo de comando basado en el input
func IdentificarComando(comando string) CommandType {
	comando = strings.ToLower(strings.TrimSpace(comando))
	if strings.HasPrefix(comando, string(CMD_MKDISK)) {
		return CMD_MKDISK
	}
	// Agregar más identificadores aquí
	return ""
}
