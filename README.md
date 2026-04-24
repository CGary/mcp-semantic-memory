# HSME (Hybrid Semantic Memory Engine) v1.0.1

HSME es un motor de memoria semántica híbrida de alto rendimiento diseñado para proporcionar una base de conocimiento persistente y con trazabilidad para agentes de IA. Combina la velocidad de la búsqueda léxica, la profundidad de la búsqueda semántica y la estructura de un Grafo de Conocimiento técnico.

## 🏗️ Arquitectura de Doble Proceso

HSME separa la interfaz de usuario de las tareas pesadas de procesamiento para garantizar una latencia mínima en el servidor MCP:

1.  **MCP Server (`hsme`)**: Servidor ligero que maneja las peticiones del agente vía `stdio`. Realiza búsquedas híbridas instantáneas y encola nuevas memorias.
2.  **Async Worker (`hsme-worker`)**: Proceso en segundo plano que consume la cola de tareas para:
    *   Generar embeddings de vectores (`nomic-embed-text`).
    *   Extraer entidades y relaciones técnicas (`phi3.5`).
    *   Construir el Grafo de Conocimiento dinámicamente.

## 🚀 Instalación y Setup

### Requisitos
- Go 1.26+ con CGO habilitado.
- [Just](https://github.com/casey/just) (recomendado) para la gestión de tareas.
- Ollama instalado y accesible con los modelos: `nomic-embed-text` y `phi3.5`.

### Instalación Rápida
```bash
# Compila e instala binarios en ~/go/bin y los copia a la raíz
just install
```

## 📂 Operación y Mantenimiento

El motor utiliza **SQLite** con los módulos **FTS5** y **sqlite-vec** integrados. La base de datos central reside en `data/engram.db`.

### Comandos de Just
- `just serve`: Inicia el servidor MCP (Interactivo).
- `just work-bg`: Lanza el procesador de grafos y embeddings en segundo plano (Log: `worker_new.log`).
- `just status`: Muestra una instantánea del progreso del procesamiento y salud del grafo.
- `just watch-status`: Monitoreo en tiempo real del procesamiento de la cola.
- `just backup`: Crea un backup atómico compatible con el modo WAL de SQLite.
- `just restore`: Restaura el backup más reciente previa verificación de integridad.

## 🔌 Configuración del Cliente MCP

Para integrar HSME con **Gemini CLI**, **Claude Desktop** o cualquier cliente MCP, añade la configuración apuntando al binario absoluto:

```json
{
  "mcpServers": {
    "hsme": {
      "command": "/home/gary/dev/hsme/hsme",
      "env": {
        "SQLITE_DB_PATH": "/home/gary/dev/hsme/data/engram.db",
        "OLLAMA_HOST": "http://localhost:11434"
      }
    }
  }
}
```

## 🧠 Capacidades del Motor

### 1. Búsqueda Híbrida (RRF)
Implementa **Reciprocal Rank Fusion** para combinar resultados de:
*   **FTS5**: Precisión léxica para términos técnicos exactos, comandos y nombres de archivos.
*   **Vector Search**: Similitud semántica para conceptos y descripciones abstractas.

### 2. Grafo de Conocimiento Técnico
El worker extrae automáticamente:
*   **Entidades**: `TECH` (tecnologías), `FILE` (rutas), `CMD` (comandos), `ERROR` (logs).
*   **Relaciones**: `DEPENDS_ON`, `RESOLVES`, `CAUSES`.
*   **Trazabilidad**: Cada nodo y relación está vinculado a la memoria original (evidencia).

### 3. Concurrencia Robusta
Optimizado para el uso local intenso:
*   **Modo WAL**: Permite lecturas concurrentes mientras el worker escribe los embeddings.
*   **Write Mutex**: Protege el canal `stdout` para evitar corrupciones de JSON-RPC durante la ejecución concurrente.

---
**Desarrollo**: Este proyecto sigue los principios de **Spec-Driven Development (SDD)** con **Strict TDD Mode** habilitado. Consulta el `Technical_Specification.md` para detalles internos del esquema y el flujo de ingesta.
