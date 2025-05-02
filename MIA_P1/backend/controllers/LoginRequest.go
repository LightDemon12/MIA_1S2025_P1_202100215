package controllers

import (
	"MIA_P1/backend/analizador"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

// LoginRequest estructura para recibir las credenciales desde el frontend
type LoginRequest struct {
	User string `json:"user" binding:"required"`
	Pass string `json:"pass" binding:"required"`
	ID   string `json:"id" binding:"required"`
}

// Login maneja la autenticación de usuario mediante API REST
func Login(c *gin.Context) {
	var loginReq LoginRequest

	// Parsear el JSON del body
	if err := c.ShouldBindJSON(&loginReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"mensaje": "Error en los datos proporcionados. Se requieren user, pass e id.",
			"exito":   false,
		})
		return
	}

	// Convertir los parámetros al formato de comando esperado por el analizador
	comando := fmt.Sprintf("login -user=%s -pass=%s -id=%s", loginReq.User, loginReq.Pass, loginReq.ID)

	// Llamar a la función existente de login
	analizador.HandleLogin(c, comando)
}

// GetCurrentSession devuelve información sobre la sesión actual
func GetCurrentSession(c *gin.Context) {
	if analizador.CurrentSession == nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": "No hay sesión activa",
			"activa":  false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"mensaje":     "Sesión activa",
		"activa":      true,
		"usuario":     analizador.CurrentSession.Username,
		"particionID": analizador.CurrentSession.PartitionID,
		"esAdmin":     analizador.CurrentSession.IsAdmin,
		"grupo":       analizador.CurrentSession.UserGroup,
	})
}

// Logout cierra la sesión activa
func Logout(c *gin.Context) {
	if analizador.CurrentSession == nil {
		c.JSON(http.StatusOK, gin.H{
			"mensaje": "No hay sesión activa que cerrar",
			"exito":   false,
		})
		return
	}

	username := analizador.CurrentSession.Username
	analizador.CurrentSession = nil

	c.JSON(http.StatusOK, gin.H{
		"mensaje": fmt.Sprintf("Sesión de %s cerrada correctamente", username),
		"exito":   true,
	})
}
