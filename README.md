# HSME (Hybrid Semantic Memory Engine) v1.0.1+

HSME es un motor de memoria semántica híbrida de alto rendimiento diseñado para proporcionar una base de conocimiento persistente, con trazabilidad y observabilidad profunda para agentes de IA. Combina la velocidad de la búsqueda léxica, la profundidad de la búsqueda semántica y la estructura de un Grafo de Conocimiento técnico.

## 🏗️ Arquitectura de Tres Procesos

HSME separa runtime, procesamiento semántico y mantenimiento operativo para mantener baja latencia y permitir observabilidad escalable:

1.  **MCP Server (`hsme`)**: Servidor ligero que maneja las peticiones del agente vía `stdio`. Realiza búsquedas híbridas instantáneas y encola tareas de enriquecimiento.
2.  **Async Worker (`hsme-worker`)**: Proceso en segundo plano que consume la cola de tareas para:
    *   Generar embeddings de vectores (`nomic-embed-text`) con aceleración GPU (CUDA).
    *   Extraer entidades y relaciones técnicas (`phi3.5`).
    *   Construir el Grafo de Conocimiento dinámicamente con parsing tolerante.
3.  **Ops Runner (`hsme-ops`)**: Runner dedicado para mantenimiento operativo. Ejecuta rollups de métricas, políticas de retención y limpieza de base de datos sin afectar el rendimiento de las consultas.

## 🛡️ Mejoras de "Hardening" (v1.0.1)

*   **Integridad Autoritativa**: Implementación de **triggers de SQLite** para la sincronización automática de FTS5. Se eliminó la sincronización manual en favor de una consistencia garantizada a nivel de base de datos.
*   **Concurrencia Optimizada**: Uso de `_txlock=immediate` y limitación estratégica del pool de conexiones (MaxOpenConns=4) para maximizar el rendimiento en modo WAL, eliminando colisiones de escritura en ingestas masivas.
*   **Aceleración GPU (CUDA)**: Soporte completo para `ollama-cuda`, logrando una aceleración de **10x** en tareas de extracción de grafos y generación de embeddings.

## 📊 Observabilidad Industrial

HSME v1.0.1 integra un sistema de telemetría interno completo persistido en SQLite:
*   **Distributed Tracing**: Registro de trazas y spans para cada request MCP y tarea del worker.
*   **Metric Rollups**: Agregación automática de métricas (p50, p95, throughput) por minuto, hora y día.
*   **Retention Policies**: Limpieza automática de datos de telemetría según antigüedad y criticidad.
*   **Operator Views**: Vistas SQL predefinidas para identificar operaciones lentas y errores recurrentes (`obs_recent_slow_operations`, `obs_error_events`).

## 🚀 Instalación y Setup

### Requisitos
- Go 1.26+ con CGO habilitado.
- [Just](https://github.com/casey/just) para la gestión de tareas.
- Ollama (preferiblemente `ollama-cuda`) con modelos: `nomic-embed-text` y `phi3.5`.

### Instalación Rápida
```bash
# Compila e instala binarios en ~/go/bin
just install
```

## 🔄 Migración de Legado (Engram)

HSME incluye herramientas para la transición completa desde el sistema Engram original, restaurando metadatos históricos y asegurando la integridad del corpus:

### Flujo de Cutover
1. **Full Run**: `just migrate full` — Sincroniza metadatos, limpia basura e ingiere huérfanos.
2. **Cutover**: Desactivar el servidor MCP de Engram en el cliente (Claude Code).
3. **Delta Replay**: `just migrate delta` — Ingiere cualquier escritura realizada durante la ventana de transición.
4. **Verificación**: `just verify-cutover` — Compara conteos entre legado y HSME.

Consulta la [Guía de Cutover](docs/legacy-cutover-checklist.md) para el checklist operativo paso a paso.

## 📁 Filtrado por Proyecto

Las herramientas de búsqueda ahora soportan un parámetro opcional `project` para restringir los resultados a un contexto específico:

*   **`search_fuzzy`**: Acepta `project` (string) para filtrar candidatos léxicos y semánticos.
*   **`search_exact`**: Acepta `project` (string) para búsquedas de subcadenas exactas.
*   **`store_context`**: Permite asignar un `project` al guardar nuevas memorias.

Si se omite el proyecto, la búsqueda se realiza sobre todo el corpus (comportamiento por defecto).


## ⏳ Ranking por Recencia (Time Decay)

HSME soporta el decaimiento exponencial de resultados según su antigüedad para priorizar información fresca sin perder relevancia semántica.

### Configuración
- `RRF_TIME_DECAY`: Establecer en `on` o `true` para activar el decaimiento (por defecto `off`).
- `RRF_HALF_LIFE_DAYS`: Vida media en días (por defecto `14.0`). Un documento con esta antigüedad verá su score de relevancia reducido a la mitad.

### Benchmarking
Puedes evaluar el impacto del decaimiento en tu corpus actual usando la herramienta de benchmark:
```bash
# Compilar y ejecutar contra el corpus actual
go build -tags "sqlite_fts5 sqlite_vec" -o bench-decay ./cmd/bench-decay
./bench-decay -db data/engram.db -half-life 14.0
```
Los reportes se generan en `data/benchmarks/<run_id>/` e incluyen comparativas OFF vs ON tanto para búsqueda difusa como exacta.

### Seguridad y Rollback
El decaimiento está desactivado por defecto. Para revertir cualquier cambio en el ranking, simplemente elimina la variable de entorno `RRF_TIME_DECAY` o establécela en `off`.

## 📂 Operación y Mantenimiento

### Comandos de Just
- `just serve`: Inicia el servidor MCP.
- `just work-bg`: Lanza el worker semántico en segundo plano (GPU acelerado).
- `just ops-loop`: Corre el runner de mantenimiento y rollups de métricas.
- `just migrate [full|delta]`: Ejecuta la migración desde Engram legado.
- `just verify-cutover`: Ejecuta el script de validación de integridad post-migración.
- `just status`: Instantánea de salud del sistema, progreso de la cola y estado del grafo.
- `just backup/restore`: Gestión de snapshots atómicos compatibles con WAL.

## 🔌 Configuración del Cliente MCP

```json
{
  "mcpServers": {
    "hsme": {
      "command": "/absolute/path/to/hsme",
      "env": {
        "SQLITE_DB_PATH": "/absolute/path/to/data/engram.db",
        "OLLAMA_HOST": "http://localhost:11434",
        "OBS_LEVEL": "basic" 
      }
    }
  }
}
```

---
**Desarrollo**: Este proyecto sigue los principios de **Spec-Driven Development (SDD)** con **Strict TDD Mode**. Consulta el `Technical_Specification.md` para detalles internos del esquema y el flujo de ingesta.
