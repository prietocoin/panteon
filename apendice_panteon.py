import urllib.request
import base64
from weasyprint import HTML

html_content = """
<!DOCTYPE html>
<html lang="es">
<head>
    <meta charset="UTF-8">
    <style>
        @page {
            size: A4 portrait;
            margin: 20mm 15mm;
            background-color: #0f172a; /* Slate 900 */
        }
        * {
            box-sizing: border-box;
        }
        body {
            font-family: 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
            color: #f8fafc; /* Slate 50 */
            margin: 0;
            padding: 0;
            font-size: 11pt;
            line-height: 1.6;
        }
        
        /* Headers */
        h1 {
            color: #38bdf8; /* Sky 400 */
            font-size: 26pt;
            text-align: center;
            margin-top: 0;
            margin-bottom: 5px;
            text-transform: uppercase;
            letter-spacing: 2px;
            border-bottom: 2px solid #334155;
            padding-bottom: 10px;
        }
        h2 {
            color: #10b981; /* Emerald 500 */
            font-size: 16pt;
            margin-top: 25px;
            margin-bottom: 15px;
            border-bottom: 1px solid #1e293b;
            padding-bottom: 5px;
            page-break-after: avoid;
        }
        h3 {
            color: #e2e8f0; /* Slate 200 */
            font-size: 13pt;
            margin-top: 20px;
            margin-bottom: 10px;
            page-break-after: avoid;
        }
        
        /* Highlights & Alerts */
        .subtitle {
            text-align: center;
            color: #94a3b8; /* Slate 400 */
            font-size: 12pt;
            margin-bottom: 30px;
            font-style: italic;
        }
        .manifesto-box {
            background-color: #1e293b; /* Slate 800 */
            border-left: 4px solid #f59e0b; /* Amber 500 */
            padding: 15px 20px;
            margin-bottom: 25px;
            border-radius: 0 8px 8px 0;
            font-size: 10.5pt;
        }
        .manifesto-box strong { color: #fcd34d; }
        
        .note-box {
            background-color: rgba(56, 189, 248, 0.1);
            border: 1px solid #0369a1;
            padding: 10px 15px;
            border-radius: 6px;
            margin: 15px 0;
            font-size: 10pt;
            color: #bae6fd;
            page-break-inside: avoid;
        }
        
        /* Code blocks */
        pre {
            background-color: #020617; /* Slate 950 */
            color: #a78bfa; /* Violet 400 */
            padding: 15px;
            border-radius: 8px;
            font-family: 'Consolas', 'Courier New', monospace;
            font-size: 9.5pt;
            overflow-x: auto;
            border: 1px solid #1e293b;
            page-break-inside: avoid;
            margin: 15px 0;
        }
        code {
            background-color: #1e293b;
            color: #fca5a5;
            padding: 2px 5px;
            border-radius: 4px;
            font-family: 'Consolas', 'Courier New', monospace;
            font-size: 9.5pt;
        }
        
        /* Tables */
        table {
            width: 100%;
            border-collapse: collapse;
            margin: 20px 0;
            font-size: 10pt;
            page-break-inside: avoid;
        }
        th {
            background-color: #1e293b;
            color: #38bdf8;
            text-align: left;
            padding: 10px;
            border-bottom: 2px solid #334155;
        }
        td {
            padding: 10px;
            border-bottom: 1px solid #1e293b;
            color: #cbd5e1;
        }
        
        /* Lists */
        ul, ol {
            margin: 10px 0 20px 0;
            padding-left: 25px;
        }
        li {
            margin-bottom: 8px;
            color: #e2e8f0;
        }
        li strong {
            color: #38bdf8;
        }
        
        /* Structural Blocks for Diagrams */
        .diagram-container {
            background-color: #1e293b;
            border-radius: 8px;
            padding: 20px;
            margin: 20px 0;
            text-align: center;
            font-family: monospace;
            color: #94a3b8;
            font-size: 9pt;
            line-height: 1.2;
            page-break-inside: avoid;
            border: 1px solid #334155;
        }
        .diagram-highlight { color: #34d399; font-weight: bold; }
        .diagram-arrow { color: #f43f5e; }
        
        /* Custom Footer */
        @page {
            @bottom-center {
                content: "El Panteón v3.0 - Apéndice Arquitectónico";
                font-size: 8pt;
                color: #64748b;
                font-family: monospace;
            }
            @bottom-right {
                content: counter(page);
                font-size: 8pt;
                color: #64748b;
            }
        }
    </style>
</head>
<body>

    <h1>🏛️ El Panteón</h1>
    <div class="subtitle">Apéndice Arquitectónico y Manual Operativo v3.0</div>

    <div class="manifesto-box">
        <strong>Visión Core:</strong> "Sin plantillas estáticas, sin Hype, sin herramientas de caja negra."<br><br>
        El Panteón es un motor de renderizado y orquestación audiovisual desarrollado en <strong>Go</strong>. Su propósito es producir vídeos técnicos (10-12 minutos) sobre Arquitectura de Software e Inteligencia Artificial, compuestos por clips atómicos procedurales (15-20s), generando Motion Graphics de alto rendimiento en Chromium sin intervención humana.
    </div>

    <h2>1. La Jerarquía Audiovisual (Fractal)</h2>
    <p>El sistema no escupe clips al azar, ni procesa un vídeo de 12 minutos en un solo pase. Utiliza una arquitectura en 3 niveles para proteger la memoria RAM del VPS (OCI/Hetzner) y permitir la paralelización:</p>

    <ul>
        <li><strong>Nivel 1: El Clip Atómico (15-20 segundos).</strong> La unidad física de compilación y renderizado. Combina una locución concreta, un árbol DOM generado al vuelo, un estado de animación CSS y efectos de sonido.</li>
        <li><strong>Nivel 2: La Escena (1 a 3 minutos).</strong> Secuencia de 4 a 9 clips unidos lógicamente. Mantiene la continuidad atmosférica y la música de fondo constante. <em>(Ej: "La Autopsia del Kernel")</em>.</li>
        <li><strong>Nivel 3: El Vídeo Maestro (10-12 minutos).</strong> Ensamblaje final de ~40 clips atómicos mediante <code>FFmpeg Concat Demuxer</code> en modo stream copy (cero recodificación).</li>
    </ul>

    <div class="note-box">
        <strong>Ventaja de la Caché (Idempotencia):</strong> Si el clip #14 necesita una corrección de guion, el orquestador SHA-256 en Go solo re-renderiza esos 15 segundos y concatena de nuevo. Costo de iteración: casi cero.
    </div>

    <h2>2. El Monorepo (Topología del Sistema)</h2>
    <p>La arquitectura adopta un enfoque de Monorepo para consolidar recursos, pero manteniendo los entornos de ejecución aislados mediante contenedores.</p>

<pre>
el-panteon/
├── services/
│   └── pavarotti/             # Microservicio de síntesis TTS (Chatterbox/Python)
├── engine/                    # Orquestador y renderizador maestro (Go)
│   ├── internal/
│   │   ├── galileo/           # Rastreador Anti-Hype (Cazador de problemas)
│   │   ├── atlas/             # Extractor de Repositorios y Docs
│   │   ├── socrates/          # Validador de Rigor Técnico
│   │   ├── cervantes/         # Guionista omnisciente (Emisor del DirectorCut)
│   │   ├── davinci/           # Motor procedural de HTML/SVG sin plantillas
│   │   ├── pixar/             # Render Determinista (Virtual Time) a 60 FPS
│   │   └── mozart/            # Mezclador FFmpeg (SFX + Auto-ducking)
├── shared/                    
│   ├── contracts/             # Esquemas JSON (EngineeringCase, DirectorCut)
│   └── assets/                # Logos (SVGs), Música, Tipografías
└── docker-compose.yml
</pre>

    <h2>3. Los Flujos de Datos Clave</h2>

    <h3>A. El Pipeline de Inteligencia (De la Web al Guion)</h3>
    <table>
        <tr>
            <th>Componente</th>
            <th>Entrada</th>
            <th>Lógica Interna</th>
            <th>Salida</th>
        </tr>
        <tr>
            <td><strong>Galileo</strong></td>
            <td>Fuentes primarias (RFCs, Post-Mortems)</td>
            <td>Aplica filtro Anti-Hype. Busca la "Tríada": Síntoma, Causa, Solución.</td>
            <td><code>engineering_case.json</code></td>
        </tr>
        <tr>
            <td><strong>Atlas</strong></td>
            <td><code>engineering_case.json</code></td>
            <td>Descarga código real de GitHub, diagramas y benchmarks de referencia.</td>
            <td><code>dossier_tecnico.json</code></td>
        </tr>
        <tr>
            <td><strong>Cervantes</strong></td>
            <td><code>dossier_tecnico.json</code></td>
            <td>Desglosa el documental en escenas y define la intención gráfica.</td>
            <td><code>director_cut.json</code> (x40)</td>
        </tr>
    </table>

    <h3>B. El Pipeline de Renderizado (Del JSON al MP4)</h3>
    <div class="diagram-container">
        [ Cervantes ] -- JSON --&gt; [ <span class="diagram-highlight">Orquestador GO</span> ] <br>
        <span class="diagram-arrow">↓</span><br>
        1. Llama a <b>Pavarotti</b> (Chatterbox HTTP) --&gt; <span class="diagram-highlight">.wav + Duración</span><br>
        2. Llama a <b>Da Vinci</b> --&gt; Construye DOM Procedural adaptado al Canvas.<br>
        3. Invoca a <b>Pixar</b> (Chromium Headless) --&gt; Avanza DOM 16.6ms / frame.<br>
        4. Graba Pipe a <b>FFmpeg</b> --&gt; MP4 crudo (60FPS Exactos).<br>
        5. Llama a <b>Mozart</b> --&gt; Mezcla audio con ducking.<br>
        <span class="diagram-arrow">↓</span><br>
        [ FFmpeg Concat ] -- Ensambla 40 clips --&gt; <span class="diagram-highlight">Master.mp4</span>
    </div>

    <h2>4. Adiós a las Plantillas: Da Vinci & Pixar</h2>
    <p>En lugar de utilizar plantillas HTML fijas donde el texto se desborda, <strong>Da Vinci opera como un solucionador de restricciones espaciales (Constraint-Based UI)</strong>.</p>
    <ul>
        <li><strong>Composición al vuelo:</strong> Si Cervantes pide "Comparativa de 3 bases de datos", Da Vinci no carga <code>plantilla_3.html</code>. Instancia 3 primitivas Atómicas (Cards), calcula un <code>flex-box</code> equitativo en un canvas estricto de 1920x1080, y traza las líneas de convergencia SVG usando matemáticas absolutas.</li>
        <li><strong>Reloj Virtual Determinista (Pixar):</strong> Pixar no captura la pantalla en tiempo real (lo que causaría <i>lag</i> o pérdida de fotogramas en el VPS). Usa <code>go-rod</code> para inyectar <code>window.advanceAnimation(16.6ms)</code>, obligando a las animaciones de interpolación CSS/JS a saltar al frame exacto, extrayendo un PNG perfecto antes de avanzar al siguiente milisegundo.</li>
    </ul>

    <h2>5. Siguiente Hito Inmediato</h2>
    <p>El desarrollo debe comenzar por la validación del bloque atómico. La prioridad número uno es:</p>
    <div class="note-box">
        <strong>Prueba de Concepto Vertical (Clip de 15 Segundos):</strong><br>
        1. Inyectar un JSON inventado (Ej: 4 tarjetas rojas de error).<br>
        2. Hacer un POST HTTP en Go hacia el contenedor Pavarotti (Easypanel) para obtener el <code>.wav</code>.<br>
        3. Generar el HTML con Da Vinci.<br>
        4. Renderizar un <code>clip_01.mp4</code> de 60FPS inyectado en FFmpeg vía pipes.
    </div>

</body>
</html>
"""

with open("apendice_panteon.html", "w", encoding="utf-8") as f:
    f.write(html_content)

HTML(filename="apendice_panteon.html").write_pdf("Apendice_Arquitectonico_El_Panteon.pdf")
print("[file-tag: Apendice_Arquitectonico_El_Panteon.pdf]")
