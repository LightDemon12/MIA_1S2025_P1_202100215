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
	r.POST("/ext2-crear-directorios", analizador.HandleEXT2CreateDirectories)
	// Nueva ruta para obtener listado de discos fase 2
	r.GET("/api/disks", controllers.GetAllDisks)
	r.GET("/api/disks/analysis", controllers.GetAllDiskAnalysis)
	r.GET("/api/disk/analysis", controllers.GetDiskAnalysis)
	r.GET("/api/partitions", controllers.GetAllPartitionsInfo)
	r.GET("/api/disk/partitions", controllers.GetDiskPartitionsInfo)
	r.GET("/api/partition", controllers.GetPartitionInfo)
	r.GET("/api/filesystem", controllers.GetFileSystem) // Obtener toda la estructura
	r.GET("/api/file", controllers.GetFileContent)      // Obtener contenido de un archivo específico
	r.GET("/api/directory", controllers.ListDirectory)  // Listar contenido de un directorio
	r.POST("/api/login", controllers.Login)
	r.GET("/api/session", controllers.GetCurrentSession)
	r.POST("/api/logout", controllers.Logout)
	return r
}
