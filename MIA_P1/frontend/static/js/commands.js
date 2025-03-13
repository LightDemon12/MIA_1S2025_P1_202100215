const commands = {
    clear: {
        execute: (input, output) => {
            output.value = '';
            input.value = '';
            return true;
        },
        description: "Limpia ambas consolas"
    },
    help: {
        execute: (input, output) => {
            let helpText = "Comandos disponibles:\n\n";
            for (const [cmd, info] of Object.entries(commands)) {
                helpText += `${cmd}: ${info.description}\n`;
            }
            output.value += helpText + '\n';
            return true;
        },
        description: "Muestra la lista de comandos disponibles"
    }
};

export default commands;