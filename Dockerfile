# syntax=docker/dockerfile:1

# Un solo contenedor con todo: la interfaz de React se compila, se incrusta en
# el binario de Go y se sirve desde él. No hay Node en producción ni CORS que
# configurar, porque API e interfaz comparten origen.

# ── 1. La interfaz ──────────────────────────────────────────────────────────
# Debian slim y no Alpine, por la misma razón que el Lab: Vite y sus
# dependencias traen binarios nativos compilados contra glibc, y en musl
# fallan al cargar.
FROM node:24-slim AS interfaz
WORKDIR /app

# Las dependencias primero, en su propia capa: cambian mucho menos que el
# código, así que la reconstrucción habitual se las salta.
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# ── 2. El binario ───────────────────────────────────────────────────────────
FROM golang:1.27-bookworm AS servidor
WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
# La interfaz ya compilada entra donde la espera go:embed.
COPY --from=interfaz /app/dist ./internal/web/dist

# CGO apagado: así el binario queda estático y no depende de la libc del
# sistema, que es lo que permite compilarlo sobre Debian y ejecutarlo sobre
# Alpine, que usa musl.
# -trimpath quita las rutas de compilación; -s -w quitan la tabla de símbolos.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /api ./cmd/api

# ── 3. Ejecución ────────────────────────────────────────────────────────────
# Alpine y no distroless, aunque distroless sea más pequeño y no traiga shell:
# Coolify comprueba la salud del contenedor con `docker exec /bin/sh -c wget…`,
# y sobre una imagen sin shell eso falla siempre, así que el despliegue nunca
# llega a darse por bueno. Vale más tener la comprobación funcionando —es la
# que evita que un contenedor roto reciba tráfico— que ahorrar unos megas.
FROM alpine:3.22

# ca-certificates para hablar con HTTPS (el monitor de uptime y Telegram);
# tzdata para que las horas que se guardan sean las de Bogotá; wget ya viene
# en busybox y es lo que usa la comprobación de salud.
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -H -u 10001 api

COPY --from=servidor /api /api

ENV PORT=8080 APP_ENV=production TZ=America/Bogota
EXPOSE 8080
USER api

HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/api"]
