package analizador

import "strings"

// CommandType representa los tipos de comandos disponibles
type CommandType string

const (
	CMD_MKDISK         CommandType = "mkdisk"
	CMD_RMDISK         CommandType = "rmdisk"
	CMD_FDISK          CommandType = "fdisk"
	CMD_MOUNT          CommandType = "mount"
	CMD_MOUNTED        CommandType = "mounted"
	CMD_REP            CommandType = "rep"
	CMD_MKFS           CommandType = "mkfs"
	CMD_EXT2AUTOINJECT CommandType = "ext2autoinject"
	CMD_LOGIN          CommandType = "login"
	CMD_LOGOUT         CommandType = "logout"
	CMD_CAT            CommandType = "cat"
	CMD_MKGRP          CommandType = "mkgrp"
	CMD_RMGRP          CommandType = "rmgrp"
	CMD_MKUSR          CommandType = "mkusr"
	CMD_RMUSR          CommandType = "rmusr"
	CMD_CHGRP          CommandType = "chgrp"
	CMD_MKFILE         CommandType = "mkfile"
	CMD_MKDIR          CommandType = "mkdir"
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
	case strings.HasPrefix(comando, string(CMD_MKGRP)):
		return CMD_MKGRP
	case strings.HasPrefix(comando, string(CMD_RMGRP)):
		return CMD_RMGRP
	case strings.HasPrefix(comando, string(CMD_MKUSR)):
		return CMD_MKUSR
	case strings.HasPrefix(comando, string(CMD_RMUSR)):
		return CMD_RMUSR
	case strings.HasPrefix(comando, string(CMD_CHGRP)):
		return CMD_CHGRP
	case strings.HasPrefix(comando, string(CMD_MKFILE)):
		return CMD_MKFILE
	case strings.HasPrefix(comando, string(CMD_MKDIR)): // Nuevo caso
		return CMD_MKDIR
	default:
		return ""
	}
}
