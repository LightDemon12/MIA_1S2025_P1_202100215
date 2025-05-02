// URL base del backend, tomada desde navbar.js
// Asegúrate que API_URL esté definida en navbar.js y no aquí

document.addEventListener('DOMContentLoaded', function() {
    // Verificar si hay sesión activa
    checkAuth();

    // Cargar datos de discos
    loadDisks();

    // Iniciar efectos visuales
    setupTerminalEffects();
});

// Verificar autenticación
function checkAuth() {
    fetch(`${API_URL}/api/session`)
        .then(response => response.json())
        .then(data => {
            if (!data.activa) {
                window.location.href = '/login';
                return;
            }
            // Usuario autenticado, continuar
        })
        .catch(error => {
            console.error('Error verificando sesión:', error);
            showError('Error de conexión con el servidor');
        });
}

// Cargar datos de discos
function loadDisks() {
    // Mostrar indicador de carga
    document.getElementById('loading-indicator').style.display = 'flex';
    document.getElementById('disks-grid').innerHTML = '';

    fetch(`${API_URL}/api/disks`)
        .then(response => {
            if (!response.ok) {
                throw new Error(`Error ${response.status}: ${response.statusText}`);
            }
            return response.json();
        })
        .then(data => {
            // Ocultar indicador de carga
            document.getElementById('loading-indicator').style.display = 'none';

            // Actualizar estadísticas
            updateStatistics(data);

            // Renderizar discos
            renderDisks(data.discos);
        })
        .catch(error => {
            document.getElementById('loading-indicator').style.display = 'none';
            console.error('Error cargando discos:', error);
            showError('Error cargando discos: ' + error.message);
        });
}

// Actualizar estadísticas
function updateStatistics(data) {
    const diskCount = data.discos.length;
    document.getElementById('disk-count').textContent = diskCount;

    // Calcular almacenamiento total
    let totalStorage = 0;
    data.discos.forEach(disk => {
        const size = disk.size;
        const unit = disk.unit;

        // Convertir a MB para unificar
        if (unit === 'G') {
            totalStorage += size * 1024;
        } else if (unit === 'K') {
            totalStorage += size / 1024;
        } else { // Asumir MB
            totalStorage += size;
        }
    });

    document.getElementById('storage-total').textContent =
        totalStorage >= 1024 ?
            `${(totalStorage / 1024).toFixed(2)} GB` :
            `${totalStorage.toFixed(2)} MB`;

    // Por ahora no tenemos el conteo de particiones, lo podemos obtener después
    // Se actualizará cuando tengamos esa información
    fetchPartitionCount();
}

// Obtener conteo de particiones
function fetchPartitionCount() {
    fetch(`${API_URL}/api/partitions`)
        .then(response => response.json())
        .then(data => {
            const partitionCount = data.partitions?.length || 0;
            document.getElementById('partition-count').textContent = partitionCount;
        })
        .catch(error => {
            console.error('Error obteniendo particiones:', error);
            document.getElementById('partition-count').textContent = "Error";
        });
}

// Renderizar discos con iconos originales
function renderDisks(disks) {
    const disksGrid = document.getElementById('disks-grid');
    const template = document.getElementById('disk-card-template');

    // Limpiar contenedor
    disksGrid.innerHTML = '';

    if (disks.length === 0) {
        const noDisks = document.createElement('div');
        noDisks.className = 'no-data-message';
        noDisks.innerHTML = `
            <i class="fas fa-exclamation-triangle"></i>
            <p>No se encontraron discos en el sistema.</p>
        `;
        disksGrid.appendChild(noDisks);
        return;
    }

    disks.forEach(disk => {
        const diskCard = template.content.cloneNode(true);

        // Llenar datos
        diskCard.querySelector('.disk-name').textContent = disk.name;
        diskCard.querySelector('.disk-size').textContent = `${disk.size} ${disk.unit}B`;

        // Formatear fecha
        const diskDate = new Date(disk.createdAt);
        const formattedDate = diskDate.toLocaleDateString('es-ES', {
            day: '2-digit',
            month: '2-digit',
            year: 'numeric'
        });
        diskCard.querySelector('.disk-date').textContent = formattedDate;

        // Mantener los iconos originales pero ajustar el espacio
        const diskIcon = diskCard.querySelector('.disk-icon');
        diskIcon.style.width = '60px';

        // Agregar event listeners
        const viewBtn = diskCard.querySelector('.btn-view');
        viewBtn.addEventListener('click', () => {
            viewDiskDetails(disk);
        });

        const browseBtn = diskCard.querySelector('.btn-browse');
        browseBtn.addEventListener('click', () => {
            browseDiskPartitions(disk);
        });

        // Agregar al contenedor
        disksGrid.appendChild(diskCard);

        // Agregar efecto de aparición con retardo
        const cards = disksGrid.querySelectorAll('.disk-card');
        cards.forEach((card, index) => {
            setTimeout(() => {
                card.classList.add('appear');
            }, index * 100);
        });
    });
}

// Ver detalles de un disco
function viewDiskDetails(disk) {
    // Mostrar un indicador de carga mientras se obtienen los detalles
    showLoading(true, "Cargando detalles...");

    fetch(`${API_URL}/api/disk/analysis?path=${encodeURIComponent(disk.path)}`)
        .then(response => {
            if (!response.ok) {
                throw new Error(`Error ${response.status}: ${response.statusText}`);
            }
            return response.json();
        })
        .then(data => {
            showLoading(false);
            showDiskDetailsModal(disk, data);
        })
        .catch(error => {
            showLoading(false);
            console.error('Error obteniendo detalles del disco:', error);
            showError('No se pudieron cargar los detalles del disco');
        });
}

// Explorar particiones de un disco (redirección a nueva página)
function browseDiskPartitions(disk) {
    // Guardar información relevante del disco en sessionStorage
    sessionStorage.setItem('currentDisk', JSON.stringify({
        name: disk.name,
        path: disk.path,
        size: disk.size,
        unit: disk.unit,
        createdAt: disk.createdAt
    }));

    // Usar ruta sin extensión para mayor compatibilidad
    window.location.href = `/partitions?disk=${encodeURIComponent(disk.path)}`;
}

/// Mostrar modal de detalles de disco - corregida la sintaxis
function showDiskDetailsModal(disk, details) { // Quitada la 's' extra aquí
    // Crear modal
    const modal = document.createElement('div');
    modal.className = 'terminal-modal';

    // HTML para el modal - solo con detalles básicos
    modal.innerHTML = `
        <div class="modal-content">
            <div class="modal-header">
                <h2><i class="fas fa-hdd"></i> DETALLES DE DISCO: ${disk.name}</h2>
                <button class="close-btn"><i class="fas fa-times"></i></button>
            </div>
            <div class="modal-body">
                <div class="detail-item">
                    <span class="detail-label">NOMBRE:</span>
                    <span class="detail-value">${disk.name}</span>
                </div>
                <div class="detail-item">
                    <span class="detail-label">TAMAÑO:</span>
                    <span class="detail-value">${disk.size} ${disk.unit}B</span>
                </div>
                <div class="detail-item">
                    <span class="detail-label">CREADO:</span>
                    <span class="detail-value">${new Date(disk.createdAt).toLocaleString()}</span>
                </div>
                <div class="detail-item">
                    <span class="detail-label">UBICACIÓN:</span>
                    <span class="detail-value">${disk.path}</span>
                </div>
            </div>
        </div>
    `;

    // Agregar al DOM
    document.body.appendChild(modal);

    // Agregar event listener para cerrar
    modal.querySelector('.close-btn').addEventListener('click', () => {
        modal.classList.add('fade-out');
        setTimeout(() => {
            modal.remove();
        }, 300);
    });

    // Efecto de aparición
    setTimeout(() => {
        modal.classList.add('active');
    }, 10);
}

// Configurar efectos visuales de terminal
function setupTerminalEffects() {
    // Efecto de scan line
    const terminalContainer = document.querySelector('.terminal-container');
    const scanline = document.createElement('div');
    scanline.className = 'scanline';
    terminalContainer.appendChild(scanline);

    // Efecto de parpadeo aleatorio
    setInterval(() => {
        const disksGrid = document.getElementById('disks-grid');
        if (disksGrid) {
            disksGrid.classList.add('flicker');
            setTimeout(() => {
                disksGrid.classList.remove('flicker');
            }, 100);
        }
    }, Math.random() * 10000 + 5000);
}

// Función auxiliar para mostrar/ocultar indicador de carga
function showLoading(show, message = "CARGANDO DATOS DEL SISTEMA") {
    const loadingIndicator = document.getElementById('loading-indicator');

    if (show) {
        loadingIndicator.querySelector('.loading-text').textContent = message;
        loadingIndicator.style.display = 'flex';
    } else {
        loadingIndicator.style.display = 'none';
    }
}

// Asegúrate de que el error visual sea evidente
function showError(message) {
    const errorBox = document.createElement('div');
    errorBox.className = 'error-message';
    errorBox.innerHTML = `
        <i class="fas fa-exclamation-circle"></i>
        <span>${message}</span>
        <button class="error-close"><i class="fas fa-times"></i></button>
    `;

    document.body.appendChild(errorBox);

    setTimeout(() => {
        errorBox.classList.add('visible');
    }, 10);

    setTimeout(() => {
        errorBox.classList.remove('visible');
        setTimeout(() => {
            errorBox.remove();
        }, 300);
    }, 5000);

    errorBox.querySelector('.error-close').addEventListener('click', () => {
        errorBox.classList.remove('visible');
        setTimeout(() => {
            errorBox.remove();
        }, 300);
    });
}