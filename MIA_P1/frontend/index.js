const express = require('express');
const path = require('path');

const app = express();
const PORT = 8080;

// Middleware para analizar JSON en las solicitudes
app.use(express.json());

// Configurar directorio de archivos estáticos
app.use('/static', express.static(path.join(__dirname, 'static')));

// Permitir acceso a las plantillas HTML directamente
app.use('/templates', express.static(path.join(__dirname, 'templates')));

// Configurar motor de vistas
app.set('view engine', 'html');
app.engine('html', require('ejs').renderFile);
app.set('views', path.join(__dirname, 'templates'));

// Ruta principal - Consola
app.get('/', (req, res) => {
    res.render('consola.html');
});

// Ruta de consola explícita
app.get('/console', (req, res) => {
    res.render('consola.html');
});

app.get('/console.html', (req, res) => {
    res.render('consola.html');
});

// Ruta de login
app.get('/login', (req, res) => {
    res.render('login.html');
});

app.get('/login.html', (req, res) => {
    res.render('login.html');
});

// Ruta del explorador de archivos
app.get('/explorer', (req, res) => {
    res.render('explorer.html');
});

app.get('/explorer.html', (req, res) => {
    res.render('explorer.html');
});

// Ruta de particiones (con y sin extensión .html)
app.get('/partitions', (req, res) => {
    res.render('partitions.html');
});

app.get('/partitions.html', (req, res) => {
    res.render('partitions.html');
});

// Servir archivos HTML directamente como alternativa
app.get('*.html', (req, res) => {
    const htmlFile = req.path;
    const htmlPath = path.join(__dirname, 'templates', htmlFile);

    // Servir el archivo si existe
    res.sendFile(htmlPath, err => {
        if (err) {
            console.log(`Error sirviendo ${htmlPath}:`, err);
            res.status(404).send('Página no encontrada');
        }
    });
});
// Agregar rutas para los nuevos archivos
app.get('/files', (req, res) => {
    res.render('files.html');
});

app.get('/files.html', (req, res) => {
    res.render('files.html');
});

app.get('/fileviewer', (req, res) => {
    res.render('fileviewer.html');
});

app.get('/fileviewer.html', (req, res) => {
    res.render('fileviewer.html');
});
// Iniciar servidor
app.listen(PORT, () => {
    console.log(`Frontend ejecutándose en http://localhost:${PORT}`);
    console.log(`Conectando con backend en http://localhost:1921`);
});