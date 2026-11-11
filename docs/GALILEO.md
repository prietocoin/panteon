# Galileo — Fase 1 (MVP)

> Para el pipeline completo (Sócrates, Atlas, Cervantes, Da Vinci, Pavarotti,
> Pixar, Mozart y encoder) consulta [`PIPELINE.md`](PIPELINE.md).

Galileo es el primer componente del pipeline de inteligencia de El Panteón. Convierte
una fuente primaria (post-mortem, RFC, reporte de incidente) en un
`engineering_case.json` verificable, con la Tríada **Síntoma → Causa → Solución** y
un veredicto Anti-Hype.

Estado actual: **MVP determinista**, sin LLM. El extractor lee encabezados conocidos.
La interfaz `extract.Extractor` queda preparada para conectar un modelo real en la
siguiente fase sin tocar el resto del pipeline.

## Estructura

```text
panteon/
├── go.mod
├── engine/
│   ├── cmd/panteon/                 # CLI
│   └── internal/galileo/
│       ├── contract/                # Tipos y validación del caso
│       ├── source/                  # Ingesta y normalización
│       ├── extract/                 # Interfaz Extractor + MarkedExtractor
│       ├── filter/                  # Reglas Anti-Hype
│       ├── store/                   # Artefactos y cache key
│       ├── galileo.go               # Orquestación
│       ├── galileo_test.go
│       └── testdata/postmortem.md
└── shared/contracts/
    ├── engineering_case.schema.json
    └── engineering_case.example.json
```

## Uso

Desde `C:\Proyectos\panteon`:

```powershell
# Caso desde un archivo local
go run ./engine/cmd/panteon galileo run --file engine/internal/galileo/testdata/postmortem.md --out out/cases

# Caso desde una URL pública
go run ./engine/cmd/panteon galileo run --url https://ejemplo.com/postmortem --out out/cases

# JSON en stdout (para tuberías)
go run ./engine/cmd/panteon galileo run --file fuente.md --json

# Compilar el binario
go build -o panteon.exe ./engine/cmd/panteon
```

Salida esperada:

```text
Galileo: Incident: connection pool exhaustion
Veredicto: accept (score 100)
Caso: out\cases\sha256-....json
Cache: sha256:...
```

## Formato de entrada del MVP

El `MarkedExtractor` reconoce encabezados Markdown, texto plano y variantes en
español/inglés. Las secciones obligatorias son `symptom`, `cause` y `solution`.

```markdown
# Incident: connection pool exhaustion

## Symptom
La API devolvió errores 503 durante 12 minutos.

## Root cause
El pool se agotó porque una migración no liberó las conexiones.

## Solution
Se añadió un límite de conexiones.
- Limitar el pool a 20 conexiones por instancia.
- Abrir el circuit breaker tras 5 timeouts.

## Tradeoffs
- Menor throughput pico.

## Verification
La latencia p99 volvió a 180 ms durante 72 horas.
```

Encabezados aceptados:

| Sección | Variantes |
|---|---|
| Symptom | `symptom`, `symptoms`, `síntoma`, `síntomas`, `señal` |
| Cause | `cause`, `root cause`, `causa`, `causa raíz` |
| Solution | `solution`, `fix`, `remediation`, `solución`, `acciones` |
| Tradeoffs | `tradeoff(s)`, `cost(s)`, `riesgos`, `compensaciones` |
| Verification | `verification`, `validation`, `verificación`, `cómo se verificó` |

Cada línea se conserva como evidencia con un localizador `line:N`. Si falta alguna
de las tres secciones obligatorias, Galileo devuelve `extract.ErrMissingSections`.

## Reglas Anti-Hype

El score arranca en 100 y se descuentan puntos por cada regla incumplida:

| Regla | Penalización | Qué detecta |
|---|---|---|
| `hype_lexicon` | 10 por término (máx. 40) | "revolucionario", "10x", "sin esfuerzo", "world-class", etc. |
| `missing_evidence` | 35 por claim | Síntoma, causa o solución sin cita literal. |
| `vague_cause` | 35 | "error humano", "mala configuración", "fallo del sistema". |
| `numeric_claim_without_evidence` | 6 por dato (máx. 18) | Números en el resumen que no aparecen en ninguna cita. |
| `not_observable` | 20 | Síntoma sin métrica, error, caída o timeout. |
| `missing_tradeoffs` | 10 | Solución sin compensaciones o riesgos. |
| `missing_verification` | 15 | Solución sin explicación de cómo se verificó. |

Veredicto:

| Score | Veredicto |
|---|---|
| 70–100 | `accept` |
| 40–69 | `review` |
| 0–39 | `reject` |

Las reglas viven en `engine/internal/galileo/filter/filter.go` y se prueban con casos
tabulares en `filter_test.go`.

## Contrato de salida

El esquema está en `shared/contracts/engineering_case.schema.json` y el ejemplo real
en `shared/contracts/engineering_case.example.json`.

Invariantes principales:

- `id` y `source.content_hash` son `sha256:` del texto normalizado.
- `symptom`, `cause` y `solution` exigen al menos una cita con `quote` y `locator`.
- `cause.mechanism` debe describir un mecanismo técnico, no una etiqueta vaga.
- `hype_filter` incluye `score`, `verdict`, `flags` y `unsupported_claims`.
- `generated_at` y `source.retrieved_at` son timestamps UTC.

## Idempotencia y caché

`store.CacheKey` combina:

```text
SHA-256(content_hash || model || prompt_version)
```

Reejecutar la misma fuente con el mismo modelo y versión de prompt produce el mismo
`CacheKey` y sobrescribe el mismo artefacto. En la fase del LLM, esta clave debe
usarse para evitar volver a gastar tokens.

## Siguiente fase: conectar un LLM

1. Implementar `extract.Extractor` en un paquete nuevo, por ejemplo
   `engine/internal/galileo/extract/llm`.
2. Enviar el texto de la fuente como **dato no confiable**, delimitado y con
   instrucciones explícitas de ignorar órdenes embebidas (defensa contra prompt
   injection).
3. Exigir salida JSON validada contra el esquema; rechazar y reintentar si no valida.
4. Obligar a que cada claim incluya `quote` literal y `locator`. Si el modelo no
   puede citar, el claim se descarta.
5. Mantener `MarkedExtractor` como extractor de referencia para tests y modo offline.
6. Pasar `Model` y `PromptVersion` al `Service` para que la caché sea correcta.

## Seguridad de ingesta

- Solo se permiten `http` y `https`.
- Se bloquean loopback, rangos privados, link-local y multicast (SSRF).
- Límite de 5 MB por respuesta, timeout de 20 s y máximo 3 redirecciones.
- El HTML se convierte a texto y se eliminan `script`, `style` y `noscript`.
- Limitación conocida del MVP: la validación de IP ocurre antes de la petición; para
  producción conviene fijar la IP resuelta en el `DialContext` para evitar DNS
  rebinding.

## Comandos de verificación

```powershell
gofmt -w engine
go vet ./...
go test ./...
```
