# Idea: CLI tool `hsme-cli` para consumir los tools del MCP

## Motivación

Hoy las cuatro herramientas (`store_context`, `search_fuzzy`, `search_exact`,
`explore_knowledge_graph`) sólo se consumen vía MCP (JSON-RPC sobre stdio).
Una CLI permite:

- Uso interactivo desde la terminal sin levantar un agente MCP.
- Scripting / automatizaciones (cron, hooks de git, pipes).
- Debugging directo del core sin pasar por el wrapper MCP.
- Inspección rápida de la base de datos en producción.

## Estado del código (verificado)

El core ya está desacoplado del MCP. Toda la lógica vive en `src/core/*` y se
consume por funciones puras con dependencias inyectadas. El paquete `src/mcp/`
sólo maneja transporte JSON-RPC.

| Tool MCP | Función core | Archivo |
|----------|--------------|---------|
| `search_fuzzy` | `search.FuzzySearch(ctx, db, embedder, query, limit)` | `src/core/search/fuzzy.go` |
| `search_exact` | `search.ExactSearch(ctx, db, keyword, limit)` | `src/core/search/fuzzy.go` |
| `store_context` | `indexer.StoreContext(db, content, sourceType, supersedesID, forceReingest)` | `src/core/indexer/ingest.go` |
| `explore_knowledge_graph` | `search.TraceDependencies(ctx, db, canonical, direction, maxDepth, maxNodes)` | `src/core/search/graph.go` |

Inicialización compartida con `cmd/hsme/main.go`:

- `sqlite.InitDB(dbPath)` — abre la DB y aplica migraciones.
- `ollama.NewClient(host)` + `ollama.NewEmbedder(client, "nomic-embed-text", 768)`
- `sqlite.ValidateEmbeddingConfig(db, embedder)` — chequea coherencia con `system_config`.

## Alcance

### En scope

1. Binario nuevo `cmd/cli/main.go` que invoque las cuatro funciones core.
2. Subcomandos: `store`, `search-fuzzy`, `search-exact`, `explore`.
3. Flags compartidos: `--db`, `--ollama-host`, `--embedding-model`, `--format` (`json`|`text`).
4. Targets en `justfile`: `just cli-build`, `just cli-install`.
5. Variables de entorno reutilizadas: `SQLITE_DB_PATH`, `OLLAMA_HOST`, `EMBEDDING_MODEL`.

### Fuera de scope

- Modo interactivo (REPL).
- Worker o ops ya existen como binarios separados (`cmd/worker`, `cmd/ops`).
- Backups / restore (ya hay `scripts/backup_hot.sh` y `scripts/restore.sh`).
- Reindex (deferred — sería un subcomando futuro `hsme-cli admin reindex`).

## Diseño

### Estructura de archivos

```
cmd/cli/
  main.go         # entry point, parser de subcomandos, init compartido
  store.go        # subcomando store
  search.go       # subcomandos search-fuzzy y search-exact
  graph.go        # subcomando explore
  output.go       # formateo JSON / text
```

### Subcomandos y mapeo a funciones core

```
hsme-cli store --source-type note [--supersedes ID] [--force-reingest] < contenido.txt
  → indexer.StoreContext(db, content, sourceType, supersedesID, forceReingest)

hsme-cli search-fuzzy "redis architecture" --limit 5
  → search.FuzzySearch(ctx, db, embedder, query, limit)

hsme-cli search-exact "FTS5" --limit 10
  → search.ExactSearch(ctx, db, keyword, limit)

hsme-cli explore "Redis" --direction both --max-depth 5 --max-nodes 100
  → indexer.CanonicalizeName(name); search.TraceDependencies(ctx, db, canonical, ...)
```

### Parser de argumentos

Recomendación: **stdlib `flag` con dispatch manual por `os.Args[1]`**.
Razones:

- Cero dependencias nuevas (alineado con la práctica del repo, que evita libs externas).
- 4 subcomandos no justifican `cobra` (~50 KB de binario extra) ni `urfave-cli`.
- El patrón `flag.NewFlagSet` por subcomando es trivial de mantener.

Si en el futuro la CLI crece a 10+ subcomandos con anidamiento, migrar a
`cobra` es directo.

### Formato de salida

Dos modos vía `--format`:

- `text` (default): humano-legible, una entrada por bloque, headers en negrita
  con códigos ANSI sólo si stdout es TTY (`isatty`).
- `json`: salida cruda, compatible con `jq` y scripts. Forma idéntica a la
  que devuelven los handlers MCP **sin** el wrapper `{"content":[{"type":"text",...}]}`.

Ejemplo:

```bash
$ hsme-cli search-exact "FTS5" --format json | jq '.results[0].text'
$ hsme-cli search-fuzzy "ollama gpu" --limit 3
```

### Códigos de salida

- `0` éxito
- `1` error de uso (flags inválidos, subcomando inexistente)
- `2` error de runtime (DB no inicializa, embedder mismatch, query falla)

### Inicialización compartida

Refactor opcional: extraer la inicialización de `cmd/hsme/main.go:67-90` a
`src/bootstrap/init.go` con una función `Bootstrap()` que devuelva
`*sql.DB` y `*ollama.Embedder`. Lo consumen `cmd/hsme`, `cmd/worker`,
`cmd/ops` y el nuevo `cmd/cli`.

Si no se hace el refactor, se duplican ~25 líneas de boilerplate. **Aceptable
para empezar; el refactor puede ir en una WP siguiente.**

### `embedder` opcional para subcomandos que no lo necesitan

`store_context` y `search_exact` no usan el embedder en runtime. Para evitar
exigir que Ollama esté arriba para `search-exact` (caso típico de scripting),
inicializar el embedder de forma **lazy** sólo cuando el subcomando lo pida.

```go
// init lazy
var embedder *ollama.Embedder
needsEmbedder := subcommand == "search-fuzzy"
if needsEmbedder {
    embedder = initEmbedder()
}
```

## Tests

### Qué se rompe

**Nada.** Los tests existentes (`tests/modules/*`) prueban el core
directamente, no a través del MCP. Agregar `cmd/cli/` no toca esa superficie.

### Qué tests nuevos

Tabla-driven en `tests/modules/cli_test.go` o `cmd/cli/main_test.go`:

1. Parseo de flags por subcomando (cada uno con su `flag.NewFlagSet`).
2. Mapeo correcto de `os.Args` a la función core llamada.
3. Formateo `json` y `text` para cada tool.
4. Manejo de stdin para `store` (lectura de contenido).
5. Códigos de salida en errores.

Para los tests de integración real se usa la misma pattern del proyecto:
build con tags `sqlite_fts5 sqlite_vec` y DB temporal (ver
`tests/modules/storage_test.go`).

## justfile

Agregar al `justfile`:

```just
cli-build:
    go build -tags "sqlite_fts5 sqlite_vec" -o hsme-cli ./cmd/cli

cli-install: cli-build
    install -m 755 hsme-cli /home/gary/go/bin/hsme-cli

# Smoke test rápido contra la DB local
cli-smoke: cli-build
    ./hsme-cli search-exact "ollama" --limit 1 --format json
```

Y al `just install` actual sumarle `cli-install` para que el flujo de
instalación deje los cuatro binarios listos: `hsme`, `hsme-worker`,
`hsme-ops`, `hsme-cli`.

## Trabajo de implementación estimado

| Item | Esfuerzo |
|------|----------|
| `cmd/cli/main.go` con dispatcher de subcomandos | 1h |
| 4 subcomandos (`store`, `search-fuzzy`, `search-exact`, `explore`) | 2h |
| Formato `text` y `json` con detección TTY | 1h |
| Tests de parseo y mapeo a core | 1h |
| Targets en `justfile` y verificación end-to-end | 30min |
| **Total** | **~5h** |

Refactor opcional `src/bootstrap/init.go` (compartir init): **+1h**.

## Riesgos

1. **Embedder lazy mal implementado** podría romper `search-fuzzy` con un
   panic si Ollama no está arriba. Mitigación: chequeo explícito antes del
   primer uso con error claro.
2. **Lectura de stdin para `store`** puede colgarse si el usuario lo invoca
   interactivo sin EOF. Mitigación: detectar TTY en stdin y mostrar mensaje
   de uso si no hay redirección.
3. **Coherencia de schema embedder** ya está cubierta por
   `sqlite.ValidateEmbeddingConfig`; la CLI debe llamarla igual que el MCP
   server al iniciar para evitar corromper datos con un embedder distinto.
4. **Concurrencia con MCP server activo**: SQLite con WAL permite multiples
   readers. Para escrituras (store), si el MCP server está corriendo, ambos
   abren la misma DB sin conflicto gracias al WAL — pero conviene
   documentarlo.

## Referencias en el código

- Inicialización de DB y embedder: `cmd/hsme/main.go:67-90`
- Registro de tools en MCP (cómo se llaman las funciones core):
  `cmd/hsme/main.go:97-207`
- Wrappers de resultados (referencia para la salida JSON):
  `cmd/hsme/main.go:20-36`
- Funciones core a invocar:
  - `src/core/indexer/ingest.go` → `StoreContext`
  - `src/core/search/fuzzy.go` → `FuzzySearch`, `ExactSearch`
  - `src/core/search/graph.go` → `TraceDependencies`
  - `src/core/indexer/normalize.go` → `CanonicalizeName`
- Validación de embedding al startup:
  `src/storage/sqlite/config.go` → `ValidateEmbeddingConfig`

## Próximo paso si se aprueba

Ejecutar Spec Kitty mission `hsme-cli-tool` con tipo `feature`:

1. `/spec-kitty.specify` con esta idea como input.
2. `/spec-kitty.plan` para arquitectura final (decidir refactor de
   bootstrap o no).
3. `/spec-kitty.tasks` para WPs.
4. `/spec-kitty.implement`.
