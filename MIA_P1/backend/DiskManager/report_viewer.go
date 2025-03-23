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

// OpenTextFile abre un archivo de texto usando el editor de texto predeterminado del sistema
func OpenTextFile(txtFilePath string) error {
	// Verificar que el archivo exista
	if _, err := os.Stat(txtFilePath); os.IsNotExist(err) {
		return fmt.Errorf("el archivo de texto no existe: %s", txtFilePath)
	}

	fmt.Printf("Abriendo archivo de texto: %s\n", txtFilePath)

	var cmd *exec.Cmd

	// Detectar el sistema operativo y usar el comando apropiado
	switch runtime.GOOS {
	case "linux":
		// En Linux, intentar abrir con el editor predeterminado
		if _, err := exec.LookPath("xed"); err == nil {
			cmd = exec.Command("xed", txtFilePath)
		} else if _, err := exec.LookPath("gedit"); err == nil {
			cmd = exec.Command("gedit", txtFilePath)
		} else if _, err := exec.LookPath("kate"); err == nil {
			cmd = exec.Command("kate", txtFilePath)
		} else if _, err := exec.LookPath("nano"); err == nil {
			cmd = exec.Command("xterm", "-e", "nano", txtFilePath)
		} else if _, err := exec.LookPath("vim"); err == nil {
			cmd = exec.Command("xterm", "-e", "vim", txtFilePath)
		} else {
			// Si no se encuentra ningún editor específico, usar xdg-open
			cmd = exec.Command("xdg-open", txtFilePath)
		}
	case "windows":
		// En Windows, notepad es el editor predeterminado
		cmd = exec.Command("notepad", txtFilePath)
	case "darwin": // macOS
		// En macOS, open -t abre con el editor de texto predeterminado
		cmd = exec.Command("open", "-t", txtFilePath)
	default:
		return fmt.Errorf("sistema operativo no soportado: %s", runtime.GOOS)
	}

	// Ejecutar el comando en segundo plano
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error al abrir el archivo de texto: %v", err)
	}

	// No esperamos a que termine, lo dejamos correr en segundo plano
	fmt.Printf("Archivo de texto abierto exitosamente\n")
	return nil
}
