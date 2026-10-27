package deploygen

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// GenerarDockerfile escribe un Dockerfile a medida de lo detectado.
//
// Es puro: la misma detección produce siempre el mismo texto. La idea no es
// acertar al cien por cien sino dar un punto de partida que funcione, que
// luego se edita antes de desplegar.
func GenerarDockerfile(d Deteccion) string {
	switch d.Runtime {
	case RuntimeNode:
		if d.IsStatic {
			return nodeEstatico(d)
		}
		return nodeServidor(d)
	case RuntimePython:
		return python(d)
	case RuntimeGo:
		return golang(d)
	case RuntimePHP:
		return php(d)
	case RuntimeRuby:
		return ruby(d)
	case RuntimeJava:
		return java(d)
	case RuntimeStatic:
		return estatico()
	default:
		return generico(d)
	}
}

var reMayor = regexp.MustCompile(`(\d{2})`)

// imagenNode saca el número mayor de un rango como ">=20" o "20.x".
func imagenNode(version string) string {
	mayor := "20"
	if m := reMayor.FindStringSubmatch(version); m != nil {
		mayor = m[1]
	}
	return "node:" + mayor + "-slim"
}

// corepack hace falta para pnpm y yarn; npm y bun no lo necesitan.
func corepack(gestor string) string {
	if gestor == "pnpm" || gestor == "yarn" {
		return "RUN corepack enable"
	}
	return ""
}

func nodeServidor(d Deteccion) string {
	img := imagenNode(d.RuntimeVersion)

	construir := "# (este proyecto no tiene build)"
	if d.BuildCommand != "" {
		construir = "RUN " + ejecutar(d.PackageManager, "build")
	}

	arranque := d.StartCommand
	if arranque == "" {
		arranque = ejecutar(d.PackageManager, "start")
	}

	return fmt.Sprintf(`# Generado por Home Lab · %s
FROM %s AS build
WORKDIR /app
%s
COPY package*.json ./
RUN %s
COPY . .
%s

FROM %s
WORKDIR /app
ENV NODE_ENV=production
%s
COPY --from=build /app ./
EXPOSE %d
CMD %s
`, nombreOEtiqueta(d.Framework, "node"), img, corepack(d.PackageManager),
		comandoDeInstalacion(d.PackageManager), construir, img,
		corepack(d.PackageManager), d.Port, comoArray(arranque))
}

func nodeEstatico(d Deteccion) string {
	img := imagenNode(d.RuntimeVersion)
	dir := d.PublishDir
	if dir == "" {
		dir = "dist"
	}

	return fmt.Sprintf(`# Generado por Home Lab · %s (compilado y servido con nginx)
FROM %s AS build
WORKDIR /app
%s
COPY package*.json ./
RUN %s
COPY . .
RUN %s

FROM nginx:alpine
COPY --from=build /app/%s /usr/share/nginx/html
EXPOSE 80
`, nombreOEtiqueta(d.Framework, "estático"), img, corepack(d.PackageManager),
		comandoDeInstalacion(d.PackageManager), ejecutar(d.PackageManager, "build"), dir)
}

func python(d Deteccion) string {
	copiar := "COPY requirements.txt ./"
	instalar := "RUN pip install --no-cache-dir -r requirements.txt"
	if d.PackageManager == "poetry" {
		copiar = "COPY pyproject.toml poetry.lock* ./"
		instalar = "RUN pip install poetry && poetry config virtualenvs.create false && " +
			"poetry install --no-root --without dev"
	}

	construir := ""
	if d.BuildCommand != "" {
		construir = "RUN " + d.BuildCommand
	}

	arranque := d.StartCommand
	if arranque == "" {
		arranque = "python main.py"
	}

	return fmt.Sprintf(`# Generado por Home Lab · %s
FROM python:3.12-slim
WORKDIR /app
ENV PYTHONUNBUFFERED=1
%s
%s
COPY . .
%s
EXPOSE %d
CMD %s
`, nombreOEtiqueta(d.Framework, "python"), copiar, instalar, construir, d.Port, comoArray(arranque))
}

func golang(d Deteccion) string {
	return fmt.Sprintf(`# Generado por Home Lab · go
FROM golang:1.25 AS build
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /server ./...

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /server /server
EXPOSE %d
USER nonroot:nonroot
CMD ["/server"]
`, d.Port)
}

func php(d Deteccion) string {
	arranque := d.StartCommand
	if arranque == "" {
		arranque = "php -S 0.0.0.0:8000 -t public"
	}

	return fmt.Sprintf(`# Generado por Home Lab · %s
FROM php:8.3-cli
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends unzip libpq-dev \
  && docker-php-ext-install pdo pdo_pgsql \
  && rm -rf /var/lib/apt/lists/*
COPY --from=composer:2 /usr/bin/composer /usr/bin/composer
COPY composer.json composer.lock* ./
RUN composer install --no-dev --optimize-autoloader --no-scripts
COPY . .
EXPOSE %d
CMD %s
`, nombreOEtiqueta(d.Framework, "php"), d.Port, comoArray(arranque))
}

func ruby(d Deteccion) string {
	construir := ""
	if d.BuildCommand != "" {
		construir = "RUN " + d.BuildCommand
	}

	arranque := d.StartCommand
	if arranque == "" {
		arranque = "bundle exec rackup -o 0.0.0.0 -p 3000"
	}

	return fmt.Sprintf(`# Generado por Home Lab · %s
FROM ruby:3.3-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends build-essential libpq-dev \
  && rm -rf /var/lib/apt/lists/*
COPY Gemfile Gemfile.lock* ./
RUN bundle install
COPY . .
%s
EXPOSE %d
CMD %s
`, nombreOEtiqueta(d.Framework, "ruby"), construir, d.Port, comoArray(arranque))
}

func java(d Deteccion) string {
	etapa := `FROM maven:3.9-eclipse-temurin-21 AS build
WORKDIR /app
COPY . .
RUN mvn -q package -DskipTests`
	artefacto := "target/*.jar"
	gestor := "maven"

	if d.PackageManager == "gradle" {
		etapa = `FROM gradle:8-jdk21 AS build
WORKDIR /app
COPY . .
RUN gradle build -x test --no-daemon`
		artefacto = "build/libs/*.jar"
		gestor = "gradle"
	}

	return fmt.Sprintf(`# Generado por Home Lab · java (%s)
%s

FROM eclipse-temurin:21-jre
WORKDIR /app
COPY --from=build /app/%s app.jar
EXPOSE %d
CMD ["java", "-jar", "app.jar"]
`, gestor, etapa, artefacto, d.Port)
}

func estatico() string {
	return `# Generado por Home Lab · sitio estático
FROM nginx:alpine
COPY . /usr/share/nginx/html
EXPOSE 80
`
}

func generico(d Deteccion) string {
	return fmt.Sprintf(`# Generado por Home Lab · genérico — REVÍSALO ANTES DE USARLO
#
# No se pudo deducir con qué está hecho el proyecto, así que este Dockerfile
# no arranca nada: hay que rellenar la instalación, la compilación y el
# arranque a mano.
FROM debian:stable-slim
WORKDIR /app
COPY . .
EXPOSE %d
CMD ["sleep", "infinity"]
`, d.Port)
}

func nombreOEtiqueta(framework, porDefecto string) string {
	if framework == "" {
		return porDefecto
	}
	return framework
}

// comoArray pasa un comando de shell a la forma exec de CMD, que es la que
// hace que la señal de parada llegue al proceso en vez de a un shell.
func comoArray(comando string) string {
	partes := strings.Fields(comando)

	trozos := make([]string, 0, len(partes))
	for _, p := range partes {
		// json.Marshal se encarga de las comillas y los escapes.
		b, _ := json.Marshal(p)
		trozos = append(trozos, string(b))
	}

	return "[" + strings.Join(trozos, ", ") + "]"
}

// ── docker-compose ─────────────────────────────────────────────────────────

type imagenBase struct {
	imagen  string
	entorno string
	volumen string
}

var imagenesBase = map[BaseDatos]imagenBase{
	Postgres: {
		imagen: "postgres:16-alpine",
		entorno: `      POSTGRES_USER: app
      POSTGRES_PASSWORD: app
      POSTGRES_DB: app`,
		volumen: "/var/lib/postgresql/data",
	},
	MySQL: {
		imagen: "mysql:8",
		entorno: `      MYSQL_ROOT_PASSWORD: app
      MYSQL_DATABASE: app
      MYSQL_USER: app
      MYSQL_PASSWORD: app`,
		volumen: "/var/lib/mysql",
	},
	MongoDB: {imagen: "mongo:7", volumen: "/data/db"},
	Redis:   {imagen: "redis:7-alpine", volumen: "/data"},
}

func urlDeBase(db BaseDatos) string {
	switch db {
	case Postgres:
		return "postgresql://app:app@db:5432/app"
	case MySQL:
		return "mysql://app:app@db:3306/app"
	case MongoDB:
		return "mongodb://db:27017/app"
	case Redis:
		return "redis://db:6379"
	}
	return ""
}

// GenerarCompose escribe un compose con la aplicación y su base de datos.
// Devuelve cadena vacía si no hace falta base: entonces basta el Dockerfile.
func GenerarCompose(d Deteccion) string {
	if !d.NeedsDatabase || d.DatabaseType == SinBase {
		return ""
	}

	base, hay := imagenesBase[d.DatabaseType]
	if !hay {
		return ""
	}

	entorno := ""
	if base.entorno != "" {
		entorno = "\n    environment:\n" + base.entorno
	}

	puerto := strconv.Itoa(d.Port)

	return fmt.Sprintf(`# Generado por Home Lab · aplicación + %s
services:
  app:
    build: .
    restart: unless-stopped
    environment:
      DATABASE_URL: %s
    ports:
      - "%s:%s"
    depends_on:
      - db

  db:
    image: %s
    restart: unless-stopped%s
    volumes:
      - db-data:%s

volumes:
  db-data:
`, d.DatabaseType, urlDeBase(d.DatabaseType), puerto, puerto,
		base.imagen, entorno, base.volumen)
}

// PlantillaDeBase devuelve un compose suelto para una sola base de datos,
// usando las variables mágicas de Coolify (SERVICE_*) para que genere él las
// credenciales, igual que hace con sus bases de un clic.
func PlantillaDeBase(tipo string) string {
	switch tipo {
	case "postgresql":
		return `services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: ${SERVICE_USER_POSTGRES:-app}
      POSTGRES_PASSWORD: ${SERVICE_PASSWORD_POSTGRES}
      POSTGRES_DB: ${POSTGRES_DB:-app}
    volumes:
      - pg-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U app"]
      interval: 10s
      timeout: 5s
      retries: 5
volumes:
  pg-data:
`
	case "mysql", "mariadb":
		servicio, imagen := "mysql", "mysql:8"
		if tipo == "mariadb" {
			servicio, imagen = "mariadb", "mariadb:11"
		}
		return fmt.Sprintf(`services:
  %s:
    image: %s
    restart: unless-stopped
    environment:
      MYSQL_ROOT_PASSWORD: ${SERVICE_PASSWORD_MYSQLROOT}
      MYSQL_DATABASE: ${MYSQL_DATABASE:-app}
      MYSQL_USER: ${SERVICE_USER_MYSQL:-app}
      MYSQL_PASSWORD: ${SERVICE_PASSWORD_MYSQL}
    volumes:
      - db-data:/var/lib/mysql
volumes:
  db-data:
`, servicio, imagen)

	case "mongodb":
		return `services:
  mongo:
    image: mongo:7
    restart: unless-stopped
    environment:
      MONGO_INITDB_ROOT_USERNAME: ${SERVICE_USER_MONGO:-app}
      MONGO_INITDB_ROOT_PASSWORD: ${SERVICE_PASSWORD_MONGO}
    volumes:
      - mongo-data:/data/db
volumes:
  mongo-data:
`
	case "redis":
		return `services:
  redis:
    image: redis:7-alpine
    restart: unless-stopped
    command: ["redis-server", "--requirepass", "${SERVICE_PASSWORD_REDIS}"]
    volumes:
      - redis-data:/data
volumes:
  redis-data:
`
	}

	return `services:
  app:
    image: nginx:alpine
    restart: unless-stopped
`
}
