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

# Consulta SQL para obtener métricas
STATS=$(sqlite3 -batch -noheader "$DB_PATH" "
    SELECT 
        COUNT(*),
        SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END),
        SUM(CASE WHEN status = 'pending' AND attempt_count < 5 THEN 1 ELSE 0 END),
        SUM(CASE WHEN status = 'processing' THEN 1 ELSE 0 END),
        SUM(CASE WHEN status = 'failed' OR attempt_count >= 5 THEN 1 ELSE 0 END)
    FROM async_tasks;")

TOTAL=$(echo $STATS | cut -d'|' -f1)
DONE=$(echo $STATS | cut -d'|' -f2)
RETRY=$(echo $STATS | cut -d'|' -f3)
PROC=$(echo $STATS | cut -d'|' -f4)
FAIL=$(echo $STATS | cut -d'|' -f5)

# Evitar división por cero
TOTAL=${TOTAL:-0}
DONE=${DONE:-0}
RETRY=${RETRY:-0}
PROC=${PROC:-0}
FAIL=${FAIL:-0}

NODES=$(sqlite3 -batch -noheader "$DB_PATH" "SELECT COUNT(*) FROM kg_nodes;")
EDGES=$(sqlite3 -batch -noheader "$DB_PATH" "SELECT COUNT(*) FROM kg_edge_evidence;")

# Calcular porcentaje y barra
if [ "$TOTAL" -eq 0 ]; then 
    PERCENT="100.00"; 
    DONE_BAR=20; 
else
    # Usar awk para aritmética de punto flotante si está disponible, sino bash
    PERCENT=$(awk "BEGIN {printf \"%.2f\", ($DONE * 100 / $TOTAL)}" 2>/dev/null || echo "$((DONE * 100 / TOTAL)).00")
    DONE_BAR=$(awk "BEGIN {print int($DONE * 20 / $TOTAL)}" 2>/dev/null || echo "$((DONE * 20 / TOTAL))")
fi
EMPTY_BAR=$((20 - $DONE_BAR))

BAR="["
for i in $(seq 1 $DONE_BAR); do BAR="${BAR}#"; done
for i in $(seq 1 $EMPTY_BAR); do BAR="${BAR} "; done
BAR="${BAR}]"

# Renderizado del Dashboard
echo -e "╔══════════════════════════════════════════════════════════════════════════════╗"
echo -e "║  ${BOLD}HSME STATUS DASHBOARD${RESET}                                 Worker: ${WORKER_STATE}   ║"
echo -e "╠══════════════════════════════════════════════════════════════════════════════╣"
echo -e "║  [ ${CYAN}SEMÁNTICA & INFERENCIA${RESET} ]                                                  ║"
printf "║  %-20s  %6s%%                                 ║\n" "$BAR" "$PERCENT"
echo -e "╠══════════════════════════════════════════════════════════════════════════════╣"
echo -e "║  [ COLA DE TAREAS ]                        [ GRAFO DE CONOCIMIENTO ]         ║"
printf "║  ✅ Completadas: %-10d                🚀 Nodos: %-10d              ║\n" "$DONE" "$NODES"
printf "║  ⏳ Pendientes:  %-10d                🔗 Relaciones: %-10d         ║\n" "$RETRY" "$EDGES"
printf "║  ⚙️ Procesando:  %-10d                                               ║\n" "$PROC"
printf "║  ❌ Bloqueadas:  %-10d                                               ║\n" "$FAIL"
echo -e "╠══════════════════════════════════════════════════════════════════════════════╣"
LAST_PENDING=$(sqlite3 -batch -noheader "$DB_PATH" "SELECT COALESCE((SELECT printf('#%d %s (Mem: %d, Int: %d)', id, task_type, memory_id, attempt_count) FROM async_tasks WHERE status = 'pending' ORDER BY updated_at DESC LIMIT 1), 'Ninguna (Cola vacía)');")
echo -e "║  Última Tarea Pendiente: ${YELLOW}$(printf "%-51s" "$LAST_PENDING")${RESET} ║"
echo -e "╚══════════════════════════════════════════════════════════════════════════════╝"
