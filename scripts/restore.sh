#!/bin/bash

# Detectar la raíz del proyecto
PROJECT_ROOT=$(cd "$(dirname "$0")/.." && pwd)
TARGET_DIR="$PROJECT_ROOT/data"
BACKUP_ROOT="$HOME/hsme/backups"

echo "🔄 Iniciando proceso de restauración de HSME..."

# 1. Listar backups disponibles
if [ ! -d "$BACKUP_ROOT" ]; then
    echo "❌ No se encontró la carpeta de backups en $BACKUP_ROOT"
    exit 1
fi

echo "Backups disponibles:"
select BACKUP_DATE in $(ls -1 "$BACKUP_ROOT" | sort -r); do
    if [ -n "$BACKUP_DATE" ]; then
        SOURCE_FILE="$BACKUP_ROOT/$BACKUP_DATE/engram.db"
        break
    else
        echo "Selección no válida."
    fi
done

# 2. Confirmación y limpieza
echo "⚠️  Esto sobrescribirá la base de datos actual. ¿Continuar? (s/n)"
read -r CONFIRM
if [[ ! "$CONFIRM" =~ ^[sS]$ ]]; then
    echo "Operación cancelada."
    exit 0
fi

mkdir -p "$TARGET_DIR"

# 3. Borrar archivos actuales (incluyendo temporales WAL/SHM para evitar corrupción)
echo "🧹 Limpiando base de datos actual..."
rm -f "$TARGET_DIR"/engram.db*

# 4. Restaurar
echo "🚚 Restaurando backup de $BACKUP_DATE..."
cp "$SOURCE_FILE" "$TARGET_DIR/engram.db"

if [ $? -eq 0 ]; then
    echo "✅ Restauración exitosa."
    echo "💡 Nota: Los archivos .db-wal y .db-shm se regenerarán automáticamente al iniciar el servidor."
else
    echo "❌ Falló la copia del archivo."
    exit 1
fi
