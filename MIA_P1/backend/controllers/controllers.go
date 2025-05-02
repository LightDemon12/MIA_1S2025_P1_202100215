package controllers

import (
	"MIA_P1/backend/DiskManager"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

func HomeController(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "MIA Proyecto 1",
	})
}

func GetAllDisks(c *gin.Context) {
	disks := DiskManager.GetAllDisks()
	c.JSON(http.StatusOK, gin.H{
		"discos": disks,
		"total":  len(disks),
	})
}

// GetAllDiskAnalysis obtiene el análisis completo de todos los discos registrados
func GetAllDiskAnalysis(c *gin.Context) {
	analyses, err := DiskManager.AnalyzeAllDisks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"mensaje": fmt.Sprintf("Error analizando discos: %v", err),
			"exito":   false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"analisis": analyses,
		"total":    len(analyses),
		"exito":    true,
	})
}

// GetDiskAnalysis obtiene el análisis completo de un disco específico
func GetDiskAnalysis(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"mensaje": "Se requiere el parámetro 'path'",
			"exito":   false,
		})
		return
	}

	analysis, err := DiskManager.AnalyzeDiskStructure(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"mensaje": fmt.Sprintf("Error analizando disco: %v", err),
			"exito":   false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"analisis": analysis,
		"exito":    true,
	})
}

// GetAllPartitionsInfo obtiene información básica de todas las particiones
func GetAllPartitionsInfo(c *gin.Context) {
	partitions, err := DiskManager.GetAllPartitionsInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"mensaje": fmt.Sprintf("Error obteniendo particiones: %v", err),
			"exito":   false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"particiones": partitions,
		"total":       len(partitions),
		"exito":       true,
	})
}

// GetDiskPartitionsInfo obtiene información básica de las particiones de un disco
func GetDiskPartitionsInfo(c *gin.Context) {
	diskPath := c.Query("disk")
	if diskPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"mensaje": "Se requiere el parámetro 'disk'",
			"exito":   false,
		})
		return
	}

	partitions, err := DiskManager.GetDiskPartitionsInfo(diskPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"mensaje": fmt.Sprintf("Error obteniendo particiones: %v", err),
			"exito":   false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"particiones": partitions,
		"total":       len(partitions),
		"diskPath":    diskPath,
		"exito":       true,
	})
}

// GetPartitionInfo obtiene información básica de una partición específica
func GetPartitionInfo(c *gin.Context) {
	diskPath := c.Query("disk")
	partName := c.Query("partition")

	if diskPath == "" || partName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"mensaje": "Se requieren los parámetros 'disk' y 'partition'",
			"exito":   false,
		})
		return
	}

	partition, err := DiskManager.GetPartitionInfoByName(diskPath, partName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"mensaje": fmt.Sprintf("Error obteniendo información de la partición: %v", err),
			"exito":   false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"particion": partition,
		"exito":     true,
	})
}
