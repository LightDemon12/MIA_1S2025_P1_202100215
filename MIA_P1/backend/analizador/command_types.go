package analizador

import "strings"

// CommandType representa los tipos de comandos disponibles
type CommandType string

const (
	CMD_MKDISK         CommandType = "mkdisk"
	CMD_RMDISK         CommandType = "rmdisk"
	CMD_FDISK          CommandType = "fdisk"
	CMD_MOUNT          CommandType = "mount"   // Agregar esta línea
	CMD_MOUNTED        CommandType = "mounted" // Nuevo comando
	CMD_REP            CommandType = "rep"     // Agregar esta línea
	CMD_MKFS           CommandType = "mkfs"
	CMD_EXT2AUTOINJECT CommandType = "ext2autoinject" // Nuevo comando
	CMD_LOGIN          CommandType = "login"          // Nuevo comando
	CMD_LOGOUT         CommandType = "logout"         // También añadimos el logout
	CMD_CAT            CommandType = "cat"            // Nuevo comando

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
	case strings.HasPrefix(comando, string(CMD_REP)):
		return CMD_REP
	case strings.HasPrefix(comando, string(CMD_MKFS)): // Añadir esta línea
		return CMD_MKFS
	case strings.HasPrefix(comando, string(CMD_EXT2AUTOINJECT)): // Nuevo caso
		return CMD_EXT2AUTOINJECT
	case strings.HasPrefix(comando, string(CMD_LOGIN)):
		return CMD_LOGIN
	case strings.HasPrefix(comando, string(CMD_LOGOUT)):
		return CMD_LOGOUT
	case strings.HasPrefix(comando, string(CMD_CAT)):
		return CMD_CAT
	default:
		return ""
	}
}
