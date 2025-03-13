import commands from './commands.js';
import { handleConsoleInput, showError, setupFileInput } from './consoleUtils.js';

async function enviarComandos(comandos, outputConsole) {
    try {
        // Dividir el texto en líneas y filtrar líneas vacías
        const listaComandos = comandos.split('\n').filter(cmd => cmd.trim() !== '');

        // Procesar cada comando
        for (const comando of listaComandos) {
            if (comando.trim()) {
                outputConsole.value += `> ${comando}\n`;

                // Verificar si es un comando local
                const cmdLower = comando.toLowerCase().trim();
                if (cmdLower === 'clear' || cmdLower === 'help') {
                    const cmd = commands[cmdLower];
                    cmd.execute(inputConsole, outputConsole);
                    continue;
                }

                // Enviar comando al backend
                const response = await fetch('http://localhost:1921/analizar', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'text/plain',
                    },
                    body: comando
                });

                if (!response.ok) {
                    throw new Error(`HTTP error! status: ${response.status}`);
                }

                const data = await response.json();
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