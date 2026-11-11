# Incident: connection pool exhaustion

## Symptom

La API de pagos devolvió errores 503 durante 12 minutos.
La latencia p99 subió de 180 ms a 4200 ms.

## Root cause

El pool de conexiones se agotó porque una migración abrió transacciones sin liberar conexiones.

## Solution

Se añadió un límite de conexiones por instancia y un circuit breaker.

- Limitar el pool a 20 conexiones por instancia.
- Abrir el circuit breaker tras 5 timeouts.

## Tradeoffs

- Menor throughput pico durante ráfagas.

## Verification

La latencia p99 volvió a 180 ms durante 72 horas y los errores 503 desaparecieron.
