// Package deploygen mira un repositorio, deduce con qué está hecho y escribe
// un Dockerfile para desplegarlo.
package deploygen

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

type Runtime string

const (
	RuntimeNode    Runtime = "node"
	RuntimePython  Runtime = "python"
	RuntimeGo      Runtime = "go"
	RuntimePHP     Runtime = "php"
	RuntimeRuby    Runtime = "ruby"
	RuntimeJava    Runtime = "java"
	RuntimeStatic  Runtime = "static"
	RuntimeUnknown Runtime = "unknown"
)

type BuildPack string

const (
	PackNixpacks   BuildPack = "nixpacks"
	PackDockerfile BuildPack = "dockerfile"
	PackStatic     BuildPack = "static"
)

type BaseDatos string

const (
	Postgres BaseDatos = "postgresql"
	MySQL    BaseDatos = "mysql"
	MongoDB  BaseDatos = "mongodb"
	Redis    BaseDatos = "redis"
	SinBase  BaseDatos = ""
)

// Archivos es lo que se ha podido leer del repositorio: la lista de rutas y
// el contenido de los manifiestos que interesan.
type Archivos struct {
	Rutas      []string
	Contenidos map[string]string // la clave va en minúsculas
}

// Deteccion es todo lo que se ha deducido del repositorio.
type Deteccion struct {
	Runtime        Runtime
	Framework      string
	PackageManager string
	RuntimeVersion string
	InstallCommand string
	BuildCommand   string
	StartCommand   string
	Port           int
	IsStatic       bool
	PublishDir     string
	NeedsDatabase  bool
	DatabaseType   BaseDatos
	HasDockerfile  bool
	HasCompose     bool
	BuildPack      BuildPack
	// Avisos para quien lo lea: lo que hace falta ajustar a mano.
	Notas []string
}

func (a Archivos) tiene(nombre string) bool {
	bajo := strings.ToLower(nombre)
	if _, hay := a.Contenidos[bajo]; hay {
		return true
	}
	for _, r := range a.Rutas {
		if strings.ToLower(r) == bajo {
			return true
		}
	}
	return false
}

func (a Archivos) leer(nombre string) string {
	return a.Contenidos[strings.ToLower(nombre)]
}

// Detectar es el punto de entrada: a partir de los archivos, deduce el stack.
//
// Es determinista y no toca la red: los mismos archivos dan siempre el mismo
// resultado, que es lo que permite probarlo a fondo.
func Detectar(a Archivos) Deteccion {
	base := Deteccion{
		Runtime:       RuntimeUnknown,
		Port:          3000,
		DatabaseType:  SinBase,
		HasDockerfile: a.tiene("Dockerfile"),
		HasCompose: a.tiene("docker-compose.yml") ||
			a.tiene("docker-compose.yaml") || a.tiene("compose.yml"),
		BuildPack: PackNixpacks,
		Notas:     []string{},
	}

	if d, hay := detectarNode(a, base); hay {
		return d
	}
	if d, hay := detectarPython(a, base); hay {
		return d
	}
	if a.tiene("go.mod") {
		return detectarGo(base)
	}
	if d, hay := detectarPHP(a, base); hay {
		return d
	}
	if d, hay := detectarRuby(a, base); hay {
		return d
	}
	if d, hay := detectarJava(a, base); hay {
		return d
	}
	if a.tiene("index.html") {
		base.Runtime = RuntimeStatic
		base.IsStatic = true
		base.PublishDir = "."
		base.BuildPack = PackStatic
		base.Notas = []string{"Sitio estático: se sirve tal cual, sin compilar nada."}
		return base
	}

	base.Notas = []string{"No se pudo deducir con qué está hecho; revisa el Dockerfile generado."}
	return base
}

// ── Node ───────────────────────────────────────────────────────────────────

type packageJSON struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Engines         struct {
		Node string `json:"node"`
	} `json:"engines"`
}

func detectarNode(a Archivos, base Deteccion) (Deteccion, bool) {
	crudo := a.leer("package.json")
	if crudo == "" {
		return base, false
	}

	var pkg packageJSON
	if err := json.Unmarshal([]byte(crudo), &pkg); err != nil {
		// Un package.json roto sigue siendo Node, solo que no se puede
		// leer: mejor decirlo que fingir que el repositorio es otra cosa.
		base.Runtime = RuntimeNode
		base.Notas = []string{"El package.json no se pudo leer; revisa el Dockerfile generado."}
		return base, true
	}

	deps := map[string]bool{}
	for n := range pkg.Dependencies {
		deps[strings.ToLower(n)] = true
	}
	for n := range pkg.DevDependencies {
		deps[strings.ToLower(n)] = true
	}

	gestor := "npm"
	switch {
	case a.tiene("pnpm-lock.yaml"):
		gestor = "pnpm"
	case a.tiene("yarn.lock"):
		gestor = "yarn"
	case a.tiene("bun.lockb"):
		gestor = "bun"
	}

	framework, puerto, estatico, publicar := frameworkDeNode(deps)

	base.Runtime = RuntimeNode
	base.Framework = framework
	base.PackageManager = gestor
	base.RuntimeVersion = primeroNoVacio(pkg.Engines.Node, strings.TrimSpace(a.leer(".nvmrc")))
	base.InstallCommand = comandoDeInstalacion(gestor)
	base.IsStatic = estatico
	base.PublishDir = publicar

	if pkg.Scripts["build"] != "" {
		base.BuildCommand = ejecutar(gestor, "build")
	}
	switch {
	case pkg.Scripts["start"] != "":
		base.StartCommand = ejecutar(gestor, "start")
	case pkg.Scripts["serve"] != "":
		base.StartCommand = ejecutar(gestor, "serve")
	}

	base.Port = puerto
	if p, hay := primerPuerto(pkg.Scripts["start"], pkg.Scripts["serve"]); hay {
		base.Port = p
	}

	base.DatabaseType = baseDeDatosPorDependencias(deps)
	base.NeedsDatabase = base.DatabaseType != SinBase

	switch {
	case base.HasDockerfile:
		base.BuildPack = PackDockerfile
	case estatico:
		base.BuildPack = PackStatic
	}

	base.Notas = notasDeNode(framework, base.StartCommand, base.DatabaseType)
	return base, true
}

func frameworkDeNode(deps map[string]bool) (framework string, puerto int, estatico bool, publicar string) {
	puerto = 3000

	switch {
	case deps["next"]:
		return "next", 3000, false, ""
	case deps["nuxt"]:
		return "nuxt", 3000, false, ""
	case deps["@nestjs/core"]:
		return "nestjs", 3000, false, ""
	case deps["vite"]:
		return "vite", 3000, true, "dist"
	case deps["react-scripts"]:
		return "cra", 3000, true, "build"
	case deps["express"], deps["fastify"], deps["koa"], deps["@hapi/hapi"]:
		return "express", 3000, false, ""
	case deps["@angular/core"]:
		return "angular", 3000, true, "dist"
	case deps["astro"]:
		return "astro", 3000, false, ""
	}

	return "", puerto, false, ""
}

func comandoDeInstalacion(gestor string) string {
	switch gestor {
	case "yarn":
		return "yarn install --frozen-lockfile"
	case "pnpm":
		return "pnpm install --frozen-lockfile"
	case "bun":
		return "bun install"
	default:
		return "npm ci"
	}
}

func ejecutar(gestor, script string) string {
	switch gestor {
	case "bun":
		return "bun run " + script
	case "yarn":
		return "yarn " + script
	case "pnpm":
		return "pnpm " + script
	default:
		return "npm run " + script
	}
}

func notasDeNode(framework, arranque string, db BaseDatos) []string {
	notas := []string{}

	if framework == "next" {
		notas = append(notas,
			`Next.js: para una imagen más pequeña, valora output: "standalone" en next.config.`)
	}
	if arranque == "" && framework != "vite" && framework != "cra" {
		notas = append(notas, `No hay script "start"; ajusta el comando de arranque.`)
	}
	if db != SinBase {
		notas = append(notas, "Se detectó "+string(db)+
			": el docker-compose incluye ese servicio y una variable DATABASE_URL.")
	}

	return notas
}

func baseDeDatosPorDependencias(deps map[string]bool) BaseDatos {
	contiene := func(trozos ...string) bool {
		for n := range deps {
			for _, t := range trozos {
				if strings.Contains(n, t) {
					return true
				}
			}
		}
		return false
	}

	switch {
	case contiene("pg", "postgres", "prisma"):
		return Postgres
	case contiene("mysql", "mariadb"):
		return MySQL
	case contiene("mongoose", "mongodb"):
		return MongoDB
	case contiene("redis"):
		return Redis
	}
	return SinBase
}

// ── Python ─────────────────────────────────────────────────────────────────

func detectarPython(a Archivos, base Deteccion) (Deteccion, bool) {
	pyproject := a.leer("pyproject.toml")
	requirements := a.leer("requirements.txt")

	if pyproject == "" && requirements == "" && !a.tiene("manage.py") {
		return base, false
	}

	texto := strings.ToLower(requirements + "\n" + pyproject)
	poetry := strings.Contains(pyproject, "[tool.poetry]")

	base.Runtime = RuntimePython
	base.Port = 8000
	base.PackageManager = "pip"
	base.InstallCommand = "pip install -r requirements.txt"
	if poetry {
		base.PackageManager = "poetry"
		base.InstallCommand = "poetry install --no-root"
	}

	switch {
	case a.tiene("manage.py") || strings.Contains(texto, "django"):
		base.Framework = "django"
		base.StartCommand = "gunicorn app.wsgi:application --bind 0.0.0.0:8000"
		base.BuildCommand = "python manage.py collectstatic --noinput"
		base.Notas = []string{
			"Django: define SECRET_KEY y DATABASE_URL como variables de entorno, " +
				"y corre las migraciones tras el primer despliegue.",
		}
	case strings.Contains(texto, "fastapi"):
		base.Framework = "fastapi"
		base.StartCommand = "uvicorn main:app --host 0.0.0.0 --port 8000"
	case strings.Contains(texto, "flask"):
		base.Framework = "flask"
		base.StartCommand = "gunicorn app:app --bind 0.0.0.0:8000"
	default:
		base.StartCommand = "python main.py"
	}

	switch {
	case strings.Contains(texto, "psycopg"), strings.Contains(texto, "postgres"):
		base.DatabaseType = Postgres
	case strings.Contains(texto, "mysqlclient"), strings.Contains(texto, "pymysql"):
		base.DatabaseType = MySQL
	case strings.Contains(texto, "pymongo"):
		base.DatabaseType = MongoDB
	case strings.Contains(texto, "redis"):
		base.DatabaseType = Redis
	}
	base.NeedsDatabase = base.DatabaseType != SinBase

	if base.HasDockerfile {
		base.BuildPack = PackDockerfile
	}
	return base, true
}

// ── Los demás ──────────────────────────────────────────────────────────────

func detectarGo(base Deteccion) Deteccion {
	base.Runtime = RuntimeGo
	base.PackageManager = "go"
	base.InstallCommand = "go mod download"
	base.BuildCommand = "go build -o /app/server ./..."
	base.StartCommand = "/app/server"
	base.Port = 8080
	base.Notas = []string{
		"Go: se da por hecho que el programa principal está en la raíz; " +
			"ajusta la compilación si tu main vive en cmd/.",
	}
	if base.HasDockerfile {
		base.BuildPack = PackDockerfile
	}
	return base
}

func detectarPHP(a Archivos, base Deteccion) (Deteccion, bool) {
	crudo := a.leer("composer.json")
	if crudo == "" {
		return base, false
	}

	var composer struct {
		Require map[string]string `json:"require"`
	}
	_ = json.Unmarshal([]byte(crudo), &composer)
	_, laravel := composer.Require["laravel/framework"]

	base.Runtime = RuntimePHP
	base.PackageManager = "composer"
	base.InstallCommand = "composer install --no-dev --optimize-autoloader"
	base.Port = 8000

	if laravel {
		base.Framework = "laravel"
		base.StartCommand = "php artisan serve --host 0.0.0.0 --port 8000"
		base.Notas = []string{
			"Laravel: define APP_KEY y DATABASE_URL, y corre las migraciones tras el despliegue.",
		}
	} else {
		base.StartCommand = "php -S 0.0.0.0:8000 -t public"
	}

	if base.HasDockerfile {
		base.BuildPack = PackDockerfile
	}
	return base, true
}

func detectarRuby(a Archivos, base Deteccion) (Deteccion, bool) {
	if !a.tiene("Gemfile") {
		return base, false
	}

	rails := strings.Contains(strings.ToLower(a.leer("Gemfile")), "rails")

	base.Runtime = RuntimeRuby
	base.PackageManager = "bundler"
	base.InstallCommand = "bundle install"
	base.Port = 3000

	if rails {
		base.Framework = "rails"
		base.BuildCommand = "bundle exec rake assets:precompile"
		base.StartCommand = "bundle exec rails server -b 0.0.0.0 -p 3000"
		base.Notas = []string{
			"Rails: define RAILS_MASTER_KEY (o SECRET_KEY_BASE) y DATABASE_URL, " +
				"y corre las migraciones tras el despliegue.",
		}
	} else {
		base.StartCommand = "bundle exec rackup -o 0.0.0.0 -p 3000"
	}

	if base.HasDockerfile {
		base.BuildPack = PackDockerfile
	}
	return base, true
}

func detectarJava(a Archivos, base Deteccion) (Deteccion, bool) {
	gradle := a.tiene("build.gradle") || a.tiene("build.gradle.kts")
	if !a.tiene("pom.xml") && !gradle {
		return base, false
	}

	base.Runtime = RuntimeJava
	base.Framework = "spring"
	base.Port = 8080
	base.StartCommand = "java -jar app.jar"

	if gradle {
		base.PackageManager = "gradle"
		base.BuildCommand = "./gradlew build -x test"
	} else {
		base.PackageManager = "maven"
		base.BuildCommand = "mvn -q package -DskipTests"
	}

	base.Notas = []string{
		"Java: se da por hecho un jar ejecutable (Spring Boot); " +
			"ajusta el nombre del artefacto si es otro.",
	}
	if base.HasDockerfile {
		base.BuildPack = PackDockerfile
	}
	return base, true
}

// ── Auxiliares ─────────────────────────────────────────────────────────────

var rePuerto = regexp.MustCompile(`(?:PORT|port|-p|--port[= ]|:)\s*[:=]?\s*(\d{2,5})`)

// primerPuerto saca el puerto del comando de arranque, si lo lleva escrito.
func primerPuerto(candidatos ...string) (int, bool) {
	for _, c := range candidatos {
		if c == "" {
			continue
		}
		if m := rePuerto.FindStringSubmatch(c); m != nil {
			n, err := strconv.Atoi(m[1])
			if err == nil && n >= 1 && n <= 65535 {
				return n, true
			}
		}
	}
	return 0, false
}

func primeroNoVacio(valores ...string) string {
	for _, v := range valores {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
