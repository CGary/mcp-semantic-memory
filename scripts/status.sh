#!/bin/bash
# scripts/status.sh - Dashboard de salud y progreso para HSME

DB_PATH=${SQLITE_DB_PATH:-"data/engram.db"}

# Colores ANSI
GREEN='\033[32m'
RED='\033[31m'
CYAN='\033[36m'
YELLOW='\033[33m'
RESET='\033[0m'
BOLD='\033[1m'

# Estado del worker
if pgrep -x hsme-worker >/dev/null; then
    WORKER_STATE="${GREEN}ONLINE${RESET}"
else
    WORKER_STATE="${RED}OFFLINE${RESET}"
fi

# Consulta SQL unificada
SQL_QUERY="
WITH stats AS (
    SELECT 
        COUNT(*) AS total,
        SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) AS done,
        SUM(CASE WHEN status = 'pending' AND attempt_count < 5 THEN 1 ELSE 0 END) AS retry,
        SUM(CASE WHEN status = 'processing' THEN 1 ELSE 0 END) AS proc,
        SUM(CASE WHEN status = 'failed' OR attempt_count >= 5 THEN 1 ELSE 0 END) AS fail
    FROM async_tasks
)
SELECT printf(
    '║  %-20s  %6.2f%%                                 ║\n' ||
    '╠══════════════════════════════════════════════════════════════════════════════╣\n' ||
    '║  [ COLA DE TAREAS ]                        [ GRAFO DE CONOCIMIENTO ]         ║\n' ||
    '║  ✅ Completadas: %-10d                🚀 Nodos: %-10d              ║\n' ||
    '║  ⏳ Pendientes:  %-10d                🔗 Relaciones: %-10d         ║\n' ||
    '║  ⚙️ Procesando:  %-10d                                               ║\n' ||
    '║  ❌ Bloqueadas:  %-10d                                               ║\n',
    '[' || substr('####################', 1, CAST((done * 20.0 / CASE WHEN total=0 THEN 1 ELSE total END) AS INT)) || substr('                    ', 1, 20 - CAST((done * 20.0 / CASE WHEN total=0 THEN 1 ELSE total END) AS INT)) || ']',
    CASE WHEN total=0 THEN 100.0 ELSE done * 100.0 / total END,
    done,
    (SELECT COUNT(*) FROM kg_nodes),
    retry,
    (SELECT COUNT(*) FROM kg_edge_evidence),
    proc,
    fail
) FROM stats;"

# Obtener última tarea pendiente por separado para evitar problemas de escape en el printf gigante
LAST_PENDING=$(sqlite3 "$DB_PATH" "SELECT COALESCE((SELECT printf('#%d %s (Mem: %d, Int: %d)', id, task_type, memory_id, attempt_count) FROM async_tasks WHERE status = 'pending' ORDER BY updated_at DESC LIMIT 1), 'Ninguna (Cola vacía)');")

# Renderizado del Dashboard
echo -e "╔══════════════════════════════════════════════════════════════════════════════╗"
echo -e "║  ${BOLD}HSME STATUS DASHBOARD${RESET}                                 Worker: ${WORKER_STATE}   ║"
echo -e "╠══════════════════════════════════════════════════════════════════════════════╣"
echo -e "║  [ ${CYAN}SEMÁNTICA & INFERENCIA${RESET} ]                                                  ║"
sqlite3 "$DB_PATH" "$SQL_QUERY"
echo -e "╠══════════════════════════════════════════════════════════════════════════════╣"
echo -e "║  Última Tarea Pendiente: ${YELLOW}$(printf "%-51s" "$LAST_PENDING")${RESET} ║"
echo -e "╚══════════════════════════════════════════════════════════════════════════════╝"
