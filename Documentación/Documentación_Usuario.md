# Manual de Usuario: Sistema de Administración de Discos

## Proyecto #1

### Primer Semestre de 2024

```js
Universidad San Carlos de Guatemala
Programador: Angel Guillermo de Jesús Pérez Jiménez 
Carne: 202100215
Correo: 3870961320101@ingenieria.usac.edu.gt
```

## Introducción

Bienvenido al Sistema de Administración de Discos, una herramienta completa para la gestión de discos virtuales, particiones y sistema de archivos EXT2. Esta aplicación le permite crear, manipular y visualizar estructuras de almacenamiento mediante una interfaz sencilla basada en comandos.

## Iniciar el Sistema

1. Ejecute el servidor backend con:

   ```bash
   go run main.go
   ```

2. Abra su navegador y acceda a:

   ```sh
   http://localhost:1921
   ```

3. Verá la interfaz con una consola interactiva donde podrá introducir comandos.

![Consola del sistema](SVG/consola.png)

## Comandos Disponibles

### Gestión de Discos

#### MKDISK - Crear un disco virtual

```sh
MKDISK -size=X -path=/ruta/disco.dsk -name=nombre [-unit=K|M]
```

- `size`: Tamaño del disco (obligatorio).
- `path`: Ruta donde se creará el disco (obligatorio).
- `name`: Nombre del disco (obligatorio).
- `unit`: Unidad de tamaño (`K`=Kilobytes, `M`=Megabytes). Por defecto: `M`.

**Ejemplo:**

```sh
MKDISK -size=10 -path=/home/discos/Disco1.dsk -name=Disco1 -unit=M
```

#### RMDISK - Eliminar un disco virtual

```sh
RMDISK -path=/ruta/disco.dsk
```

- `path`: Ruta del disco a eliminar (obligatorio).

**Ejemplo:**

```sh
RMDISK -path=/home/discos/Disco1.dsk
```

#### FDISK - Administrar particiones

```sh
FDISK -size=X -path=/ruta/disco.dsk -name=nombre [-unit=K|M|B] [-type=P|E|L] [-fit=BF|FF|WF] [-delete=full|fast] [-add=X]
```

- `size`: Tamaño de la partición.
- `path`: Ruta del disco.
- `name`: Nombre de la partición.
- `unit`: Unidad de tamaño (`K`=Kilobytes, `M`=Megabytes, `B`=Bytes). Por defecto: `K`.
- `type`: Tipo de partición (`P`=Primaria, `E`=Extendida, `L`=Lógica). Por defecto: `P`.
- `fit`: Tipo de ajuste (`BF`=Best Fit, `FF`=First Fit, `WF`=Worst Fit). Por defecto: `FF`.
- `delete`: Eliminar partición (`full`=completo, `fast`=rápido).
- `add`: Agregar espacio a partición existente.

**Ejemplo:**

```sh
FDISK -size=5 -path=/home/discos/Disco1.dsk -name=Part1 -unit=M
FDISK -path=/home/discos/Disco1.dsk -name=Part1 -delete=full
```

### Sistema de Archivos

#### MKFS - Formatear partición

```sh
MKFS -id=151A [-type=full] [-fs=2fs]
```

- `id`: ID de la partición montada (obligatorio).
- `type`: Tipo de formateo (`full`=completo). Por defecto: `full`.
- `fs`: Tipo de sistema de archivos (`2fs`=EXT2). Por defecto: `2fs`.

**Ejemplo:**

```sh
MKFS -id=151A -type=full
```

#### MKDIR - Crear directorio

```sh
MKDIR -path=/directorio/nuevo [-p]
```

- `path`: Ruta del directorio a crear.
- `p`: Crear directorios padres si no existen.

**Ejemplo:**

```sh
MKDIR -path=/home/user/docs -p
```

#### MKFILE - Crear archivo

```sh
MKFILE -path=/ruta/archivo.txt [-size=X] [-cont=/ruta/contenido.txt] [-p]
```

- `path`: Ruta del archivo a crear.
- `size`: Tamaño del archivo (crea contenido aleatorio).
- `cont`: Ruta de archivo local para usar como contenido.
- `p`: Crear directorios padres si no existen.

**Ejemplo:**

```sh
MKFILE -path=/home/user/datos.txt -size=2
MKFILE -path=/home/user/docs/info.txt -cont=/home/local/texto.txt -p
```

#### CAT - Mostrar contenido de archivo

```sh
CAT -file=/ruta/archivo.txt
```

- `file`: Ruta del archivo a mostrar.

**Ejemplo:**

```sh
CAT -file=/home/user/datos.txt
```

### Usuarios y Grupos

#### LOGIN - Iniciar sesión

```sh
LOGIN -user=usuario -pass=contraseña -id=151A
```

#### LOGOUT - Cerrar sesión

```sh
LOGOUT
```

#### MKGRP - Crear grupo

```sh
MKGRP -name=nombre_grupo
```

#### RMGRP - Eliminar grupo

```sh
RMGRP -name=nombre_grupo
```

#### MKUSR - Crear usuario

```sh
MKUSR -user=nombre -pass=contraseña -grp=grupo
```

#### RMUSR - Eliminar usuario

```sh
RMUSR -user=nombre
```

### Reportes

#### REP - Generar reportes

```sh
REP -name=nombre -path=/ruta/reporte.jpg -id=151A -ruta=/directorio
```

- `name`: Tipo de reporte (`mbr`, `disk`, `sb`, `inode`, `block`, etc.).
- `path`: Ruta donde se guardará el reporte.
- `id`: ID de la partición montada.
- `ruta`: Ruta dentro del sistema de archivos (para ciertos reportes).

**Ejemplo:**

```sh
REP -name=mbr -path=/home/reportes/reporte_mbr.jpg -id=151A
```

## Consejos de Uso

- **Montaje:** Anote los IDs de las particiones montadas para usarlos después.
- **Login:** Inicie sesión antes de realizar operaciones en el sistema de archivos.
- **Permisos:** Los directorios tienen permisos `755` y los archivos `644`.
- **Reportes:** Genere reportes regularmente para visualizar el sistema.
- **Scripts:** Para operaciones complejas, prepare un script con los comandos.
- **Comentarios:** Use `#` para documentar sus scripts.

## Respaldo y Recuperación

- Puede hacer copias de seguridad de sus archivos `.dsk`.
- Mover discos entre ubicaciones.
- Reutilizar discos en diferentes sesiones.

## Solución de Problemas

- **Error de comando no reconocido:** Verifique la sintaxis.
- **Error de disco no encontrado:** Compruebe la ruta.
- **Error de partición llena:** Libere espacio o aumente tamaño.
- **Error de acceso denegado:** Verifique permisos o inicie sesión.
- **Error de conexión:** Asegúrese de que el servidor está en ejecución.
