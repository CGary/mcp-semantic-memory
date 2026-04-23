#!/bin/bash

# Detectar la raíz del proyecto (donde está el script)
PROJECT_ROOT=$(cd "$(dirname "$0")/.." && pwd)
SOURCE_DB="$PROJECT_ROOT/data/engram.db"
BACKUP_ROOT="$HOME/hsme/backups"
TIMESTAMP=$(date +"%Y%m%d-%H%M")
TARGET_DIR="$BACKUP_ROOT/$TIMESTAMP"
TARGET_FILE="$TARGET_DIR/engram.db"

echo "🚀 Iniciando backup universal de HSME..."

# Verificar dependencias
if ! command -v sqlite3 &> /dev/null; then
    echo "❌ Error: sqlite3 no está instalado."
    exit 1
fi

# Verificar origen
if [ ! -f "$SOURCE_DB" ]; then
    echo "❌ Error: No se encontró la base de datos en $SOURCE_DB"
    exit 1
fi

mkdir -p "$TARGET_DIR"

# Backup atómico
sqlite3 "$SOURCE_DB" ".backup '$TARGET_FILE'"

if [ $? -eq 0 ]; then
    echo "✅ Backup completado en: $TARGET_DIR"
    echo "📦 Info: Este archivo único contiene todos los datos (incluyendo transacciones WAL)."
else
    echo "❌ Error al realizar el backup."
    exit 1
fi
