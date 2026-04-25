set shell := ["sh", "-c"]

# Configuración de compilación (Sincronizado con Makefile)
export CGO_ENABLED := "1"
GO_TAGS := "sqlite_fts5 sqlite_vec"
INSTALL_PATH := home_dir() + "/go/bin"

# Rutas de datos
PROJECT_ROOT := invocation_directory()
DB_DIR := PROJECT_ROOT + "/data"
export SQLITE_DB_PATH := DB_DIR + "/engram.db"
BACKUP_DIR := "backups"

# Compilar binarios locales
build:
        go build -tags "{{GO_TAGS}}" -o hsme ./cmd/hsme
        go build -tags "{{GO_TAGS}}" -o hsme-worker ./cmd/worker
        go build -tags "{{GO_TAGS}}" -o hsme-ops ./cmd/ops
        @echo "✅ Binarios compilados en la raíz."

# Ejecutar tests con soporte para FTS5 y Vectores
test:
        go test -v -tags "{{GO_TAGS}}" ./...

# Compilar e instalar binarios de forma global
install:
        @mkdir -p {{INSTALL_PATH}}
        go build -tags "{{GO_TAGS}}" -o {{INSTALL_PATH}}/hsme ./cmd/hsme
        go build -tags "{{GO_TAGS}}" -o {{INSTALL_PATH}}/hsme-worker ./cmd/worker
        go build -tags "{{GO_TAGS}}" -o {{INSTALL_PATH}}/hsme-ops ./cmd/ops
        @tmp_hsme="{{PROJECT_ROOT}}/.hsme.tmp" && cp {{INSTALL_PATH}}/hsme "$$tmp_hsme" && mv -f "$$tmp_hsme" {{PROJECT_ROOT}}/hsme
        @tmp_worker="{{PROJECT_ROOT}}/.hsme-worker.tmp" && cp {{INSTALL_PATH}}/hsme-worker "$$tmp_worker" && mv -f "$$tmp_worker" {{PROJECT_ROOT}}/hsme-worker
        @tmp_ops="{{PROJECT_ROOT}}/.hsme-ops.tmp" && cp {{INSTALL_PATH}}/hsme-ops "$$tmp_ops" && mv -f "$$tmp_ops" {{PROJECT_ROOT}}/hsme-ops
        @echo "✅ Binarios instalados en {{INSTALL_PATH}} y copiados a la raíz."
# Ejecutar el servidor MCP
serve:
	./hsme

# Ejecutar el worker de grafos
work:
	./hsme-worker

# Lanzar el worker en segundo plano
work-bg:
	@nohup ./hsme-worker > worker_new.log 2>&1 &
	@echo "🚀 Worker lanzado en segundo plano (tail -f worker_new.log)"

# Ejecutar el runner de observabilidad/ops
ops:
	./hsme-ops once

# Lanzar ops en modo loop
ops-loop:
	./hsme-ops loop

# Ver progreso actual (instantánea)
status:
	@WORKER_STATE=$(if pgrep -x hsme-worker >/dev/null; then echo "online"; else echo "offline"; fi); \
	STATS=$(sqlite3 -separator '|' {{SQLITE_DB_PATH}} " \
	WITH stats AS ( \
		SELECT \
			COUNT(*) AS total_tasks, \
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) AS completed, \
			SUM(CASE WHEN status = 'pending' AND attempt_count < 5 THEN 1 ELSE 0 END) AS pending_retryable, \
			SUM(CASE WHEN status = 'processing' THEN 1 ELSE 0 END) AS processing, \
			SUM(CASE WHEN status = 'failed' OR attempt_count >= 5 THEN 1 ELSE 0 END) AS blocked_failed \
		FROM async_tasks \
	) \
	SELECT \
		CASE WHEN total_tasks = 0 THEN 100.0 ELSE completed * 100.0 / total_tasks END, \
		completed, \
		pending_retryable, \
		processing, \
		blocked_failed, \
		(SELECT COUNT(*) FROM kg_nodes), \
		(SELECT COUNT(*) FROM kg_edge_evidence) \
	FROM stats;"); \
	PROGRESS=$(printf '%s' "$STATS" | cut -d'|' -f1); \
	COMPLETED=$(printf '%s' "$STATS" | cut -d'|' -f2); \
	PENDING=$(printf '%s' "$STATS" | cut -d'|' -f3); \
	PROCESSING=$(printf '%s' "$STATS" | cut -d'|' -f4); \
	BLOCKED=$(printf '%s' "$STATS" | cut -d'|' -f5); \
	NODES=$(printf '%s' "$STATS" | cut -d'|' -f6); \
	EDGES=$(printf '%s' "$STATS" | cut -d'|' -f7); \
	printf 'Progreso: %.2f%% | Worker: %s | Completado: %s | Pendientes: %s | Processing: %s | Bloqueadas: %s | Nodos: %s | Relaciones: %s\n' "$PROGRESS" "$WORKER_STATE" "$COMPLETED" "$PENDING" "$PROCESSING" "$BLOCKED" "$NODES" "$EDGES"; \
	sqlite3 {{SQLITE_DB_PATH}} " \
	SELECT printf( \
		'Detalle: retryables=%d | agotadas_max_attempts=%d | ultimo_pending=%s', \
		(SELECT COUNT(*) FROM async_tasks WHERE status = 'pending' AND attempt_count < 5), \
		(SELECT COUNT(*) FROM async_tasks WHERE status = 'failed' OR attempt_count >= 5), \
		COALESCE((SELECT printf('#%d %s mem=%d intentos=%d', id, task_type, memory_id, attempt_count) FROM async_tasks WHERE status = 'pending' ORDER BY updated_at DESC LIMIT 1), 'ninguno') \
	);"

# Monitorear progreso en tiempo real (refresco cada 2s)
watch-status:
	@watch -n 2 -c "sqlite3 {{SQLITE_DB_PATH}} \"SELECT printf('PROGRESO: %.2f%% | RESTANTE: %d | FALLANDO: %d\nNODOS: %d | RELACIONES: %d', (SELECT COUNT(*) FROM async_tasks WHERE status = 'completed') * 100.0 / (SELECT COUNT(*) FROM async_tasks), (SELECT COUNT(*) FROM async_tasks WHERE status = 'pending'), (SELECT COUNT(*) FROM async_tasks WHERE attempt_count >= 5), (SELECT COUNT(*) FROM kg_nodes), (SELECT COUNT(*) FROM kg_edge_evidence)) as status;\""

# Reencolar tareas fallidas agotadas para que el worker pueda retomarlas
retry-failed:
	@TO_RETRY=$(sqlite3 {{SQLITE_DB_PATH}} "SELECT COUNT(*) FROM async_tasks WHERE status = 'failed' OR attempt_count >= 5;"); \
	if [ "$TO_RETRY" = "0" ]; then \
		echo "ℹ️ No hay tareas fallidas/bloqueadas para reintentar."; \
		exit 0; \
	fi; \
	sqlite3 {{SQLITE_DB_PATH}} " \
	UPDATE async_tasks \
	SET status = 'pending', \
	    attempt_count = 0, \
	    leased_until = NULL, \
	    last_error = NULL, \
	    updated_at = datetime('now') \
	WHERE status = 'failed' OR attempt_count >= 5;"; \
	echo "🔁 Tareas reencoladas: $TO_RETRY"; \
	sqlite3 {{SQLITE_DB_PATH}} "SELECT printf('Estado actual: pending=%d | failed=%d', (SELECT COUNT(*) FROM async_tasks WHERE status = 'pending'), (SELECT COUNT(*) FROM async_tasks WHERE status = 'failed'));"

# Realizar un backup ATÓMICO (Compatible con WAL)
backup:
	@mkdir -p {{BACKUP_DIR}}
	@sqlite3 {{SQLITE_DB_PATH}} ".backup '{{BACKUP_DIR}}/engram_$(date +%Y%m%d_%H%M%S).db'"
	@echo "✅ Backup guardado en {{BACKUP_DIR}}/"

# Restaurar base de datos
restore:
	@LATEST=$(ls -t {{BACKUP_DIR}}/engram_*.db 2>/dev/null | head -1); \
	if [ -z "$$LATEST" ]; then echo "❌ No hay backups"; exit 1; fi; \
	sqlite3 $$LATEST "PRAGMA integrity_check;" | grep -q "ok" || (echo "❌ Backup corrupto"; exit 1); \
	rm -f {{SQLITE_DB_PATH}}-wal {{SQLITE_DB_PATH}}-shm; \
	cp "$$LATEST" {{SQLITE_DB_PATH}}; \
	echo "✅ Restaurado en {{SQLITE_DB_PATH}}"

# Limpiar binarios locales
clean:
	rm -f hsme hsme-worker hsme-ops worker_new.log
