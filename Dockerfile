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

# CGO apagado: así el binario no depende de ninguna biblioteca del sistema y
# puede correr sobre una imagen sin nada dentro.
# -trimpath quita las rutas de compilación; -s -w quitan la tabla de símbolos.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /api ./cmd/api

# ── 3. Ejecución ────────────────────────────────────────────────────────────
# distroless: solo los certificados raíz y la zona horaria. Sin shell, sin
# gestor de paquetes y sin usuario root, que es menos superficie de ataque y
# una imagen de unos pocos megas.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=servidor /api /api

ENV PORT=8080 APP_ENV=production TZ=America/Bogota
EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/api"]
