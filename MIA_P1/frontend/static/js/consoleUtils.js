export function handleConsoleInput(e, textArea) {
    if (e.key === 'Enter') {
        e.preventDefault();
        return true;
    }
    return false;
}


export function showError(output, command) {
    output.value += `Error: El comando '${command}' no está permitido. Ejecute 'help' para más información.\n`;
}

export function setupFileInput(inputConsole, outputConsole) {
    const fileInput = document.getElementById('fileInput');
    const fileLabel = document.querySelector('.custom-file-label');

    fileInput.addEventListener('click', function() {
        // Limpiar el valor del input para permitir cargar el mismo archivo
        fileInput.value = '';
    });

    fileInput.addEventListener('change', function(e) {
        const file = e.target.files[0];
        if (file) {
            // Limpiar consolas
            inputConsole.value = '';
            outputConsole.value = '';

            fileLabel.textContent = file.name;

            const reader = new FileReader();
            reader.onload = function(event) {
                const content = event.target.result;

                // Pequeño delay para asegurar que la limpieza se complete
                setTimeout(() => {
                    inputConsole.value = content;
                    if (!inputConsole.value.endsWith('\n')) {
                        inputConsole.value += '\n';
                    }
                    inputConsole.scrollTop = inputConsole.scrollHeight;
                }, 100);
            };

            reader.onerror = function(error) {
                console.error('Error leyendo archivo:', error);
                outputConsole.value += 'Error al cargar el archivo\n';
            };

            reader.readAsText(file);
        }
    });
}