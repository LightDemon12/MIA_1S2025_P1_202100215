document.addEventListener('DOMContentLoaded', function() {
    // Inicializar
    const API_URL = 'http://localhost:1921';
    let currentPath = '/';
    let currentPartitionId = '';
    let currentDiskPath = ''; // Añadir variable para almacenar el disco

    // Obtener la partición ID de la URL
    const urlParams = new URLSearchParams(window.location.search);

    // Verificar si se pasó el ID de partición
    if (urlParams.has('id')) {
        currentPartitionId = urlParams.get('id');
        init();
    } else {
        // Intentar recuperar desde sessionStorage
        const partitionInfo = JSON.parse(sessionStorage.getItem('currentPartition'));
        if (partitionInfo && partitionInfo.mountId) {
            currentPartitionId = partitionInfo.mountId;

            // Almacenar el path del disco si existe en la información de la partición
            if (partitionInfo.diskPath) {
                currentDiskPath = partitionInfo.diskPath;
            }

            init();
        } else {
            showError("No se especificó ninguna partición para explorar");
            setTimeout(() => {
                window.location.href = '/partitions';
            }, 2000);
        }
    }

    // Configurar el botón de volver
    document.getElementById('back-button').addEventListener('click', function() {
        if (currentPath === '/') {
            // Si estamos en la raíz, regresar a la página de particiones

            // Primero intentar obtener el diskPath desde sessionStorage si no lo tenemos
            if (!currentDiskPath) {
                const partitionInfo = JSON.parse(sessionStorage.getItem('currentPartition'));
                if (partitionInfo && partitionInfo.diskPath) {
                    currentDiskPath = partitionInfo.diskPath;
                }
            }

            // Redireccionar a la página de particiones con el parámetro del disco
            if (currentDiskPath) {
                window.location.href = `/partitions?disk=${encodeURIComponent(currentDiskPath)}`;
            } else {
                // Si no tenemos información del disco, intentar obtenerla del servidor
                fetch(`${API_URL}/api/partition/info?id=${encodeURIComponent(currentPartitionId)}`)
                    .then(response => response.json())
                    .then(data => {
                        if (data.exito && data.diskPath) {
                            window.location.href = `/partitions?disk=${encodeURIComponent(data.diskPath)}`;
                        } else {
                            // Si todo falla, simplemente ir a la lista de discos
                            window.location.href = '/disks';
                        }
                    })
                    .catch(error => {
                        console.error("Error obteniendo información de la partición:", error);
                        window.location.href = '/disks'; // Como fallback ir a la lista de discos
                    });
            }
        } else {
            // Navegar al directorio padre
            const pathParts = currentPath.split('/').filter(p => p);
            pathParts.pop(); // Remover el último segmento
            const parentPath = '/' + pathParts.join('/');
            navigateToDirectory(parentPath || '/');
        }
    });

    // Función de inicialización
    function init() {
        // Configurar scanline y efectos
        setupTerminalEffects();

        // Actualizar ID de partición en la UI
        document.getElementById('partition-id').textContent = currentPartitionId;

        // Intentar obtener la información completa de la partición si no tenemos diskPath
        if (!currentDiskPath) {
            fetch(`${API_URL}/api/partition/info?id=${encodeURIComponent(currentPartitionId)}`)
                .then(response => response.json())
                .then(data => {
                    if (data.exito && data.diskPath) {
                        currentDiskPath = data.diskPath;

                        // Actualizar la información en sessionStorage
                        const partitionInfo = JSON.parse(sessionStorage.getItem('currentPartition') || '{}');
                        partitionInfo.diskPath = currentDiskPath;
                        sessionStorage.setItem('currentPartition', JSON.stringify(partitionInfo));
                    }
                })
                .catch(error => {
                    console.error("Error obteniendo información de la partición:", error);
                });
        }

        // Cargar directorio raíz
        loadDirectory(currentPath);
    }
    // Cargar directorio
    function loadDirectory(path) {
        showLoading(true);

        // Actualizar la navegación
        updatePathNavigation(path);

        // Guardar el path actual
        currentPath = path;

        fetch(`${API_URL}/api/directory?id=${encodeURIComponent(currentPartitionId)}&path=${encodeURIComponent(path)}`)
            .then(response => {
                if (!response.ok) {
                    throw new Error(`Error ${response.status}: ${response.statusText}`);
                }
                return response.json();
            })
            .then(data => {
                showLoading(false);

                if (!data.exito) {
                    showError(`Error: ${data.mensaje}`);
                    return;
                }

                // Renderizar los archivos y directorios
                renderFilesList(data.contenido);

                // Actualizar estadísticas
                updateStatistics(data.contenido);
            })
            .catch(error => {
                showLoading(false);
                console.error("Error cargando directorio:", error);
                showError("Error cargando el directorio: " + error.message);
            });
    }

    function renderFilesList(files) {
        const filesList = document.getElementById('files-list');
        const noFilesMessage = document.getElementById('no-files-message');

        // Limpiar contenido actual
        filesList.innerHTML = '';

        if (!files || files.length === 0) {
            // Mostrar mensaje de directorio vacío
            noFilesMessage.style.display = 'block';
            return;
        }

        noFilesMessage.style.display = 'none';

        // Ordenar: primero directorios, luego archivos
        const sortedFiles = [...files].sort((a, b) => {
            // Prioridad a directorios
            if (a.type === 'directory' && b.type !== 'directory') return -1;
            if (a.type !== 'directory' && b.type === 'directory') return 1;
            // Luego ordenar por nombre
            return a.name.localeCompare(b.name);
        });

        // Renderizar cada archivo/directorio
        sortedFiles.forEach(file => {
            // Log de depuración para permisos
            console.log(`Permisos de ${file.name}:`, file.permissions);

            const row = document.createElement('tr');
            row.className = `file-row ${file.type}`;

            // Determinar ícono según tipo
            let iconClass = 'fa-file-alt';
            if (file.type === 'directory') {
                iconClass = 'fa-folder';
            } else if (file.name.endsWith('.sh') || (file.permissions && file.permissions.includes('x'))) {
                iconClass = 'fa-file-code';
                row.classList.add('executable');
            } else if (file.name.endsWith('.txt') || file.name.endsWith('.md')) {
                iconClass = 'fa-file-alt';
            } else if (file.name.endsWith('.jpg') || file.name.endsWith('.png')) {
                iconClass = 'fa-file-image';
            }

            // Formatear fecha
            const modifiedDate = new Date(file.modifiedAt);
            const formattedDate = modifiedDate.toLocaleDateString() + ' ' +
                modifiedDate.toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'});

            // Formatear permisos para visualización bonita usando la función mejorada
            const formattedPermissions = formatPermissions(file.permissions);

            // Crear las celdas de forma segura
            // Icono
            const iconCell = document.createElement('td');
            iconCell.innerHTML = `<span class="file-icon ${file.type}"><i class="fas ${iconClass}"></i></span>`;
            row.appendChild(iconCell);

            // Nombre
            const nameCell = document.createElement('td');
            nameCell.innerHTML = `<span class="file-name"><a data-path="${file.path}" data-type="${file.type}" class="file-link">${file.name}</a></span>`;
            row.appendChild(nameCell);

            // Tamaño
            const sizeCell = document.createElement('td');
            sizeCell.innerHTML = `<span class="file-size">${formatSize(file.size)}</span>`;
            row.appendChild(sizeCell);

            // Permisos - Asegurarnos de que se interpreta el HTML
            const permCell = document.createElement('td');
            const permSpan = document.createElement('span');
            permSpan.className = 'file-permissions';
            permSpan.innerHTML = formattedPermissions; // Usar innerHTML para interpretar las etiquetas
            permCell.appendChild(permSpan);
            row.appendChild(permCell);

            // Propietario
            const ownerCell = document.createElement('td');
            ownerCell.innerHTML = `<span class="file-owner">${file.owner || 'root'}</span>`;
            row.appendChild(ownerCell);

            // Grupo
            const groupCell = document.createElement('td');
            groupCell.innerHTML = `<span class="file-group">${file.group || 'root'}</span>`;
            row.appendChild(groupCell);

            // Fecha
            const dateCell = document.createElement('td');
            dateCell.innerHTML = `<span class="file-date">${formattedDate}</span>`;
            row.appendChild(dateCell);

            // Acciones
            const actionsCell = document.createElement('td');
            actionsCell.innerHTML = `
            <div class="file-actions">
                ${file.type === 'directory'
                ? `<button class="btn-file-action btn-open" title="Abrir directorio">
                        <i class="fas fa-folder-open"></i>
                      </button>`
                : `<button class="btn-file-action btn-view" title="Ver archivo">
                        <i class="fas fa-eye"></i>
                      </button>`
            }
            </div>
        `;
            row.appendChild(actionsCell);

            // Agregar eventos a los botones
            const fileLink = row.querySelector('.file-link');
            const actionBtn = row.querySelector('.btn-file-action');

            // Evento para el enlace o botón
            fileLink.addEventListener('click', () => handleFileClick(file));
            actionBtn.addEventListener('click', () => handleFileClick(file));

            // Agregar a la lista
            filesList.appendChild(row);
        });
    }

    // Manejar clic en archivo o directorio
    function handleFileClick(file) {
        if (file.type === 'directory') {
            // Navegar al directorio
            navigateToDirectory(file.path);
        } else {
            // Abrir el visor de archivos
            openFileViewer(file);
        }
    }

    // Navegar a un directorio
    function navigateToDirectory(path) {
        // Actualizar path y cargar directorio
        currentPath = path;
        loadDirectory(path);
    }

    // Abrir el visor de archivos
    function openFileViewer(file) {
        // Guardar información del archivo actual en sessionStorage
        sessionStorage.setItem('currentFile', JSON.stringify({
            path: file.path,
            name: file.name,
            size: file.size,
            permissions: file.permissions,
            owner: file.owner,
            group: file.group,
            partitionId: currentPartitionId
        }));

        // Redireccionar al visor de archivos
        window.location.href = `/fileviewer?id=${encodeURIComponent(currentPartitionId)}&path=${encodeURIComponent(file.path)}`;
    }

    // Actualizar navegación por rutas
    function updatePathNavigation(path) {
        const currentLocation = document.getElementById('current-location');

        // Limpiar navegación actual
        currentLocation.innerHTML = '';

        // Crear segmentos de ruta
        const pathParts = path.split('/').filter(p => p);

        // Agregar segmento de raíz
        const rootSegment = document.createElement('span');
        rootSegment.className = 'path-segment';
        rootSegment.innerHTML = '<i class="fas fa-hdd"></i> /';
        rootSegment.addEventListener('click', () => navigateToDirectory('/'));

        currentLocation.appendChild(rootSegment);

        // Construir ruta completa
        let currentPath = '';
        pathParts.forEach((part, index) => {
            currentPath += '/' + part;

            // Agregar separador
            const separator = document.createElement('span');
            separator.className = 'breadcrumb-separator';
            separator.textContent = ' / ';
            currentLocation.appendChild(separator);

            // Agregar segmento de ruta
            const segment = document.createElement('span');
            segment.className = 'path-segment';
            segment.textContent = part;

            // Si es el último segmento, marcarlo como activo
            if (index === pathParts.length - 1) {
                segment.classList.add('active');
            } else {
                // Sino, agregar evento para navegar
                const pathCopy = currentPath; // Crear copia para el closure
                segment.addEventListener('click', () => navigateToDirectory(pathCopy));
            }

            currentLocation.appendChild(segment);
        });
    }

    // Actualizar estadísticas
    function updateStatistics(files) {
        if (!files) {
            document.getElementById('directory-count').textContent = '0';
            document.getElementById('file-count').textContent = '0';
            return;
        }

        const dirCount = files.filter(f => f.type === 'directory').length;
        const fileCount = files.filter(f => f.type !== 'directory').length;

        document.getElementById('directory-count').textContent = dirCount;
        document.getElementById('file-count').textContent = fileCount;
    }

    // Mostrar/ocultar indicador de carga
    function showLoading(show) {
        document.getElementById('loading-indicator').style.display = show ? 'flex' : 'none';
    }

    // Formatear tamaño de archivo
    function formatSize(bytes) {
        if (bytes === undefined || bytes === null) return '-';
        if (bytes === 0) return '0 B';

        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        return parseFloat((bytes / Math.pow(1024, i)).toFixed(2)) + ' ' + sizes[i];
    }

    // Formatear permisos a formato legible
    function formatPermissions(permsString) {
        // Añadir log para ver el formato exacto que está llegando desde la API
        console.log("Permisos recibidos:", permsString);

        // Si no hay permisos o vienen como "---", mostrar guiones con estilo
        if (!permsString || permsString === "---") {
            return '<span class="permission-dash">-</span><span class="permission-dash">-</span><span class="permission-dash">-</span>' +
                '<span class="permission-dash">-</span><span class="permission-dash">-</span><span class="permission-dash">-</span>' +
                '<span class="permission-dash">-</span><span class="permission-dash">-</span><span class="permission-dash">-</span>';
        }

        // Verificar si los permisos son numéricos (como 644, 755, etc.)
        if (/^\d+$/.test(permsString)) {
            if (permsString.length <= 3) {
                // Convertir permisos numéricos a simbólicos
                permsString = convertNumericToSymbolic(permsString);
            } else {
                // Si son más de 3 dígitos, podría ser un timestamp u otro formato
                console.warn("Formato de permisos numérico inesperado:", permsString);
                return '<span class="permission-dash">-</span>'.repeat(9);
            }
        }

        // Si los permisos ya están en formato rwxrwxrwx pero tienen longitud incorrecta
        if (!/^[-rwxst]+$/.test(permsString) || permsString.length !== 9) {
            console.warn("Formato de permisos inesperado:", permsString);

            // Intenta normalizarlo si es posible
            if (permsString.length < 9) {
                // Rellenar con guiones hasta 9 caracteres
                permsString = permsString.padEnd(9, '-');
            } else if (permsString.length > 9) {
                // Truncar a 9 caracteres
                permsString = permsString.substring(0, 9);
            }
        }

        // Convertir a formato visual con colores
        let formatted = '';
        for (let i = 0; i < permsString.length; i++) {
            const char = permsString.charAt(i);

            if (char === 'r') {
                formatted += '<span class="permission-r">r</span>';
            } else if (char === 'w') {
                formatted += '<span class="permission-w">w</span>';
            } else if (char === 'x' || char === 's' || char === 't') {
                formatted += '<span class="permission-x">' + char + '</span>';
            } else {
                formatted += '<span class="permission-dash">-</span>';
            }
        }

        return formatted;
    }

    // Mostrar mensaje de error
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

    // Configurar efectos visuales de terminal
    function setupTerminalEffects() {
        // Efecto de scan line
        const terminalContainer = document.querySelector('.terminal-container');
        const scanline = document.createElement('div');
        scanline.className = 'scanline';
        terminalContainer.appendChild(scanline);

        // Efecto de parpadeo aleatorio
        setInterval(() => {
            const filesList = document.getElementById('files-list');
            filesList.classList.add('flicker');
            setTimeout(() => {
                filesList.classList.remove('flicker');
            }, 100);
        }, Math.random() * 10000 + 5000);
    }
});

function formatPermissions(permsString) {
    // Si no hay permisos, mostrar guiones
    if (!permsString || typeof permsString !== 'string') {
        console.warn("Permisos no válidos:", permsString);
        return '---------';
    }

    // Normalizar a 9 caracteres si no tiene el formato esperado
    if (permsString.length !== 9) {
        console.warn("Formato de permisos inesperado:", permsString);
        // Intentar interpretar el formato numérico si es posible
        if (permsString.length === 3 && /^\d+$/.test(permsString)) {
            return convertNumericToSymbolic(permsString);
        }
        return '---------';
    }

    // Convertir a formato visual con colores
    let formatted = '';
    for (let i = 0; i < permsString.length; i++) {
        const char = permsString.charAt(i);

        if (char === 'r') {
            formatted += '<span class="permission-r">r</span>';
        } else if (char === 'w') {
            formatted += '<span class="permission-w">w</span>';
        } else if (char === 'x') {
            formatted += '<span class="permission-x">x</span>';
        } else {
            formatted += '<span class="permission-dash">-</span>';
        }
    }

    return formatted;
}

function convertNumericToSymbolic(numericPerm) {
    // Asegurarnos de que sea una cadena de 3 dígitos
    if (numericPerm.length === 1) {
        numericPerm = "00" + numericPerm;
    } else if (numericPerm.length === 2) {
        numericPerm = "0" + numericPerm;
    }

    const mapping = {
        '0': '---',
        '1': '--x',
        '2': '-w-',
        '3': '-wx',
        '4': 'r--',
        '5': 'r-x',
        '6': 'rw-',
        '7': 'rwx'
    };

    let symbolic = '';
    for (let i = 0; i < numericPerm.length; i++) {
        symbolic += mapping[numericPerm[i]] || '---';
    }

    return symbolic;
}