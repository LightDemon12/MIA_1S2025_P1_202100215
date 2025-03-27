import commands from './commands.js';
import {setupFileInput } from './consoleUtils.js';
import '/static/js/encendido.js';

function createConsoleDialog(message) {
    return new Promise((resolve) => {
        const dialog = document.createElement('div');
        dialog.className = 'console-dialog';

        const content = document.createElement('div');
        content.className = 'console-dialog-content';
        content.textContent = message;

        const buttonsDiv = document.createElement('div');
        buttonsDiv.className = 'console-dialog-buttons';

        const yesButton = document.createElement('button');
        yesButton.className = 'console-button';
        yesButton.textContent = 'Si';
        yesButton.onclick = () => {
            document.body.removeChild(dialog);
            resolve(true);
        };

        const noButton = document.createElement('button');
        noButton.className = 'console-button';
        noButton.textContent = 'No';
        noButton.onclick = () => {
            document.body.removeChild(dialog);
            resolve(false);
        };

        buttonsDiv.appendChild(yesButton);
        buttonsDiv.appendChild(noButton);
        dialog.appendChild(content);
        dialog.appendChild(buttonsDiv);
        document.body.appendChild(dialog);
    });
}

// Función auxiliar para hacer scroll al final de la consola
function scrollToBottom(console) {
    console.scrollTop = console.scrollHeight;
}

async function enviarComandos(comandos, outputConsole) {
    try {
        // Filtrar líneas vacías y separar por saltos de línea
        const listaComandos = comandos.split('\n').filter(cmd => cmd.trim() !== '');

        for (const comando of listaComandos) {
            if (comando.trim()) {
                // Mostrar los comentarios con formato diferente
                if (comando.trim().startsWith('#')) {
                    outputConsole.value += `${comando}\n`;
                } else {
                    outputConsole.value += `> ${comando}\n`;
                }

                scrollToBottom(outputConsole);

                // Para comandos especiales del frontend, no enviar al backend
                if (comando.toLowerCase().trim() === 'clear' || comando.toLowerCase().trim() === 'help') {
                    const cmd = commands[comando.toLowerCase().trim()];
                    cmd.execute(inputConsole, outputConsole);
                    continue;
                }

                // Todos los comandos, incluidos los comentarios, se envían al backend
                const response = await fetch('http://localhost:1921/analizar', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'text/plain',
                    },
                    body: comando
                });

                const data = await response.json();

                if (data.requiereConfirmacion) {
                    console.log("Debug - Received confirmation request:", data); // Debug log
                    outputConsole.value += `${data.mensaje}\n`;
                    scrollToBottom(outputConsole);

                    const confirmar = await createConsoleDialog(data.mensaje);

                    if (confirmar) {
                        try {
                            // Always use ext2-crear-directorios for mkfile operations
                            const isMkfileOperation = data.comando.toLowerCase().startsWith('mkfile');
                            const endpoint = isMkfileOperation ?
                                'http://localhost:1921/ext2-crear-directorios' :
                                'http://localhost:1921/crear-directorio';

                            let requestBody;
                            if (isMkfileOperation) {
                                requestBody = {
                                    path: data.path || data.dirPath,
                                    command: data.comando,
                                    confirm: true,
                                    overwrite: data.tipoConfirmacion === 'sobreescribir'
                                };
                            } else {
                                requestBody = {
                                    path: data.dirPath,
                                    comando: data.comando
                                };
                            }

                            console.log(`Debug - Sending request to ${endpoint}:`, requestBody); // Debug log

                            const createResponse = await fetch(endpoint, {
                                method: 'POST',
                                headers: {
                                    'Content-Type': 'application/json',
                                },
                                body: JSON.stringify(requestBody)
                            });

                            const createData = await createResponse.json();
                            outputConsole.value += `${createData.mensaje}\n`;
                            scrollToBottom(outputConsole);

                            // Keep existing retry logic for non-mkfile operations
                            if (createData.exito && createData.comando && !isMkfileOperation) {
                                await enviarComandos(data.comando, outputConsole);
                            }
                        } catch (error) {
                            outputConsole.value += `Error al procesar confirmación: ${error}\n`;
                            scrollToBottom(outputConsole);
                            console.error('Error:', error);
                        }
                    } else {
                        outputConsole.value += "Operación cancelada\n";
                        scrollToBottom(outputConsole);
                    }
                    continue;
                }

                // Mostrar respuesta del backend
                if (!comando.trim().startsWith('#') || data.mensaje.trim() !== "") {
                    outputConsole.value += `${data.mensaje}\n`;
                    scrollToBottom(outputConsole);
                }
            }
        }
    } catch (error) {
        outputConsole.value += `Error de conexión: Asegúrese que el servidor esté corriendo en puerto 1921\n`;
        scrollToBottom(outputConsole);
        console.error('Error:', error);
    }
}

document.addEventListener('DOMContentLoaded', () => {
    const inputConsole = document.getElementById('inputConsole');
    const outputConsole = document.getElementById('outputConsole');
    const btnExecute = document.getElementById('btnExecute');
    const btnClear = document.getElementById('btnClear');

    setupFileInput(inputConsole, outputConsole);

    // Botón Ejecutar
    btnExecute.addEventListener('click', async () => {
        const contenido = inputConsole.value;
        if (contenido.trim()) {
            await enviarComandos(contenido, outputConsole);
            inputConsole.value += '\n';
            inputConsole.scrollTop = inputConsole.scrollHeight;
        }
    });

    // Manejar pegado de texto
    inputConsole.addEventListener('paste', async (e) => {
        // Permitir que el texto se pegue normalmente
        setTimeout(async () => {
            // No enviamos automáticamente al pegar
            inputConsole.scrollTop = inputConsole.scrollHeight;
        }, 0);
    });

    // Manejar tecla Enter
    inputConsole.addEventListener('keydown', async (e) => {
        if (e.key === 'Enter') {
            e.preventDefault();
            const comando = inputConsole.value.split('\n').pop().trim();

            if (comando) {
                await enviarComandos(comando, outputConsole);
            }

            inputConsole.value += '\n';
            inputConsole.scrollTop = inputConsole.scrollHeight;
        }
    });

    // Botón Limpiar
    btnClear.addEventListener('click', () => {
        commands.clear.execute(inputConsole, outputConsole);
    });

    // Función para activar pantalla completa
    const goFullScreen = () => {
        const docElement = document.documentElement;

        if (docElement.requestFullscreen) {
            docElement.requestFullscreen();
        } else if (docElement.mozRequestFullScreen) { /* Firefox */
            docElement.mozRequestFullScreen();
        } else if (docElement.webkitRequestFullscreen) { /* Chrome, Safari & Opera */
            docElement.webkitRequestFullscreen();
        } else if (docElement.msRequestFullscreen) { /* IE/Edge */
            docElement.msRequestFullscreen();
        }
    };

    // Activar pantalla completa con el primer clic del usuario
    document.body.addEventListener('click', function fullscreenOnClick() {
        goFullScreen();
        // Eliminar el event listener después del primer clic
        document.body.removeEventListener('click', fullscreenOnClick);
    }, { once: true });
});

document.addEventListener('DOMContentLoaded', () => {
    // Primero configuramos la interfaz de consola
    const inputConsole = document.getElementById('inputConsole');
    const outputConsole = document.getElementById('outputConsole');
    const btnExecute = document.getElementById('btnExecute');
    const btnClear = document.getElementById('btnClear');

    // Variable para rastrear si necesitamos volver a pantalla completa
    let needsFullscreen = false;

    setupFileInput(inputConsole, outputConsole);

    // Función para activar pantalla completa
    const goFullScreen = () => {
        const docElement = document.documentElement;

        if (docElement.requestFullscreen) {
            docElement.requestFullscreen();
        } else if (docElement.mozRequestFullScreen) { /* Firefox */
            docElement.mozRequestFullScreen();
        } else if (docElement.webkitRequestFullscreen) { /* Chrome, Safari & Opera */
            docElement.webkitRequestFullscreen();
        } else if (docElement.msRequestFullscreen) { /* IE/Edge */
            docElement.msRequestFullscreen();
        }
    };

    // Añadir detector para fileInput que marca la necesidad de volver a pantalla completa
    document.getElementById('fileInput').addEventListener('change', () => {
        // Marcar que necesitamos volver a pantalla completa
        needsFullscreen = true;
    });

    // Detector de clics global para restaurar pantalla completa después de seleccionar archivo
    document.addEventListener('click', (e) => {
        // Si necesitamos volver a pantalla completa y no se está haciendo clic en el selector de archivos
        if (needsFullscreen && e.target.id !== 'fileInput') {
            // Restablecer la variable
            needsFullscreen = false;

            // Pequeño retraso para asegurar que el selector se haya cerrado
            setTimeout(() => {
                goFullScreen();
            }, 100);
        }
    });

    inputConsole.addEventListener('keydown', async (e) => {
        if (e.key === 'Enter') {
            e.preventDefault();
            const command = inputConsole.value.split('\n').pop().trim();

            if (command) {
                // Añadir efecto de vibración a la consola
                outputConsole.classList.add('shake-effect');
                setTimeout(() => {
                    outputConsole.classList.remove('shake-effect');
                }, 500);

                outputConsole.value += `> ${command}\n`;
                const cmdLower = command.toLowerCase();

                // Manejar comandos locales
                if (cmdLower === 'clear' || cmdLower === 'help') {
                    const cmd = commands[cmdLower];
                    cmd.execute(inputConsole, outputConsole);
                    if (cmdLower !== 'clear') {
                        inputConsole.value += '\n';
                    }
                } else {
                    // Enviar al backend cualquier otro comando
                    await enviarComando(command, outputConsole);
                    inputConsole.value += '\n';
                }
            } else {
                inputConsole.value += '\n';
            }

            inputConsole.scrollTop = inputConsole.scrollHeight;
        }
    });

    btnExecute.addEventListener('click', async () => {
        const command = inputConsole.value.split('\n').pop().trim();
        if (command) {
            // Añadir efecto de vibración a la consola
            outputConsole.classList.add('shake-effect');
            setTimeout(() => {
                outputConsole.classList.remove('shake-effect');
            }, 500);

            outputConsole.value += `> ${command}\n`;
            const cmdLower = command.toLowerCase();

            // Manejar comandos locales
            if (cmdLower === 'clear' || cmdLower === 'help') {
                const cmd = commands[cmdLower];
                cmd.execute(inputConsole, outputConsole);
                if (cmdLower !== 'clear') {
                    inputConsole.value += '\n';
                }
            } else {
                // Enviar al backend cualquier otro comando
                await enviarComando(command, outputConsole);
                inputConsole.value += '\n';
            }

            inputConsole.scrollTop = inputConsole.scrollHeight;
        }
    });

    btnClear.addEventListener('click', () => {
        commands.clear.execute(inputConsole, outputConsole);
    });

    // Activar pantalla completa con el primer clic del usuario
    document.body.addEventListener('click', function fullscreenOnClick() {
        goFullScreen();
        // Eliminar el event listener después del primer clic
        document.body.removeEventListener('click', fullscreenOnClick);
    }, { once: true });

    // Efecto periódico de vibración de terminal
    function setupPeriodicShake() {
        // Función que aplica un efecto aleatorio de shake
        const applyRandomShake = () => {
            // Intensidad aleatoria
            const intensity = Math.random();
            let effectClass = '';

            if (intensity < 0.7) {
                // 70% de probabilidad: efecto ligero
                effectClass = 'shake-effect-light';
            } else if (intensity < 0.95) {
                // 25% de probabilidad: efecto medio
                effectClass = 'shake-effect-medium';
            } else {
                // 5% de probabilidad: efecto fuerte
                effectClass = 'shake-effect-strong';
            }

            // Aplicar el efecto
            outputConsole.classList.add(effectClass);

            // Quitar el efecto después de la animación
            setTimeout(() => {
                outputConsole.classList.remove(effectClass);
            }, 500);
        };

        // Configurar intervalos aleatorios para el shake
        function scheduleNextShake() {
            // Tiempo entre 10 y 40 segundos (más natural)
            const nextTime = 10000 + Math.random() * 30000;

            setTimeout(() => {
                applyRandomShake();
                scheduleNextShake(); // Programar el siguiente
            }, nextTime);
        }

        // Iniciar la secuencia de shakes
        scheduleNextShake();
    }

    // Iniciar los efectos periódicos
    setupPeriodicShake();
});

