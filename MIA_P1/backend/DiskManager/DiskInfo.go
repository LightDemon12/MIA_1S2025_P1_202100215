package DiskManager

import (
	"sync"
	"time"
)

// DiskInfo estructura para almacenar información del disco
type DiskInfo struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Size      int       `json:"size"`
	Unit      string    `json:"unit"`
	CreatedAt time.Time `json:"createdAt"`
	Fit       byte      `json:"fit"`
}

var (
	disks     = make(map[string]DiskInfo)
	diskMutex = &sync.RWMutex{}
)

// RegisterDisk registra un nuevo disco en la lista de discos
func RegisterDisk(disk DiskInfo) {
	diskMutex.Lock()
	defer diskMutex.Unlock()
	disk.CreatedAt = time.Now()
	disks[disk.Path] = disk
}

// GetAllDisks retorna todos los discos registrados
func GetAllDisks() []DiskInfo {
	diskMutex.RLock()
	defer diskMutex.RUnlock()

	result := make([]DiskInfo, 0, len(disks))
	for _, disk := range disks {
		result = append(result, disk)
	}
	return result
}

// GetDisk obtiene la información de un disco por su path
func GetDisk(path string) (DiskInfo, bool) {
	diskMutex.RLock()
	defer diskMutex.RUnlock()
	disk, exists := disks[path]
	return disk, exists
}

// DiskExists verifica si un disco ya existe en el registro
func DiskExists(path string) bool {
	diskMutex.RLock()
	defer diskMutex.RUnlock()
	_, exists := disks[path]
	return exists
}

// RemoveDisk elimina un disco del registro en memoria
func RemoveDisk(path string) bool {
	diskMutex.Lock()
	defer diskMutex.Unlock()

	_, exists := disks[path]
	if exists {
		delete(disks, path)
		return true
	}
	return false
}
