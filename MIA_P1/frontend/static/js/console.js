import commands from './commands.js';
import { handleConsoleInput, showError, setupFileInput } from './consoleUtils.js';

document.addEventListener('DOMContentLoaded', () => {
    const inputConsole = document.getElementById('inputConsole');
    const outputConsole = document.getElementById('outputConsole');
    const btnExecute = document.getElementById('btnExecute');
    const btnClear = document.getElementById('btnClear');

    // Configurar el manejador de archivos
    setupFileInput(inputConsole, outputConsole);

    // Resto del código para manejar comandos
    inputConsole.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') {
            e.preventDefault();
            const command = inputConsole.value.split('\n').pop().trim();

            if (command) {
                outputConsole.value += `> ${command}\n`;

                const cmd = commands[command.toLowerCase()];
                if (cmd) {
                    if (command.toLowerCase() === 'clear') {
                        cmd.execute(inputConsole, outputConsole);
                    } else {
                        cmd.execute(inputConsole, outputConsole);
                        inputConsole.value += '\n';
                    }
                } else {
                    showError(outputConsole, command);
                    inputConsole.value += '\n';
                }
            } else {
                inputConsole.value += '\n';
            }

            inputConsole.scrollTop = inputConsole.scrollHeight;
        }
    });
});

btnExecute.addEventListener('click', () => {
    const lastLine = inputConsole.value.split('\n').pop().trim();
    if (lastLine) {
        outputConsole.value += `> ${lastLine}\n`;

        const cmd = commands[lastLine.toLowerCase()];
        if (cmd) {
            cmd.execute(inputConsole, outputConsole);
        } else {
            showError(outputConsole, lastLine);
        }

        inputConsole.value += '\n';
        inputConsole.scrollTop = inputConsole.scrollHeight;
    }
});

btnClear.addEventListener('click', () => {
    commands.clear.execute(inputConsole, outputConsole);
});