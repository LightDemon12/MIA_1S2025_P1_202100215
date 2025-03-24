document.addEventListener('DOMContentLoaded', () => {
    // Crear overlay para efecto de encendido
    const bootOverlay = document.createElement('div');
    bootOverlay.className = 'boot-overlay';
    document.body.appendChild(bootOverlay);

    // Crear contenedor para el texto de arranque
    const bootTerminal = document.createElement('div');
    bootTerminal.className = 'boot-terminal';
    bootOverlay.appendChild(bootTerminal);

    // Variable para controlar si la animación se ha omitido
    let animationSkipped = false;

    // Función para cerrar el overlay
    const skipAnimation = () => {
        if (animationSkipped) return; // Evitar múltiples llamadas

        animationSkipped = true;

        // Mostrar mensaje de omisión
        bootTerminal.innerHTML += '\n\n> [Secuencia omitida por usuario]';
        bootTerminal.scrollTop = bootTerminal.scrollHeight;

        // Cerrar el overlay después de un breve retraso
        setTimeout(() => {
            bootOverlay.classList.add('fade-out');
            setTimeout(() => {
                if (bootOverlay.parentNode) {
                    bootOverlay.remove();
                }
            }, 1000);
        }, 500);
    };

    // Escuchar la tecla Enter para omitir la animación
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && !animationSkipped) {
            skipAnimation();
        }
    });

    // Función para simular escritura de terminal
    const typeWriter = (text, element, speed = 20, startDelay = 0) => {
        return new Promise(resolve => {
            // Si la animación ya se omitió, resolvemos inmediatamente
            if (animationSkipped) {
                resolve();
                return;
            }

            setTimeout(() => {
                let i = 0;
                const typing = setInterval(() => {
                    // Si la animación se omitió mientras se escribía
                    if (animationSkipped) {
                        clearInterval(typing);
                        resolve();
                        return;
                    }

                    if (i < text.length) {
                        element.innerHTML += text.charAt(i);
                        i++;
                        // Scroll hacia abajo para mostrar el texto nuevo
                        element.scrollTop = element.scrollHeight;
                    } else {
                        clearInterval(typing);
                        resolve();
                    }
                }, speed);
            }, startDelay);
        });
    };

    // Secuencia de arranque
    const bootSequence = async () => {

        // Inicialización
        await typeWriter('Iniciando sistema...', bootTerminal);
        await typeWriter('\n\n> Verificando integridad del sistema...', bootTerminal, 15, 300);
        await typeWriter('\n> Cargando módulos principales...', bootTerminal, 15, 500);

        // Verificar si la animación fue omitida
        if (animationSkipped) return;

        // Carga de componentes
        const components = [
            'Kernel v1.0.0',
            'Sistema de Archivos EXT2',
            'Módulo de Seguridad',
            'Interfaz de Línea de Comandos',
            'Subsistema de Entrada/Salida'
        ];

        await typeWriter('\n\n> Inicializando componentes del sistema:\n', bootTerminal, 10, 400);

        for (const component of components) {
            if (animationSkipped) return;
            await typeWriter(`\n  [■] ${component}`, bootTerminal, 20, 200);
            await new Promise(r => setTimeout(r, 300));
        }

        // Verificar de nuevo si la animación fue omitida
        if (animationSkipped) return;

        // Verificación completa
        await typeWriter('\n\n> Comprobando permisos y accesos...', bootTerminal, 15, 400);
        await typeWriter('\n> Configurando entorno de usuario...', bootTerminal, 15, 600);

        // ASCII Art Logo
        const asciiLogo = `
   __  __  _____            _______  _____  _____   __  __  _____  _   _            _      
  |  \\/  ||_   _|   /\\     |__   __||  ___||  __ \\ |  \\/  ||_   _|| \\ | |    /\\    | |     
  | \\  / |  | |    /  \\       | |   | |__  | |__) || \\  / |  | |  |  \\| |   /  \\   | |     
  | |\\/| |  | |   / /\\ \\      | |   |  __| |  _  / | |\\/| |  | |  | . \` |  / /\\ \\  | |     
  | |  | | _| |_ / ____ \\     | |   | |___ | | \\ \\ | |  | | _| |_ | |\\  | / ____ \\ | |____ 
  |_|  |_||_____|_/    \\_\\    |_|   |_____||_|  \\_\\|_|  |_||_____||_| \\_|/_/    \\_\\|______|
                                                                                         
  ========== MANEJO E IMPLEMENTACIÓN DE ARCHIVOS v1.0.0 ==========
        `;

        await typeWriter('\n\n', bootTerminal, 5, 400);
        await typeWriter(asciiLogo, bootTerminal, 1);

        // Mensaje final
        await typeWriter('\n\n> Inicialización completa.', bootTerminal, 15, 500);

        // Si llegamos aquí, la animación se completó normalmente
        if (!animationSkipped) {
            // Cierre automático después de completar la secuencia
            setTimeout(() => {
                bootOverlay.classList.add('fade-out');
                setTimeout(() => {
                    if (bootOverlay.parentNode) {
                        bootOverlay.remove();
                    }
                }, 1000);
            }, 1500);
        }
    };

    // Iniciar secuencia de arranque
    setTimeout(() => {
        bootSequence();
    }, 500);
});