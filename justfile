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

# Compilar e instalar binarios de forma global
install:
	@mkdir -p {{INSTALL_PATH}}
	go build -tags "{{GO_TAGS}}" -o {{INSTALL_PATH}}/hsme ./cmd/hsme
	go build -tags "{{GO_TAGS}}" -o {{INSTALL_PATH}}/hsme-worker ./cmd/worker
	@tmp_hsme="{{PROJECT_ROOT}}/.hsme.tmp" && cp {{INSTALL_PATH}}/hsme "$$tmp_hsme" && mv -f "$$tmp_hsme" {{PROJECT_ROOT}}/hsme
	@tmp_worker="{{PROJECT_ROOT}}/.hsme-worker.tmp" && cp {{INSTALL_PATH}}/hsme-worker "$$tmp_worker" && mv -f "$$tmp_worker" {{PROJECT_ROOT}}/hsme-worker
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

# Ver progreso actual (instantánea)
status:
	@sqlite3 {{SQLITE_DB_PATH}} "SELECT printf('Progreso: %.2f%% | Completado: %d | Restante: %d | Fallando: %d | Nodos: %d | Relaciones: %d', (SELECT COUNT(*) FROM async_tasks WHERE status = 'completed') * 100.0 / (SELECT COUNT(*) FROM async_tasks), (SELECT COUNT(*) FROM async_tasks WHERE status = 'completed'), (SELECT COUNT(*) FROM async_tasks WHERE status = 'pending'), (SELECT COUNT(*) FROM async_tasks WHERE attempt_count >= 5), (SELECT COUNT(*) FROM kg_nodes), (SELECT COUNT(*) FROM kg_edge_evidence)) as status;"

# Monitorear progreso en tiempo real (refresco cada 2s)
watch-status:
	@watch -n 2 -c "sqlite3 {{SQLITE_DB_PATH}} \"SELECT printf('PROGRESO: %.2f%% | RESTANTE: %d | FALLANDO: %d\nNODOS: %d | RELACIONES: %d', (SELECT COUNT(*) FROM async_tasks WHERE status = 'completed') * 100.0 / (SELECT COUNT(*) FROM async_tasks), (SELECT COUNT(*) FROM async_tasks WHERE status = 'pending'), (SELECT COUNT(*) FROM async_tasks WHERE attempt_count >= 5), (SELECT COUNT(*) FROM kg_nodes), (SELECT COUNT(*) FROM kg_edge_evidence)) as status;\""

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
	rm -f hsme hsme-worker worker_new.log
