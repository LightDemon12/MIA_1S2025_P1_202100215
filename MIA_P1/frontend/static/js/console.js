import commands from './commands.js';
import { handleConsoleInput, showError, setupFileInput } from './consoleUtils.js';

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

async function enviarComandos(comandos, outputConsole) {
    try {
        // Filtrar líneas vacías y separar por saltos de línea
        const listaComandos = comandos.split('\n').filter(cmd => cmd.trim() !== '');

        for (const comando of listaComandos) {
            if (comando.trim()) {
                // Verificar si es un comentario (línea que comienza con #)
                if (comando.trim().startsWith('#')) {
                    // Simplemente omitir comentarios y continuar con el siguiente comando
                    continue;
                }

                outputConsole.value += `> ${comando}\n`;

                if (comando.toLowerCase().trim() === 'clear' || comando.toLowerCase().trim() === 'help') {
                    const cmd = commands[comando.toLowerCase().trim()];
                    cmd.execute(inputConsole, outputConsole);
                    continue;
                }


                // Resto del código para enviar al servidor...
                const response = await fetch('http://localhost:1921/analizar', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'text/plain',
                    },
                    body: comando
                });

                const data = await response.json();

                if (data.requiereConfirmacion) {
                    outputConsole.value += `${data.mensaje}\n`;

                    // Usar la ventana personalizada en lugar de confirm
                    const confirmar = await createConsoleDialog(data.mensaje);

                    // Imprimir en consola para debug
                    console.log("Solicitud de confirmación recibida:", data);
                    console.log("Usuario respondió:", confirmar);

                    if (confirmar) {
                        try {
                            const createResponse = await fetch('http://localhost:1921/crear-directorio', {
                                method: 'POST',
                                headers: {
                                    'Content-Type': 'application/json',
                                },
                                body: JSON.stringify({
                                    path: data.dirPath,
                                    comando: data.comando
                                })
                            });

                            const createData = await createResponse.json();
                            outputConsole.value += `${createData.mensaje}\n`;

                            // Si se creó exitosamente, reintentamos el comando original
                            if (createData.exito && createData.comando) {
                                // Reintentar el comando original correctamente
                                await enviarComandos(data.comando, outputConsole);
                            }
                        } catch (error) {
                            outputConsole.value += `Error al crear directorio: ${error}\n`;
                            console.error('Error:', error);
                        }
                    } else {
                        outputConsole.value += "Operación cancelada\n";
                    }
                    continue;
                }

                outputConsole.value += `${data.mensaje}\n`;
            }
        }
    } catch (error) {
        outputConsole.value += `Error de conexión: Asegúrese que el servidor esté corriendo en puerto 1921\n`;
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
});

document.addEventListener('DOMContentLoaded', () => {
    const inputConsole = document.getElementById('inputConsole');
    const outputConsole = document.getElementById('outputConsole');
    const btnExecute = document.getElementById('btnExecute');
    const btnClear = document.getElementById('btnClear');

    setupFileInput(inputConsole, outputConsole);

    inputConsole.addEventListener('keydown', async (e) => {
        if (e.key === 'Enter') {
            e.preventDefault();
            const command = inputConsole.value.split('\n').pop().trim();

            if (command) {
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
});