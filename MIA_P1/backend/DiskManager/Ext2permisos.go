package DiskManager

import (
	"MIA_P1/backend/common"
	"fmt"
	"strings"
)

// FilePermissions representa los diferentes tipos de acceso a archivos
const (
	PERM_READ     = 4
	PERM_WRITE    = 2
	PERM_EXECUTE  = 1
	FILE_READ_DIR = 3 // Nueva constante para lectura de directorios

)

// CheckFilePermissions verifica si un usuario tiene los permisos necesarios
func CheckFilePermissions(fileInode *Inode, requiredPerms int) error {
	// Si el usuario es admin o root (ID 1), tiene todos los permisos
	if common.ActiveUserID == 1 {
		fmt.Printf("DEBUG: Usuario root - acceso permitido\n")
		return nil
	}

	fmt.Printf("DEBUG: Verificando permisos - Usuario ID: %d, Grupo ID: %d\n",
		common.ActiveUserID, common.ActiveGroupID)

	var effectivePerms byte

	// Determinar qué conjunto de permisos aplica
	if fileInode.IUid == common.ActiveUserID {
		// El usuario es el dueño
		effectivePerms = fileInode.IPerm[0]
		fmt.Printf("DEBUG: Usando permisos de propietario: %03o\n", effectivePerms)
	} else if fileInode.IGid == common.ActiveGroupID {
		// El usuario está en el grupo
		effectivePerms = fileInode.IPerm[1]
		fmt.Printf("DEBUG: Usando permisos de grupo: %03o\n", effectivePerms)
	} else {
		// Otros usuarios
		effectivePerms = fileInode.IPerm[2]
		fmt.Printf("DEBUG: Usando permisos de otros: %03o\n", effectivePerms)
	}

	// Verificar si los permisos requeridos están presentes
	if int(effectivePerms)&requiredPerms != requiredPerms {
		// Construir mensaje de error detallado
		var missingPerms []string
		if requiredPerms&PERM_READ > 0 && effectivePerms&PERM_READ == 0 {
			missingPerms = append(missingPerms, "lectura")
		}
		if requiredPerms&PERM_WRITE > 0 && effectivePerms&PERM_WRITE == 0 {
			missingPerms = append(missingPerms, "escritura")
		}
		if requiredPerms&PERM_EXECUTE > 0 && effectivePerms&PERM_EXECUTE == 0 {
			missingPerms = append(missingPerms, "ejecución")
		}

		return fmt.Errorf("permisos insuficientes: falta permiso de %s", strings.Join(missingPerms, ", "))
	}

	fmt.Printf("DEBUG: Permisos verificados correctamente\n")
	return nil
}
