package DiskManager

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// OpenReport abre automáticamente la imagen generada por un reporte usando el visor predeterminado del sistema
func OpenReport(reportPath string) error {
	// Verificar que el archivo exista
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		return fmt.Errorf("el archivo no existe: %s", reportPath)
	}

	fmt.Printf("Abriendo reporte: %s\n", reportPath)

	var cmd *exec.Cmd

	// Detectar el sistema operativo y usar el comando apropiado
	switch runtime.GOOS {
	case "linux":
		// En Linux, intentar varios comandos comunes
		// xdg-open es el estándar para la mayoría de distribuciones
		cmd = exec.Command("xdg-open", reportPath)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", reportPath)
	case "darwin": // macOS
		cmd = exec.Command("open", reportPath)
	default:
		return fmt.Errorf("sistema operativo no soportado: %s", runtime.GOOS)
	}

	// Ejecutar el comando en segundo plano
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error al abrir el reporte: %v", err)
	}

	// No esperamos a que termine, lo dejamos correr en segundo plano
	fmt.Printf("Reporte abierto exitosamente\n")
	return nil
}
