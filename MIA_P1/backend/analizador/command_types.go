package analizador

import "strings"

// CommandType representa los tipos de comandos disponibles
type CommandType string

const (
	CMD_MKDISK  CommandType = "mkdisk"
	CMD_RMDISK  CommandType = "rmdisk"
	CMD_FDISK   CommandType = "fdisk"
	CMD_MOUNT   CommandType = "mount"   // Agregar esta línea
	CMD_MOUNTED CommandType = "mounted" // Nuevo comando

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
	case strings.HasPrefix(comando, string(CMD_MOUNTED)):
		return CMD_MOUNTED
	case strings.HasPrefix(comando, string(CMD_MOUNT)):
		return CMD_MOUNT

	default:
		return ""
	}
}
