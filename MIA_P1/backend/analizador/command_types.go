package analizador

import "strings"

// CommandType representa los tipos de comandos disponibles
type CommandType string

const (
	CMD_MKDISK CommandType = "mkdisk"
	CMD_RMDISK CommandType = "rmdisk"
	CMD_FDISK  CommandType = "fdisk"
)

func IdentificarComando(comando string) CommandType {
	comando = strings.ToLower(strings.TrimSpace(comando))

	switch {
	case strings.HasPrefix(comando, string(CMD_MKDISK)):
		return CMD_MKDISK
	case strings.HasPrefix(comando, string(CMD_RMDISK)):
		return CMD_RMDISK
	case strings.HasPrefix(comando, string(CMD_FDISK)):
		return CMD_FDISK
	default:
		return ""
	}
}
