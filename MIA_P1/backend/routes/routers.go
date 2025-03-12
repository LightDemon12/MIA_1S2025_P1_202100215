package routes

import (
	"MIA_P1/backend/controllers"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	gin.SetMode(gin.DebugMode)
	r := gin.Default()

	// Configuración básica de CORS
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://localhost:1921"}
	r.Use(cors.New(config))

	// Solo configurar el directorio de templates
	r.LoadHTMLGlob("backend/templates/*")

	// Ruta principal que renderiza index.html
	r.GET("/", controllers.HomeController)

	return r
}
