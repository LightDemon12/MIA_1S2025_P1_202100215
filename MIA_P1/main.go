package main

import (
	"MIA_P1/backend/routes"
	"log"
)

func main() {
	r := routes.SetupRouter()

	log.Printf("Servidor iniciado en http://localhost:1921")
	if err := r.Run(":1921"); err != nil {
		log.Fatal("Error al iniciar el servidor:", err)
	}
}
