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
            output.value += `
  +--------------------------------------------------+
  |                SISTEMA EXT2 - AYUDA              |
  +--------------------------------------------------+
  |                                                  |
  |  mkdisk    : Crea un disco                       |
  |  rmdisk    : Elimina un disco                    |
  |  fdisk     : Administra particiones              |
  |  mount     : Monta particiones                   |
  |  unmount   : Desmonta particiones                |
  |  mkfs      : Formatea particiones                |
  |  rep       : Genera reportes                     |
  |  exec      : Ejecuta archivo de comandos         |
  |  pause     : Pausa la ejecución                  |
  |  clear     : Limpia la consola                   |
  |                                                  |
  +--------------------------------------------------+
  
`;
            output.scrollTop = output.scrollHeight;
            return true;
        },
        description: "Muestra la lista de comandos disponibles"
    }
};

export default commands;