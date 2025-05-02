// URL base del backend

document.addEventListener('DOMContentLoaded', function() {
    // Verificar si ya hay una sesión activa
    checkSession();

    // Event listeners
    document.getElementById('loginSubmitBtn').addEventListener('click', handleLogin);

    document.getElementById('loginCancelBtn').addEventListener('click', function() {
        window.location.href = '/';
    });

    // Permitir enviar el formulario con Enter
    document.getElementById('loginPass').addEventListener('keypress', function(e) {
        if (e.key === 'Enter') {
            handleLogin();
        }
    });

    // Añadir efectos de escaneo a los campos
    document.querySelectorAll('.terminal-input').forEach(input => {
        input.addEventListener('focus', function() {
            this.parentNode.classList.add('scan-effect');
        });

        input.addEventListener('blur', function() {
            this.parentNode.classList.remove('scan-effect');
        });
    });
});

// Verificar si hay sesión activa
function checkSession() {
    fetch(`${API_URL}/api/session`)
        .then(response => response.json())
        .then(data => {
            const sessionActive = data.activa === true;

            // Actualizar UI
            updateNavbarUI(sessionActive);

            // Solo redireccionar en áreas protegidas, no en la consola pública
            // Cambio clave: Quita la redirección automática de la consola si no hay sesión

            // Si estamos en explorador sin sesión, redirigir a login
            if (window.location.pathname.includes('explorer') && !sessionActive) {
                window.location.href = '/login';
            }

            // Si estamos en login con sesión activa, redirigir a consola
            if (window.location.pathname.includes('login') && sessionActive) {
                window.location.href = '/';
            }
        })
        .catch(error => {
            console.error('Error verificando sesión:', error);
        });
}

// Manejar el inicio de sesión
function handleLogin() {
    const user = document.getElementById('loginUser').value;
    const pass = document.getElementById('loginPass').value;
    const id = document.getElementById('loginPartition').value;

    // Validación básica
    if (!user || !pass || !id) {
        showTerminalMessage('ERROR: Todos los campos son obligatorios', 'error');
        shakeForm();
        return;
    }

    // Mostrar animación de carga
    showTerminalMessage('Autenticando...', 'loading');

    // Crear el objeto de datos exactamente como lo espera el backend
    const loginData = {
        user: user,
        pass: pass,
        id: id
    };

    console.log('Enviando datos de login:', loginData);

    fetch(`${API_URL}/api/login`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(loginData)
    })
        .then(response => {
            console.log('Respuesta del servidor:', response);
            // Verificar si la respuesta es 401 (No autorizado)
            if (response.status === 401) {
                throw new Error('Credenciales inválidas');
            }
            // Verificar si hay otros errores HTTP
            if (!response.ok) {
                throw new Error(`Error del servidor: ${response.status}`);
            }
            return response.json();
        })
        .then(data => {
            console.log('Datos de respuesta:', data);

            if (data.exito) {
                // Mensaje de éxito
                showTerminalMessage('ACCESO CONCEDIDO', 'success');

                // Efectos adicionales de éxito
                const loginForm = document.querySelector('.login-terminal');
                loginForm.style.boxShadow = '0 0 30px rgba(45, 194, 0, 0.7)';

                // Redirección después de un breve retardo
                setTimeout(() => {
                    window.location.href = '/';
                }, 1500);
            } else {
                // Mostrar mensaje específico si existe
                const errorMsg = data.mensaje || 'Error desconocido en la autenticación';
                showTerminalMessage('ACCESO DENEGADO: ' + errorMsg, 'error');

                // Mostrar detalles adicionales en la consola para depuración
                console.error('Detalles del error:', data);

                shakeForm('strong');
            }
        })
        .catch(error => {
            console.error('Error completo:', error);

            // Mensaje de error más descriptivo
            let errorMessage;
            if (error.message.includes('Failed to fetch') ||
                error.message.includes('NetworkError')) {
                errorMessage = 'No se pudo conectar al servidor. Verifique que el backend esté funcionando.';
            } else if (error.message.includes('Credenciales inválidas')) {
                errorMessage = 'Usuario o contraseña incorrectos';
            } else {
                errorMessage = `Error: ${error.message}`;
            }

            showTerminalMessage('ERROR: ' + errorMessage, 'error');
            shakeForm('strong');
        });
}

// Función mejorada para mostrar mensajes al estilo terminal
function showTerminalMessage(message, type) {
    const messageElement = document.getElementById('loginMessage');

    // Limpiar cualquier contenido o clase previa
    messageElement.innerHTML = '';
    messageElement.className = 'login-message';

    // Eliminar diagnóstico anterior si existe
    const oldDiagnostic = document.querySelector('.diagnostic-help');
    if (oldDiagnostic) {
        oldDiagnostic.remove();
    }

    switch(type) {
        case 'error':
            messageElement.classList.add('message-error');
            messageElement.innerHTML = `
                <span class="terminal-prefix">[!] </span>
                <span class="message-content">${message}</span>
            `;
            // Efecto de parpadeo para errores
            messageElement.style.animation = 'textflicker 1s ease-in-out';

            // Solo mostrar ayuda de diagnóstico en errores
            setTimeout(getDiagnosticHelp, 800);
            break;

        case 'success':
            messageElement.classList.add('message-success');
            messageElement.innerHTML = `
                <span class="terminal-prefix">[✓] </span>
                <span class="message-content">${message}</span>
            `;
            break;

        case 'loading':
            messageElement.classList.add('message-loading');
            messageElement.innerHTML = `
                <span class="terminal-prefix">[⚙] </span>
                <span class="message-content">${message}</span>
                <span class="terminal-cursor">_</span>
            `;
            break;

        default:
            messageElement.innerHTML = `
                <span class="terminal-prefix">[>] </span>
                <span class="message-content">${message}</span>
            `;
    }
}

// Efecto de shake para formulario inválido con niveles diferentes
function shakeForm(intensity = 'medium') {
    const loginForm = document.querySelector('.login-terminal');
    loginForm.classList.add(`shake-effect-${intensity}`);

    // Efecto de glitch para errores severos
    if (intensity === 'strong') {
        const inputs = document.querySelectorAll('.terminal-input');
        inputs.forEach(input => {
            input.classList.add('glitch-effect');
            setTimeout(() => {
                input.classList.remove('glitch-effect');
            }, 1000);
        });
    }

    setTimeout(() => {
        loginForm.classList.remove(`shake-effect-${intensity}`);
    }, 500);
}

// Función separada para actualizar UI de la navbar
function updateNavbarUI(sessionActive) {
    // Mostrar/ocultar botones según el estado de la sesión
    const loginBtn = document.getElementById('btn-login');
    if (loginBtn) {
        loginBtn.style.display = sessionActive ? 'none' : 'flex';
    }

    const sessionButtons = document.querySelectorAll('.session-required');
    sessionButtons.forEach(button => {
        button.style.display = sessionActive ? 'flex' : 'none';
    });

    // Si estamos en la página del explorador, ocultar el botón del explorador
    if (window.location.pathname.includes('explorer')) {
        const explorerBtn = document.getElementById('btn-explorer');
        if (explorerBtn) {
            explorerBtn.style.display = 'none';
        }
    }

    // Si estamos en login o consola, ocultar el botón de regresar
    if (window.location.pathname.includes('login') ||
        window.location.pathname.includes('console') ||
        window.location.pathname === '/' ||
        window.location.pathname === '') {
        const backBtn = document.getElementById('btn-back');
        if (backBtn) {
            backBtn.style.display = 'none';
        }
    }
}

// Añade esta función para mostrar los errores más comunes de forma amigable
// Función actualizada para mostrar los errores con la fuente correcta
function getDiagnosticHelp() {
    const helpElement = document.createElement('div');
    helpElement.className = 'diagnostic-help';

    // Crear cada elemento por separado para mejor control del estilo
    const header = document.createElement('h4');
    header.textContent = '[DIAGNÓSTICO]';

    const paragraph = document.createElement('p');
    paragraph.textContent = 'Si no puede iniciar sesión, verifique lo siguiente:';

    const list = document.createElement('ul');

    // Crear cada elemento de la lista con el estilo correcto
    const items = [
        'El servidor backend está ejecutándose en puerto 1921',
        'La partición especificada existe y tiene un sistema de archivos EXT2/3',
        'El usuario y contraseña son correctos',
        'El ID de partición está en formato correcto (ej. 151A)'
    ];

    items.forEach(item => {
        const li = document.createElement('li');
        li.textContent = item;
        list.appendChild(li);
    });

    // Crear el botón con la clase correcta
    const button = document.createElement('button');
    button.id = 'btnTestBackend';
    button.className = 'btn-terminal btn-sm';
    button.textContent = 'VERIFICAR BACKEND';

    // Agregar todo al contenedor
    helpElement.appendChild(header);
    helpElement.appendChild(paragraph);
    helpElement.appendChild(list);
    helpElement.appendChild(button);

    // Agregarlo al DOM después del mensaje de error
    const messageElement = document.getElementById('loginMessage');
    messageElement.parentNode.insertBefore(helpElement, messageElement.nextSibling);

    // Configurar el botón de verificación
    document.getElementById('btnTestBackend').addEventListener('click', testBackendConnection);
}

// Función para probar la conexión con el backend
function testBackendConnection() {
    showTerminalMessage('Verificando conexión con el backend...', 'loading');

    fetch(`${API_URL}/api/disks`)
        .then(response => {
            if (!response.ok) {
                throw new Error(`Error del servidor: ${response.status}`);
            }
            return response.json();
        })
        .then(data => {
            showTerminalMessage('Conexión exitosa con el backend. El servidor está funcionando correctamente.', 'success');
            console.log('Discos disponibles:', data);
        })
        .catch(error => {
            showTerminalMessage('Error al conectar con el backend. Verifique que el servidor esté ejecutándose.', 'error');
            console.error('Error de conexión:', error);
        });
}