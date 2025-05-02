document.addEventListener('DOMContentLoaded', function() {
    // Inicializar
    const API_URL = 'http://localhost:1921';
    let currentFileInfo = null;

    // Obtener parámetros de la URL
    const urlParams = new URLSearchParams(window.location.search);
    const fileId = urlParams.get('id');
    const filePath = urlParams.get('path');

    // Verificar que tenemos los parámetros necesarios
    if (!fileId || !filePath) {
        // Intentar recuperar de sessionStorage
        const fileInfo = JSON.parse(sessionStorage.getItem('currentFile'));
        if (fileInfo) {
            currentFileInfo = fileInfo;
            loadFile(fileInfo.partitionId, fileInfo.path);
        } else {
            showError("No se especificó ningún archivo para visualizar");
            setTimeout(() => {
                window.location.href = '/files?id=' + (fileId || '');
            }, 2000);
        }
    } else {
        loadFile(fileId, filePath);
    }

    // Botón de volver
    document.getElementById('back-to-files').addEventListener('click', function() {
        const returnId = currentFileInfo?.partitionId || fileId;
        const returnPath = currentFileInfo?.path || filePath;

        if (returnPath) {
            // Extraer directorio padre del path
            const pathParts = returnPath.split('/');
            pathParts.pop(); // Quitar el nombre de archivo
            const parentDir = pathParts.join('/') || '/';

            window.location.href = `/files?id=${encodeURIComponent(returnId)}&path=${encodeURIComponent(parentDir)}`;
        } else {
            window.location.href = `/files?id=${encodeURIComponent(returnId)}`;
        }
    });

    // Botón de descargar
    document.getElementById('btn-download').addEventListener('click', function() {
        if (!currentFileInfo) return;

        // Crear un elemento de texto con el contenido
        const content = document.getElementById('file-content').textContent;
        const blob = new Blob([content], {type: 'text/plain'});
        const url = URL.createObjectURL(blob);

        // Crear un enlace temporal y hacer clic en él
        const a = document.createElement('a');
        a.href = url;
        a.download = currentFileInfo.name;
        document.body.appendChild(a);
        a.click();

        // Limpiar
        setTimeout(() => {
            document.body.removeChild(a);
            window.URL.revokeObjectURL(url);
        }, 0);
    });

    // Cargar contenido de archivo
    function loadFile(id, path) {
        showLoading(true);

        fetch(`${API_URL}/api/file?id=${encodeURIComponent(id)}&path=${encodeURIComponent(path)}`)
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

                // Guardar información del archivo
                currentFileInfo = {
                    name: data.nombre,
                    path: data.path,
                    size: data.tamaño,
                    permissions: data.permisos,
                    owner: data.propietario,
                    group: data.grupo,
                    partitionId: id,
                    content: data.contenido
                };

                // Actualizar UI
                updateFileInfo(currentFileInfo);

                // Mostrar contenido
                displayFileContent(data.contenido);
            })
            .catch(error => {
                showLoading(false);
                console.error("Error cargando archivo:", error);
                showError("Error cargando el archivo: " + error.message);
            });
    }

    // Actualizar información del archivo en la UI
    function updateFileInfo(fileInfo) {
        document.getElementById('file-name').textContent = fileInfo.name;
        document.getElementById('file-size').textContent = formatSize(fileInfo.size);
        document.getElementById('file-permissions').textContent = fileInfo.permissions || 'rwxrwxrwx';

        const ownerGroup = (fileInfo.owner || 'root') + ':' + (fileInfo.group || 'root');
        document.getElementById('file-owner').textContent = ownerGroup;

        document.title = `${fileInfo.name} - Visor de Archivos`;
    }

    // Mostrar contenido del archivo
    function displayFileContent(content) {
        const contentElement = document.getElementById('file-content');
        const noContentMessage = document.getElementById('no-content-message');

        if (!content) {
            contentElement.style.display = 'none';
            noContentMessage.style.display = 'block';
            return;
        }

        noContentMessage.style.display = 'none';
        contentElement.style.display = 'block';

        // Detectar tipo de contenido para formateo
        if (isJsonString(content)) {
            contentElement.innerHTML = formatJson(content);
        } else if (content.startsWith('<?xml') || content.startsWith('<')) {
            contentElement.innerHTML = formatXml(content);
        } else {
            // Texto plano
            contentElement.textContent = content;
        }

        // Agregar cursor parpadeante al final
        const cursor = document.createElement('span');
        cursor.className = 'cursor-blink';
        contentElement.appendChild(cursor);
    }

    // Formatear JSON para mejor visualización
    function formatJson(jsonString) {
        try {
            const obj = JSON.parse(jsonString);
            const formattedJson = JSON.stringify(obj, null, 2);

            // Colorear JSON
            return formattedJson
                .replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g, function(match) {
                    let cls = 'json-number';
                    if (/^"/.test(match)) {
                        if (/:$/.test(match)) {
                            cls = 'json-key';
                            match = match.replace(/: *$/, '');
                        } else {
                            cls = 'json-string';
                        }
                    } else if (/true|false/.test(match)) {
                        cls = 'json-boolean';
                    } else if (/null/.test(match)) {
                        cls = 'json-null';
                    }
                    return '<span class="' + cls + '">' + match + '</span>';
                })
                .replace(/: */g, ': ');
        } catch (e) {
            // Si hay error, mostrar como texto plano
            return jsonString;
        }
    }

    // Formatear XML para mejor visualización
    function formatXml(xmlString) {
        try {
            // Formatear XML
            const formatted = xmlString.replace(/</g, "&lt;")
                .replace(/>/g, "&gt;")
                .replace(/&lt;(\/?[\w:-]+)(.*?)(\/?)\&gt;/g, function(match, p1, p2, p3) {
                    // Tag name
                    const tag = '<span class="xml-tag">&lt;' + p1;

                    // Attributes
                    const attrs = p2.replace(/(\w+)="([^"]+)"/g,
                        '<span class="xml-attr">$1</span>="<span class="xml-value">$2</span>"');

                    // Closing bracket
                    const close = p3 + '&gt;</span>';

                    return tag + attrs + close;
                })
                .replace(/&lt;!--(.*)--&gt;/g, '<span class="xml-comment">&lt;!--$1--&gt;</span>');

            return formatted;
        } catch (e) {
            // Si hay error, mostrar como texto plano
            return xmlString.replace(/</g, "&lt;").replace(/>/g, "&gt;");
        }
    }

    // Verificar si un string es JSON válido
    function isJsonString(str) {
        try {
            JSON.parse(str);
            return true;
        } catch (e) {
            return false;
        }
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
});