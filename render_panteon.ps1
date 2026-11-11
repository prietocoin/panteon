# Genera Apendice_Arquitectonico_El_Panteon.pdf
# WeasyPrint necesita Pango/GTK3: se usa el runtime instalado en el perfil del usuario
# (C:\Users\maracaibo\gtk-env, creado con micromamba/conda-forge).
$gtk = "C:\Users\maracaibo\gtk-env"
$env:PATH = "$gtk\Library\bin;$gtk\Library\mingw-w64\bin;$env:PATH"
$env:WEASYPRINT_DLL_DIRECTORIES = "$gtk\Library\bin;$gtk\Library\mingw-w64\bin"

Set-Location $PSScriptRoot
& C:\Python314\python.exe .\apendice_panteon.py
