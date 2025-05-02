document.addEventListener('DOMContentLoaded', function() {
    // Cargar el navbar desde el archivo HTML
    loadNavbar();
});

// URL base del backend
const API_URL = 'http://localhost:1921';

// URL base del frontend (por defecto es la actual)
const FRONTEND_URL = 'http://localhost:8080';

// Función para cargar el navbar
function loadNavbar() {
    fetch('/templates/navbar.html')
        .then(response => {
            if (!response.ok) {
                throw new Error('No se pudo cargar la navbar');
            }
            return response.text();
        })
        .then(html => {
            document.getElementById('navbar-container').innerHTML = html;

            // Agregar event listeners a los botones después de cargar la navbar
            setupNavbarListeners();

            // Ajustar visibilidad según la página actual (no según la sesión)
            adjustButtonVisibility();

            // Verificar sesión solo para redirecciones necesarias
            checkSessionForRedirects();
        })
        .catch(error => {
            console.error('Error cargando la navbar:', error);
        });
}

// Configurar listeners para los botones de navbar
function setupNavbarListeners() {
    // Botón de login
    const loginBtn = document.getElementById('btn-login');
    if (loginBtn) {
        loginBtn.addEventListener('click', function() {
            window.location.href = '/login';
        });
    }

    // Botón de consola
    const consoleBtn = document.getElementById('btn-console');
    if (consoleBtn) {
        consoleBtn.addEventListener('click', function() {
            window.location.href = '/';
        });
    }

    // Botón del explorador
    const explorerBtn = document.getElementById('btn-explorer');
    if (explorerBtn) {
        explorerBtn.addEventListener('click', function() {
            checkSessionBeforeNavigate('/explorer');
        });
    }

    // Botón regresar
    const backBtn = document.getElementById('btn-back');
    if (backBtn) {
        backBtn.addEventListener('click', function() {
            window.history.back();
        });
    }

    // Botón logout
    const logoutBtn = document.getElementById('btn-logout');
    if (logoutBtn) {
        logoutBtn.addEventListener('click', function() {
            logout();
        });
    }
}

// Ajustar visibilidad basado solo en la página actual
function adjustButtonVisibility() {
    // Asegurar que todos los botones estén visibles por defecto
    const allButtons = document.querySelectorAll('.navbar-btn');
    allButtons.forEach(button => {
        button.style.display = 'flex';
    });

    // Si estamos en la página de consola, ocultar el botón de consola
    if (window.location.pathname.includes('console') || window.location.pathname === '/' || window.location.pathname === '') {
        const consoleBtn = document.getElementById('btn-console');
        if (consoleBtn) {
            consoleBtn.style.display = 'none';
        }
    }

    // Si estamos en la página del explorador, ocultar el botón del explorador
    if (window.location.pathname.includes('explorer')) {
        const explorerBtn = document.getElementById('btn-explorer');
        if (explorerBtn) {
            explorerBtn.style.display = 'none';
        }
    }

    // Si estamos en la página de la consola, ocultar el botón de regresar
    if (window.location.pathname.includes('console') || window.location.pathname === '/' || window.location.pathname === '') {
        const backBtn = document.getElementById('btn-back');
        if (backBtn) {
            backBtn.style.display = 'none';
        }
    }

    // Si estamos en la página de login, ocultar el botón de regresar
    if (window.location.pathname.includes('login')) {
        const backBtn = document.getElementById('btn-back');
        if (backBtn) {
            backBtn.style.display = 'none';
        }
    }
}

// Verificar si hay sesión activa (solo para redirecciones)
function checkSessionForRedirects() {
    fetch(`${API_URL}/api/session`)
        .then(response => response.json())
        .then(data => {
            const sessionActive = data.activa === true;
            const isLoginPage = window.location.pathname.includes('login');

            // Si estamos en explorador sin sesión, redirigir a login
            if (window.location.pathname.includes('explorer') && !sessionActive) {
                window.location.href = '/login';
            }

            // Si estamos en login con sesión activa, redirigir a consola
            if (isLoginPage && sessionActive) {
                window.location.href = '/';
            }
        })
        .catch(error => {
            console.error('Error verificando sesión:', error);
        });
}

// Verificar sesión antes de navegar a áreas protegidas
function checkSessionBeforeNavigate(destination) {
    fetch(`${API_URL}/api/session`)
        .then(response => response.json())
        .then(data => {
            if (data.activa) {
                window.location.href = destination;
            } else {
                window.location.href = '/login';
            }
        })
        .catch(error => {
            console.error('Error verificando sesión:', error);
            // En caso de error, intentar navegar de todas formas
            window.location.href = destination;
        });
}


// Cerrar sesión
function logout() {
    fetch(`${API_URL}/api/logout`, {
        method: 'POST'
    })
        .then(response => response.json())
        .then(data => {
            // Redirigir a login después de cerrar sesión
            window.location.href = '/login';
        })
        .catch(error => {
            console.error('Error al cerrar sesión:', error);
        });
}