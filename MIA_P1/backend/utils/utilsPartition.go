package utils

import (
	"fmt"
)

type PartitionConfig struct {
	Size int    // Tamaño de la partición
	Path string // Ruta del disco
	Name string // Nombre de la partición
	Type string // Tipo de partición (P, E, L)
	Fit  string // Tipo de ajuste (BF, FF, WF)
	Unit string // Unidad de medida (B, K, M)
}

func NewPartitionConfig() PartitionConfig {
	return PartitionConfig{
		Type: "P",  // Default: Primaria
		Fit:  "FF", // Default: First Fit
		Unit: "K",  // Default: Kilobytes
	}
}

// Estructura para almacenar información de particiones montadas
// MountedPartition representa una partición montada en el sistema
type MountedPartition struct {
	ID            string
	DiskPath      string
	PartitionName string
	PartitionType byte // 'P' para primaria, 'L' para lógica
	Status        byte // Estado de la partición (montada o no)
	Letter        byte // Letra asignada (A, B, C, etc.)
	Number        int  // Número de partición
}

// Variable global para almacenar las particiones montadas
var MountedPartitions []MountedPartition

// GenerateID genera un ID según el formato especificado
func GenerateID(carnet string, number int, letter byte) string {
	// Obtener los últimos dos dígitos del carnet
	lastTwoDigits := "00"
	if len(carnet) >= 2 {
		lastTwoDigits = carnet[len(carnet)-2:]
	}

	return fmt.Sprintf("%s%d%c", lastTwoDigits, number, letter)
}

// GetDiskLetter obtiene la letra asignada a un disco específico
// o asigna una nueva letra si el disco no tiene particiones montadas
func GetDiskLetter(diskPath string) byte {
	// Mapa para asociar discos con sus letras
	diskLetters := make(map[string]byte)

	// Recorrer todas las particiones montadas para encontrar qué letras ya están asignadas a cada disco
	for _, mp := range MountedPartitions {
		diskLetters[mp.DiskPath] = mp.Letter
	}

	// Si el disco ya tiene una letra asignada, retornarla
	if letter, exists := diskLetters[diskPath]; exists {
		fmt.Printf("Debug: Disco %s ya tiene letra asignada: %c\n", diskPath, letter)
		return letter
	}

	// Si el disco no tiene letra, asignar la siguiente disponible en el alfabeto
	usedLetters := make(map[byte]bool)
	for _, letter := range diskLetters {
		usedLetters[letter] = true
	}

	// Buscar la primera letra disponible
	for letter := byte('A'); letter <= byte('Z'); letter++ {
		if !usedLetters[letter] {
			fmt.Printf("Debug: Asignando nueva letra %c a disco %s\n", letter, diskPath)
			return letter
		}
	}

	// Si todas las letras están usadas, volver a A (no debería ocurrir en la práctica)
	return 'A'
}

// GetNextPartitionNumber obtiene el siguiente número de partición para un disco específico
func GetNextPartitionNumber(diskPath string, letter byte) int {
	maxNumber := 0

	// Buscar el número más alto para este disco y esta letra
	for _, mp := range MountedPartitions {
		if mp.DiskPath == diskPath && mp.Letter == letter {
			if mp.Number > maxNumber {
				maxNumber = mp.Number
			}
		}
	}

	// Incrementar el número en 1
	nextNumber := maxNumber + 1
	fmt.Printf("Debug: Siguiente número para disco %s, letra %c: %d\n", diskPath, letter, nextNumber)
	return nextNumber
}

func GetNextLetter(diskPath string) byte {
	// Mapa para asociar discos con sus letras
	diskLetters := make(map[string]byte)

	// Recorrer todas las particiones montadas para encontrar qué letras ya están asignadas a cada disco
	for _, mp := range MountedPartitions {
		diskLetters[mp.DiskPath] = mp.Letter
	}

	// Si el disco ya tiene una letra asignada, retornarla
	if letter, exists := diskLetters[diskPath]; exists {
		fmt.Printf("Debug: Disco %s ya tiene letra asignada: %c\n", diskPath, letter)
		return letter
	}

	// Si el disco no tiene letra, asignar la siguiente disponible en el alfabeto
	usedLetters := make(map[byte]bool)
	for _, letter := range diskLetters {
		usedLetters[letter] = true
	}

	// Buscar la primera letra disponible
	for letter := byte('A'); letter <= byte('Z'); letter++ {
		if !usedLetters[letter] {
			fmt.Printf("Debug: Asignando nueva letra %c a disco %s\n", letter, diskPath)
			return letter
		}
	}

	// Si todas las letras están usadas, volver a A (no debería ocurrir en la práctica)
	return 'A'
}
