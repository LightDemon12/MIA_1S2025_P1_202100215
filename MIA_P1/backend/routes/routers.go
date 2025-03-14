package routes

import (
	"MIA_P1/backend/analizador"
	"MIA_P1/backend/controllers"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	gin.SetMode(gin.DebugMode)
	r := gin.Default()

	// Configuración de CORS
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://localhost:8080"} // Puerto del frontend
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept"}
	r.Use(cors.New(config))

	// Solo configurar el directorio de templates
	r.LoadHTMLGlob("backend/templates/*")

	// Ruta principal que renderiza index.html
	r.GET("/", controllers.HomeController)
	// Ruta para el analizador
	r.POST("/analizar", analizador.AnalizarComando)
	r.POST("/crear-directorio", analizador.CrearDirectorio)

	return r
}
