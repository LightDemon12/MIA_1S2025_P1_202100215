# Documentación Técnica: Sistema de Administración de Discos y Sistema de Archivos EXT2

## Proyecto #1

### Primer Semestre de 2024

```js
Universidad San Carlos de Guatemala
Programador: Angel Guillermo de Jesús Pérez Jiménez 
Carne: 202100215
Correo: 3870961320101@ingenieria.usac.edu.gt
```

## 1. Descripción General del Proyecto

Este proyecto implementa un sistema completo para administrar discos virtuales y un sistema de archivos EXT2. Desarrollado en Go con una interfaz web, permite crear, manipular y visualizar estructuras de discos, particiones y sistema de archivos mediante una serie de comandos especializados. El sistema está diseñado con un enfoque modular, separando claramente las responsabilidades entre analizador de comandos, gestor de discos y sistema de archivos.

## 2. Arquitectura del Sistema

El sistema se compone de tres componentes principales:

- **Frontend:** Interfaz web con una consola interactiva.
- **Analizador de Comandos:** Interpreta y valida comandos del usuario.
- **Gestor de Discos:** Ejecuta operaciones en discos, particiones y sistema de archivos.

La arquitectura sigue un patrón cliente-servidor donde el frontend envía comandos al servidor Go, que los analiza, valida y ejecuta utilizando sus componentes especializados.

## 3. Componentes Principales

### 3.1 Frontend (Interfaz de Usuario)

Construido con HTML, CSS y JavaScript, proporciona:

- Consola interactiva para entrada de comandos.
- Soporte para scripts con múltiples comandos.
- Visualización de respuestas y reportes gráficos.
- Diálogos de confirmación para operaciones destructivas.
- Auto-desplazamiento para mostrar siempre los resultados más recientes.

### 3.2 Analizador de Comandos

Módulo que procesa la entrada del usuario:

- Identifica tipos de comandos mediante análisis léxico simple.
- Extrae parámetros con sus valores (formato `-parámetro=valor`).
- Valida sintaxis y semántica de cada comando.
- Distribuye comandos a sus manejadores específicos.
- Gestiona respuestas y mensajes de error.
- Maneja confirmaciones para operaciones potencialmente peligrosas.

### 3.3 Gestor de Discos (DiskManager)

Núcleo del sistema que implementa:

- Creación y eliminación de discos virtuales (archivos binarios).
- Particionado conforme a estándar MBR (primarias, extendidas, lógicas).
- Sistema de montaje con identificadores únicos.
- Formateo e inicialización de sistema de archivos EXT2.
- Operaciones CRUD para archivos y directorios.
- Gestión de usuarios, grupos y permisos.
- Generación de reportes visuales de estructuras internas.

## 4. Sistema de Archivos EXT2

### 4.1 Estructuras Implementadas

- **Superbloque:** Almacena metadatos del sistema de archivos.
- **Bitmap de Inodos:** Registra inodos libres/ocupados.
- **Bitmap de Bloques:** Registra bloques libres/ocupados.
- **Tabla de Inodos:** Almacena inodos (metadatos de archivos).
- **Bloques de Datos:** Almacena contenido de archivos/directorios.
- **Bloques de Punteros:** Maneja referencia a bloques para archivos grandes.

### 4.2 Características Implementadas

- Sistema de archivos jerárquico con directorios y archivos.
- Bloque de directorios con entradas `.` y `..`.
- Sistema de permisos `rwx` para propietario, grupo y otros (`755/644`).
- Gestión de bloques directos e indirectos (simple, doble, triple).
- Gestión de espacio eficiente con asignación dinámica.
- Recuperación de espacio al eliminar archivos/directorios.
- Fechas de creación, modificación y acceso.

## 5. Comandos Principales

### 5.1 Gestión de Discos

- `MKDISK`: Crea un disco virtual con tamaño específico.
- `RMDISK`: Elimina un disco virtual existente.
- `FDISK`: Crea, elimina o modifica particiones.
- `MOUNT`: Monta una partición para su uso.
- `MOUNTED`: Lista particiones montadas.

### 5.2 Sistema de Archivos

- `MKFS`: Formatea una partición con sistema EXT2.
- `MKDIR`: Crea directorios en el sistema de archivos.
- `MKFILE`: Crea archivos con contenido opcional.
- `CAT`: Muestra contenido de archivos.

### 5.3 Usuarios y Permisos

- `LOGIN/LOGOUT`: Gestión de sesión de usuario.
- `MKGRP/RMGRP`: Administra grupos de usuarios.
- `MKUSR/RMUSR`: Administra usuarios del sistema.
- `CHGRP`: Cambia grupo principal de usuario.

### 5.4 Reportes

- `REP`: Genera reportes visuales con diferentes tipos:
  - `MBR`: Estructura del Master Boot Record.
  - `DISK`: Visualización del disco y particiones.
  - `SB`: Detalles del Superbloque.
  - `INODE`: Información de inodos.
  - `BLOCK`: Visualización de bloques.
  - `BM_INODE`: Bitmap de inodos.
  - `BM_BLOCK`: Bitmap de bloques.
  - `TREE`: Árbol de directorios y archivos.
  - `LS`: Listado de directorios.
  - `FILE`: Contenido y detalles de archivo.

## 6. Flujo de Ejecución Típico

1. Usuario introduce comando en la interfaz web.
2. Frontend envía comando al servidor Go.
3. Analizador identifica tipo de comando y extrae parámetros.
4. Validador específico verifica integridad y coherencia.
5. Manejador ejecuta operación usando componentes del DiskManager.
6. Sistema genera respuesta y la envía al frontend.
7. Frontend muestra resultados y/o visualizaciones.

## 7. Gestión de Errores

- Validación exhaustiva de parámetros antes de ejecutar comandos.
- Verificación de existencia de archivos, directorios y particiones.
- Control de permisos basado en usuario actual.
- Manejo de conflictos en operaciones (nombres duplicados, etc.).
- Confirmaciones para operaciones destructivas o riesgosas.
- Sistema de logging para depuración y auditoría.

## 8. Tecnologías Utilizadas

- **Go:** Lenguaje principal del backend.
- **Gin:** Framework HTTP para Go.
- **JavaScript/HTML/CSS:** Frontend y consola interactiva.
- **Graphviz:** Generación de visualizaciones y reportes.
- **Manejo binario puro:** Para manipulación de estructuras en disco.

## 9. Seguridad

- Control de acceso basado en usuario/grupo.
- Permisos `rwx` para propietario, grupo y otros.
- Prevención de eliminación accidental con confirmaciones.
- Validación de rutas para prevenir acceso no autorizado.

## 10. Limitaciones y Consideraciones

- Implementación educativa del estándar EXT2.
- Optimizado para operaciones interactivas más que para alto rendimiento.
- No implementa journal (como en EXT3/EXT4).

## 11. Diagrama de flujo del proyecto

![Diagrama de flujo del proyecto](SVG/Flujo.svg)
