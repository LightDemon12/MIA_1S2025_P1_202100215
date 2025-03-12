package controllers

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func HomeController(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "MIA Proyecto 1",
	})
}
