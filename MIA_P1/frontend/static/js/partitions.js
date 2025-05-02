document.addEventListener('DOMContentLoaded', function() {
    // Verificar si hay sesión activa
    checkAuth();

    // Inicializar la página
    setupPage();

    // Cargar datos de particiones
    loadPartitions();

    // Iniciar efectos visuales
    setupTerminalEffects();
});

// Verificar autenticación (mismo que en otras páginas)
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

// Configurar elementos de la página
function setupPage() {
    // Obtener información del disco de sessionStorage
    const diskInfoStr = sessionStorage.getItem('currentDisk');
    let diskInfo;

    try {
        diskInfo = JSON.parse(diskInfoStr);
        if (!diskInfo) throw new Error("Información de disco no encontrada");
    } catch (e) {
        console.error("Error obteniendo información del disco:", e);
        showError("No se pudo cargar la información del disco");
        return;
    }

    // Actualizar título de la página
    document.title = `Particiones: ${diskInfo.name} - Sistema de Archivos`;

    // Actualizar breadcrumb
    document.getElementById('current-disk-name').innerHTML = `
        <i class="fas fa-hdd"></i> ${diskInfo.name}
    `;

    // Configurar botón de volver
    document.getElementById('back-to-disks').addEventListener('click', () => {
        window.location.href = '/explorer.html';
    });
}

// Cargar particiones del disco
function loadPartitions() {
    // Obtener path del disco de la URL
    const urlParams = new URLSearchParams(window.location.search);
    const diskPath = urlParams.get('disk');

    if (!diskPath) {
        showError('No se especificó ningún disco');
        return;
    }

    // Mostrar indicador de carga
    document.getElementById('loading-indicator').style.display = 'flex';
    document.getElementById('partitions-grid').innerHTML = '';

    // Obtener datos de la API
    fetch(`${API_URL}/api/disk/partitions?disk=${encodeURIComponent(diskPath)}`)
        .then(response => {
            if (!response.ok) {
                throw new Error(`Error ${response.status}: ${response.statusText}`);
            }
            return response.json();
        })
        .then(data => {
            // Ocultar indicador de carga
            document.getElementById('loading-indicator').style.display = 'none';

            // Verificar éxito
            if (!data.exito) {
                showError(`Error: ${data.mensaje}`);
                return;
            }

            // Actualizar estadísticas
            updateStatistics(data.particiones);

            // Renderizar particiones
            renderPartitions(data.particiones);
        })
        .catch(error => {
            document.getElementById('loading-indicator').style.display = 'none';
            console.error('Error cargando particiones:', error);
            showError('Error cargando particiones: ' + error.message);
        });
}

// Actualizar estadísticas
function updateStatistics(partitions) {
    // Contar particiones
    document.getElementById('partition-count').textContent = partitions.length;

    // Contar particiones montadas
    const mountedCount = partitions.filter(p => p.mounted).length;
    document.getElementById('mounted-count').textContent = mountedCount;

    // Calcular almacenamiento
    let totalSize = 0;
    let usedSize = 0;

    partitions.forEach(partition => {
        totalSize += partition.size || 0;
        usedSize += partition.used || 0;
    });

    // Formatear para mostrar
    const formattedTotal = formatSize(totalSize);
    const formattedUsed = formatSize(usedSize);
    const percentUsed = totalSize > 0 ? Math.round((usedSize / totalSize) * 100) : 0;

    document.getElementById('storage-info').innerHTML = `
        ${formattedUsed} / ${formattedTotal} <br>
        <small>${percentUsed}% usado</small>
    `;
}

// Formatear tamaño
function formatSize(bytes) {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1073741824) return `${(bytes / 1048576).toFixed(1)} MB`;
    return `${(bytes / 1073741824).toFixed(1)} GB`;
}

// Función auxiliar para interpretar el fit desde código ASCII
function interpretFit(fitCode) {
    // Si es un número, asumimos que es un código ASCII
    if (typeof fitCode === 'number') {
        // 66 = 'B' (Best Fit)
        if (fitCode === 66) return "Best Fit (B)";
        // 70 = 'F' (First Fit)
        if (fitCode === 70) return "First Fit (F)";
        // 87 = 'W' (Worst Fit)
        if (fitCode === 87) return "Worst Fit (W)";

        // Si no coincide con ninguno conocido, mostrar el carácter
        return `Fit: ${String.fromCharCode(fitCode)}`;
    }

    // Si ya es un string, devolver directamente
    if (typeof fitCode === 'string') {
        return `Fit: ${fitCode}`;
    }

    return "Fit desconocido";
}
// Función auxiliar para interpretar el fit desde código ASCII
function interpretFit(fitCode) {
    // Si es un número, asumimos que es un código ASCII
    if (typeof fitCode === 'number') {
        // 66 = 'B' (Best Fit)
        if (fitCode === 66) return "Best Fit (B)";
        // 70 = 'F' (First Fit)
        if (fitCode === 70) return "First Fit (F)";
        // 87 = 'W' (Worst Fit)
        if (fitCode === 87) return "Worst Fit (W)";

        // Si no coincide con ninguno conocido, mostrar el carácter
        return `Fit: ${String.fromCharCode(fitCode)}`;
    }

    // Si ya es un string, devolver directamente
    if (typeof fitCode === 'string') {
        return `Fit: ${fitCode}`;
    }

    return "Fit desconocido";
}

// Renderizar particiones
function renderPartitions(partitions) {
    const partitionsGrid = document.getElementById('partitions-grid');
    const template = document.getElementById('partition-card-template');

    // Limpiar contenedor
    partitionsGrid.innerHTML = '';

    if (partitions.length === 0) {
        const noPartitions = document.createElement('div');
        noPartitions.className = 'no-data-message';
        noPartitions.innerHTML = `
            <i class="fas fa-exclamation-triangle"></i>
            <p>No se encontraron particiones en este disco.</p>
        `;
        partitionsGrid.appendChild(noPartitions);
        return;
    }

    // Renderizar cada partición
    partitions.forEach((partition, index) => {
        const partitionCard = template.content.cloneNode(true);

        // Determinar si está montada (unificando propiedades)
        const isMounted = partition.isMounted || partition.mounted;

        // Usar typeName del JSON
        const partitionType = partition.typeName || 'Desconocido';
        const typeLower = partitionType.toLowerCase();

        // Seleccionar icono según tipo
        const iconElement = partitionCard.querySelector('.partition-icon');
        iconElement.classList.add(typeLower.replace(/[^a-z]/g, ''));

        // Ícono según tipo de partición
        let iconClass = 'fa-hdd'; // Default

        if (typeLower.includes('lóg')) iconClass = 'fa-folder';
        else if (typeLower.includes('ext')) iconClass = 'fa-object-group';

        iconElement.querySelector('i').className = `fas ${iconClass}`;

        // Nombre de partición
        partitionCard.querySelector('.partition-name').textContent =
            partition.name || `Partición ${index + 1}`;

        // Detalles de la partición
        partitionCard.querySelector('.partition-size').textContent = formatSize(partition.size || 0);

        const typeElement = partitionCard.querySelector('.partition-type');
        typeElement.textContent = partitionType;
        typeElement.classList.add(typeLower.replace(/[^a-z]/g, ''));

        // Estado de montaje
        const statusIndicator = partitionCard.querySelector('.status-indicator');
        const statusText = partitionCard.querySelector('.status-text');

        if (isMounted) {
            statusIndicator.classList.add('mounted');
            statusText.classList.add('mounted');
            statusText.textContent = `Montada (${partition.mountId || '-'})`;
        } else {
            statusText.textContent = 'Desmontada';
        }

        // Mostrar el fit si está disponible
        if (partition.fit !== undefined) {
            const fitInfo = document.createElement('div');
            fitInfo.className = 'partition-fit';

            // Interpretar el código ASCII del fit
            const fitText = interpretFit(partition.fit);

            fitInfo.innerHTML = `<span class="fit-value">${fitText}</span>`;

            // Agregar al contenedor adecuado
            const detailsContainer = partitionCard.querySelector('.partition-details');
            detailsContainer.appendChild(fitInfo);
        }

        // Solo botón de explorar - si está montada
        const exploreBtn = partitionCard.querySelector('.btn-explore');
        if (isMounted) {
            exploreBtn.addEventListener('click', () => {
                explorePartition(partition);
            });
        } else {
            // Ocultar botón si la partición no está montada
            exploreBtn.style.display = 'none';
        }

        // Agregar al contenedor
        partitionsGrid.appendChild(partitionCard);
    });

    // Agregar efecto de aparición con retardo
    const cards = partitionsGrid.querySelectorAll('.partition-card');
    cards.forEach((card, index) => {
        setTimeout(() => {
            card.classList.add('appear');
        }, index * 100);
    });
}
// Actualizar estadísticas para usar isMounted
function updateStatistics(partitions) {
    // Contar particiones
    document.getElementById('partition-count').textContent = partitions.length;

    // Contar particiones montadas
    const mountedCount = partitions.filter(p => p.isMounted).length;
    document.getElementById('mounted-count').textContent = mountedCount;

    // Calcular almacenamiento
    let totalSize = 0;
    let usedSize = 0;

    partitions.forEach(partition => {
        totalSize += partition.size || 0;
        usedSize += partition.used || 0;
    });

    // Formatear para mostrar
    const formattedTotal = formatSize(totalSize);
    const formattedUsed = formatSize(usedSize);
    const percentUsed = totalSize > 0 ? Math.round((usedSize / totalSize) * 100) : 0;

    document.getElementById('storage-info').innerHTML = `
        ${formattedUsed} / ${formattedTotal} <br>
        <small>${percentUsed}% usado</small>
    `;
}

// Función toggleMount actualizada
function toggleMount(partition) {
    // Mostrar indicador de carga
    document.getElementById('loading-indicator').style.display = 'flex';

    const action = partition.isMounted ? 'unmount' : 'mount';

    fetch(`${API_URL}/api/partition/${action}`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            path: partition.diskPath,
            name: partition.name,
            index: partition.index
        })
    })
        .then(response => response.json())
        .then(data => {
            document.getElementById('loading-indicator').style.display = 'none';

            if (!data.exito) {
                showError(`Error: ${data.mensaje}`);
                return;
            }

            // Recargar particiones para reflejar cambios
            loadPartitions();

            // Mostrar mensaje de éxito
            const actionName = partition.isMounted ? 'desmontada' : 'montada';
            showSuccess(`Partición ${actionName} correctamente`);
        })
        .catch(error => {
            document.getElementById('loading-indicator').style.display = 'none';
            console.error(`Error ${action} partición:`, error);
            showError(`Error: No se pudo ${action === 'mount' ? 'montar' : 'desmontar'} la partición`);
        });
}
// Montar/Desmontar partición
function toggleMount(partition) {
    // Mostrar indicador de carga
    document.getElementById('loading-indicator').style.display = 'flex';

    const action = partition.mounted ? 'unmount' : 'mount';

    fetch(`${API_URL}/api/partition/${action}`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            path: partition.path,
            name: partition.name,
            index: partition.index
        })
    })
        .then(response => response.json())
        .then(data => {
            document.getElementById('loading-indicator').style.display = 'none';

            if (!data.exito) {
                showError(`Error: ${data.mensaje}`);
                return;
            }

            // Recargar particiones para reflejar cambios
            loadPartitions();

            // Mostrar mensaje de éxito
            const actionName = partition.mounted ? 'desmontada' : 'montada';
            showSuccess(`Partición ${actionName} correctamente`);
        })
        .catch(error => {
            document.getElementById('loading-indicator').style.display = 'none';
            console.error(`Error ${action} partición:`, error);
            showError(`Error: No se pudo ${action === 'mount' ? 'montar' : 'desmontar'} la partición`);
        });
}

// Explorar partición
function explorePartition(partition) {
    // Redireccionar a la página de exploración de archivos
    sessionStorage.setItem('currentPartition', JSON.stringify(partition));
    window.location.href = `/files?id=${encodeURIComponent(partition.mountId)}`;
}
// Mostrar error
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

// Mostrar mensaje de éxito
function showSuccess(message) {
    const successBox = document.createElement('div');
    successBox.className = 'success-message';
    successBox.innerHTML = `
        <i class="fas fa-check-circle"></i>
        <span>${message}</span>
        <button class="success-close"><i class="fas fa-times"></i></button>
    `;

    document.body.appendChild(successBox);

    setTimeout(() => {
        successBox.classList.add('visible');
    }, 10);

    setTimeout(() => {
        successBox.classList.remove('visible');
        setTimeout(() => {
            successBox.remove();
        }, 300);
    }, 5000);

    successBox.querySelector('.success-close').addEventListener('click', () => {
        successBox.classList.remove('visible');
        setTimeout(() => {
            successBox.remove();
        }, 300);
    });
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
        const partitionsGrid = document.getElementById('partitions-grid');
        if (partitionsGrid) {
            partitionsGrid.classList.add('flicker');
            setTimeout(() => {
                partitionsGrid.classList.remove('flicker');
            }, 100);
        }
    }, Math.random() * 10000 + 5000);
}