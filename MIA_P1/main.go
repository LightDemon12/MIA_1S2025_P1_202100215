package main

import (
	"MIA_P1/backend/routes"
	"log"
	"os/exec"
	"time"
)

func openBrowser(url string) error {
	return exec.Command("xdg-open", url).Start()
}

func startFrontend() error {
	cmd := exec.Command("npm", "run", "dev")
	cmd.Dir = "./frontend" // Establecer el directorio de trabajo
	return cmd.Start()
}

func main() {
	// Iniciar el frontend
	go func() {
		if err := startFrontend(); err != nil {
			log.Printf("Error al iniciar el frontend: %v", err)
		}
	}()

	r := routes.SetupRouter()

	// Abrir el navegador después de un pequeño delay
	go func() {
		time.Sleep(1500 * time.Millisecond) // Aumentamos el delay para dar tiempo al frontend
		if err := openBrowser("http://localhost:1921"); err != nil {
			log.Printf("Error al abrir el navegador: %v", err)
		}
	}()

	log.Printf("Servidor iniciado en http://localhost:1921")
	if err := r.Run(":1921"); err != nil {
		log.Fatal("Error al iniciar el servidor:", err)
	}
}
