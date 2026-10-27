#!/usr/bin/env bash
#
# Prepara el backend para hablar con la MISMA base que usa el Lab en producción.
#
# Abre un túnel SSH hasta el PostgreSQL del servidor y escribe backend/.env con
# la cadena de conexión y el secreto de sesión del propio Lab, de modo que una
# sesión abierta allí valga aquí.
#
# Las credenciales se leen del servidor en el momento y se escriben directas al
# archivo: no se muestran por pantalla ni quedan en el historial del shell.
#
#   Uso:  ./scripts/conectar-local.sh
#   Parar el túnel:  ./scripts/conectar-local.sh --parar

set -euo pipefail

SERVIDOR="coolify"                                   # alias de ~/.ssh/config
CONTENEDOR_PG="postgres-db-z4439oqrbhktaslf29yhmwyq"
CONTENEDOR_LAB="lab-r135b2kfijcbwqel909vx8lh-183632405987"
PUERTO_LOCAL=5433
PUERTO_REMOTO=5433                                   # el que publica el contenedor

RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DESTINO="$RAIZ/.env"

parar_tunel() {
  # -O exit cierra solo el túnel, no otras sesiones SSH.
  pkill -f "ssh.*-L ${PUERTO_LOCAL}:localhost:${PUERTO_REMOTO}.*${SERVIDOR}" 2>/dev/null || true
  echo "Túnel cerrado."
}

if [[ "${1:-}" == "--parar" ]]; then
  parar_tunel
  exit 0
fi

# ── 1. Túnel ────────────────────────────────────────────────────────────────
if nc -z localhost "$PUERTO_LOCAL" 2>/dev/null; then
  echo "· El puerto $PUERTO_LOCAL ya está ocupado; se reutiliza."
else
  echo "· Abriendo túnel $PUERTO_LOCAL → $SERVIDOR:$PUERTO_REMOTO…"
  ssh -f -N -L "${PUERTO_LOCAL}:localhost:${PUERTO_REMOTO}" "$SERVIDOR"
  sleep 1
fi

# ── 2. Credenciales ─────────────────────────────────────────────────────────
echo "· Leyendo la configuración del servidor…"

USUARIO=$(ssh "$SERVIDOR" "docker exec $CONTENEDOR_PG printenv POSTGRES_USER")
CLAVE=$(ssh "$SERVIDOR"   "docker exec $CONTENEDOR_PG printenv POSTGRES_PASSWORD")
SECRETO=$(ssh "$SERVIDOR" "docker exec $CONTENEDOR_LAB printenv SESSION_SECRET")

if [[ -z "$CLAVE" || -z "$SECRETO" ]]; then
  echo "No se pudo leer la configuración del servidor." >&2
  exit 1
fi

# ── 3. Escribir .env ────────────────────────────────────────────────────────
umask 077   # el archivo nace solo legible por su dueño
cat > "$DESTINO" <<EOF
# Generado por scripts/conectar-local.sh — NO subir a git.
#
# Apunta, a través del túnel SSH, a la misma base que usa el Lab en producción.
# search_path=lab es el equivalente en lib/pq al ?schema=lab de Prisma.
DATABASE_URL="postgres://${USUARIO}:${CLAVE}@localhost:${PUERTO_LOCAL}/master_db?sslmode=disable&search_path=lab"

# El mismo secreto del Lab: así las sesiones valen en las dos aplicaciones.
SESSION_SECRET="${SECRETO}"

PORT=8080
APP_ENV=development
ALLOWED_ORIGINS="http://localhost:5173"
EOF

echo "· Escrito $DESTINO (solo lectura para ti)."
echo
echo "Listo. Ahora:"
echo "    cd $RAIZ && go run ./cmd/api"
echo
echo "CUIDADO: esto apunta a PRODUCCIÓN. Las altas, bajas y el botón"
echo "«Comprobar ahora» escriben en la base real."
