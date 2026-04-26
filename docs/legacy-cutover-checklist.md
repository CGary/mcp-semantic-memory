# Checklist de Cutover: Migración de Engram a HSME

Esta guía detalla los pasos exactos para completar la migración del corpus de memoria desde el sistema legado Engram hacia HSME.

## T-minus 10 min: Preparación y Validación
1. [ ] Asegurar que Ollama está corriendo (`nomic-embed-text` disponible).
2. [ ] Validar conectividad con ambas bases de datos:
   ```bash
   just migrate dry-run
   ```
3. [ ] Revisar el reporte en `data/migrations/<RUN_ID>-dryrun/report.txt`.

## T-0: Migración Inicial (Full)
1. [ ] Ejecutar la migración completa (esto sincroniza metadatos e ingiere huérfanos):
   ```bash
   just migrate full
   ```
2. [ ] Verificar que no haya errores fatales en la salida.
3. [ ] Ejecutar el worker para procesar los nuevos registros:
   ```bash
   just work
   ```

## T+1 min: Corte de Escritura (Cutover)
1. [ ] **ACCIÓN CRÍTICA**: Eliminar el servidor MCP `engram` de la configuración de Claude Code (o el cliente que estés usando).
2. [ ] Reiniciar Claude Code para asegurar que no haya sesiones activas escribiendo a Engram legado.

## T+2 min: Replay de Delta
Si hubo escrituras entre el inicio de la fase "Full" y el corte real, este paso las captura:
1. [ ] Ejecutar el replay delta:
   ```bash
   just migrate delta
   ```

## T+5 min: Verificación Final
1. [ ] Ejecutar el script de comparación:
   ```bash
   just verify-cutover
   ```
2. [ ] Confirmar que `migration_tags_remaining` es 0 (o un número muy bajo si se esperan resúmenes de sesión).
3. [ ] Confirmar que `hsme_active` >= `legacy_active`.

## T+24h: Auditoría de No-Escritura
1. [ ] Volver a ejecutar `just verify-cutover`.
2. [ ] Confirmar que `legacy_active` no ha cambiado desde T+5m.

---
**Handoff**: A partir de este momento, `data/engram.db` (en HSME) es la única fuente de verdad. El archivo en `~/.engram/engram.db` puede conservarse como backup histórico frío.
