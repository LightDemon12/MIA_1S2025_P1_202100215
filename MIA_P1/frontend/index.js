const express = require('express');
const path = require('path');

const app = express();
const PORT = 8080;

// Configurar directorio de archivos estáticos
app.use('/static', express.static(path.join(__dirname, 'static')));

// Configurar motor de vistas
app.set('view engine', 'html');
app.engine('html', require('ejs').renderFile);
app.set('views', path.join(__dirname, '..', 'frontend', 'templates'));

// Ruta principal
app.get('/', (req, res) => {
    res.render('consola.html');
});

// Iniciar servidor
app.listen(PORT, () => {
    console.log(`Frontend ejecutándose en http://localhost:${PORT}`);
});