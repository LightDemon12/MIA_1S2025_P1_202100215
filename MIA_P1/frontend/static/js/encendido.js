document.addEventListener('DOMContentLoaded', () => {
    // Crear y precargar el sonido
    const bootSound = new Audio('/static/sounds/vintage-hard-drive-read-and-idle-28393.mp3');
    bootSound.volume = 0.4; // Ajustar volumen al 40%
    bootSound.loop = true; // Hacer que el sonido se repita

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

        // Detener el sonido con fade out
        const fadeAudio = setInterval(() => {
            if (bootSound.volume > 0.1) {
                bootSound.volume -= 0.1;
            } else {
                bootSound.pause();
                clearInterval(fadeAudio);
            }
        }, 100);

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
        // Mostrar mensaje inicial
        bootTerminal.innerHTML = '> Iniciando hardware del sistema...\n';

        // Empezar a reproducir el sonido con fade in
        bootSound.volume = 0;
        bootSound.play().catch(e => {
            console.log("Error reproduciendo sonido:", e);
            // Falló la reproducción automática, necesitamos interacción del usuario
            bootOverlay.addEventListener('click', () => {
                if (bootSound.paused) {
                    bootSound.play().catch(e => console.log("Error reproduciendo sonido:", e));
                }
            }, { once: true });
        });

        // Fade in del sonido durante 1.5 segundos
        const fadeIn = setInterval(() => {
            if (bootSound.volume < 0.4) {
                bootSound.volume += 0.05;
            } else {
                clearInterval(fadeIn);
            }
        }, 200);

        // Esperar 2 segundos para que se escuche el sonido antes de empezar la secuencia visual
        await new Promise(resolve => setTimeout(resolve, 2000));

        // Añadir un mensaje que indique que se puede omitir
        bootTerminal.innerHTML += '> Presione ENTER para omitir la animación...\n\n';

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
            // Fade out del sonido
            const fadeOut = setInterval(() => {
                if (bootSound.volume > 0.1) {
                    bootSound.volume -= 0.1;
                } else {
                    bootSound.pause();
                    clearInterval(fadeOut);
                }
            }, 100);

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

    // Iniciar secuencia de arranque con un retraso mínimo
    setTimeout(() => {
        bootSequence();
    }, 300);
});