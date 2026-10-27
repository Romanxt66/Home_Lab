package deploygen

import (
	"strings"
	"testing"
)

func archivos(rutas []string, contenidos map[string]string) Archivos {
	if contenidos == nil {
		contenidos = map[string]string{}
	}
	return Archivos{Rutas: rutas, Contenidos: contenidos}
}

// ── Detección ──────────────────────────────────────────────────────────────

func TestDetectarNodeConVite(t *testing.T) {
	d := Detectar(archivos(
		[]string{"package.json", "index.html", "vite.config.ts"},
		map[string]string{"package.json": `{
			"scripts": {"build": "vite build", "dev": "vite"},
			"devDependencies": {"vite": "^5.0.0"}
		}`},
	))

	if d.Runtime != RuntimeNode {
		t.Errorf("runtime = %q", d.Runtime)
	}
	if d.Framework != "vite" {
		t.Errorf("framework = %q", d.Framework)
	}
	if !d.IsStatic || d.PublishDir != "dist" {
		t.Errorf("debería ser estático con salida dist: %+v", d)
	}
	if d.BuildPack != PackStatic {
		t.Errorf("buildpack = %q", d.BuildPack)
	}
}

func TestDetectarNextDetectaPostgresPorPrisma(t *testing.T) {
	d := Detectar(archivos(
		[]string{"package.json"},
		map[string]string{"package.json": `{
			"scripts": {"start": "next start", "build": "next build"},
			"dependencies": {"next": "15.0.0", "@prisma/client": "^6"}
		}`},
	))

	if d.Framework != "next" {
		t.Errorf("framework = %q", d.Framework)
	}
	if d.DatabaseType != Postgres || !d.NeedsDatabase {
		t.Errorf("no dedujo postgres por prisma: %+v", d)
	}
	if d.IsStatic {
		t.Error("Next no es un sitio estático")
	}
}

func TestElGestorDePaquetesSaleDelArchivoDeBloqueo(t *testing.T) {
	casos := map[string]string{
		"pnpm-lock.yaml":    "pnpm",
		"yarn.lock":         "yarn",
		"bun.lockb":         "bun",
		"package-lock.json": "npm",
	}

	for bloqueo, quiero := range casos {
		d := Detectar(archivos(
			[]string{"package.json", bloqueo},
			map[string]string{"package.json": `{"scripts":{}}`},
		))
		if d.PackageManager != quiero {
			t.Errorf("con %s salió %q, quería %q", bloqueo, d.PackageManager, quiero)
		}
	}
}

func TestElPuertoSaleDelComandoDeArranque(t *testing.T) {
	d := Detectar(archivos(
		[]string{"package.json"},
		map[string]string{"package.json": `{
			"scripts": {"start": "node server.js --port 4321"},
			"dependencies": {"express": "^4"}
		}`},
	))

	if d.Port != 4321 {
		t.Errorf("puerto = %d, quería 4321", d.Port)
	}
}

func TestDetectarPython(t *testing.T) {
	casos := []struct {
		nombre     string
		rutas      []string
		contenidos map[string]string
		framework  string
		db         BaseDatos
	}{
		{
			"fastapi con postgres",
			[]string{"requirements.txt", "main.py"},
			map[string]string{"requirements.txt": "fastapi\nuvicorn\npsycopg2-binary"},
			"fastapi", Postgres,
		},
		{
			"django",
			[]string{"manage.py", "requirements.txt"},
			map[string]string{"requirements.txt": "Django==5.0"},
			"django", SinBase,
		},
		{
			"flask con redis",
			[]string{"requirements.txt"},
			map[string]string{"requirements.txt": "Flask\nredis"},
			"flask", Redis,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			d := Detectar(archivos(c.rutas, c.contenidos))
			if d.Runtime != RuntimePython {
				t.Errorf("runtime = %q", d.Runtime)
			}
			if d.Framework != c.framework {
				t.Errorf("framework = %q, quería %q", d.Framework, c.framework)
			}
			if d.DatabaseType != c.db {
				t.Errorf("base = %q, quería %q", d.DatabaseType, c.db)
			}
		})
	}
}

func TestPoetrySeDistingueDePip(t *testing.T) {
	d := Detectar(archivos(
		[]string{"pyproject.toml"},
		map[string]string{"pyproject.toml": "[tool.poetry]\nname = \"app\""},
	))

	if d.PackageManager != "poetry" {
		t.Errorf("gestor = %q", d.PackageManager)
	}
	if !strings.Contains(d.InstallCommand, "poetry") {
		t.Errorf("instalación = %q", d.InstallCommand)
	}
}

func TestDetectarGoYJavaYRuby(t *testing.T) {
	if d := Detectar(archivos([]string{"go.mod"}, nil)); d.Runtime != RuntimeGo || d.Port != 8080 {
		t.Errorf("go: %+v", d)
	}

	maven := Detectar(archivos([]string{"pom.xml"}, nil))
	if maven.Runtime != RuntimeJava || maven.PackageManager != "maven" {
		t.Errorf("maven: %+v", maven)
	}

	gradle := Detectar(archivos([]string{"build.gradle.kts"}, nil))
	if gradle.PackageManager != "gradle" {
		t.Errorf("gradle: %+v", gradle)
	}

	rails := Detectar(archivos([]string{"Gemfile"}, map[string]string{"gemfile": "gem 'rails'"}))
	if rails.Runtime != RuntimeRuby || rails.Framework != "rails" {
		t.Errorf("rails: %+v", rails)
	}
}

// Un Dockerfile en el repositorio manda sobre la deducción: si ya lo
// escribieron, es el que hay que usar.
func TestUnDockerfileExistenteCambiaElBuildPack(t *testing.T) {
	d := Detectar(archivos(
		[]string{"package.json", "Dockerfile"},
		map[string]string{"package.json": `{"dependencies":{"express":"^4"}}`},
	))

	if !d.HasDockerfile {
		t.Error("no vio el Dockerfile")
	}
	if d.BuildPack != PackDockerfile {
		t.Errorf("buildpack = %q, quería dockerfile", d.BuildPack)
	}
}

// Un package.json roto sigue siendo un proyecto de Node: decirlo es mejor
// que fingir que el repositorio es otra cosa.
func TestUnPackageJSONRotoSeDiceEnLugarDeIgnorarse(t *testing.T) {
	d := Detectar(archivos(
		[]string{"package.json"},
		map[string]string{"package.json": "{esto no es json"},
	))

	if d.Runtime != RuntimeNode {
		t.Errorf("runtime = %q, quería node", d.Runtime)
	}
	if len(d.Notas) == 0 {
		t.Error("no avisó de que el package.json no se pudo leer")
	}
}

func TestSitioEstaticoSinPackageJSON(t *testing.T) {
	d := Detectar(archivos([]string{"index.html", "estilo.css"}, nil))
	if d.Runtime != RuntimeStatic || !d.IsStatic {
		t.Errorf("no lo vio como sitio estático: %+v", d)
	}
}

func TestRepositorioIndescifrableLoDice(t *testing.T) {
	d := Detectar(archivos([]string{"LEEME.txt"}, nil))
	if d.Runtime != RuntimeUnknown {
		t.Errorf("runtime = %q", d.Runtime)
	}
	if len(d.Notas) == 0 {
		t.Error("no avisó de que no pudo deducir nada")
	}
}

// ── Generación ─────────────────────────────────────────────────────────────

// La forma exec de CMD es la que hace que la señal de parada llegue al
// proceso en vez de quedarse en un shell.
func TestComoArray(t *testing.T) {
	casos := map[string]string{
		"npm run start":        `["npm", "run", "start"]`,
		"  node   server.js  ": `["node", "server.js"]`,
		`python -c "print(1)"`: `["python", "-c", "\"print(1)\""]`,
		"/server":              `["/server"]`,
	}
	for dado, quiero := range casos {
		if got := comoArray(dado); got != quiero {
			t.Errorf("comoArray(%q) = %s, quería %s", dado, got, quiero)
		}
	}
}

func TestImagenNodeSaleDeEngines(t *testing.T) {
	casos := map[string]string{
		">=20":  "node:20-slim",
		"20.x":  "node:20-slim",
		"^22.1": "node:22-slim",
		"":      "node:20-slim", // por defecto
		"18":    "node:18-slim",
	}
	for dado, quiero := range casos {
		if got := imagenNode(dado); got != quiero {
			t.Errorf("imagenNode(%q) = %q, quería %q", dado, got, quiero)
		}
	}
}

func TestDockerfileDeNodeLlevaLoEsencial(t *testing.T) {
	d := Detectar(archivos(
		[]string{"package.json", "pnpm-lock.yaml"},
		map[string]string{"package.json": `{
			"scripts": {"build": "tsc", "start": "node dist/main.js"},
			"dependencies": {"express": "^4"},
			"engines": {"node": ">=22"}
		}`},
	))

	df := GenerarDockerfile(d)
	for _, esperado := range []string{
		"node:22-slim",
		"RUN corepack enable", // pnpm lo necesita
		"pnpm install --frozen-lockfile",
		"ENV NODE_ENV=production",
		"EXPOSE 3000",
	} {
		if !strings.Contains(df, esperado) {
			t.Errorf("falta %q en el Dockerfile:\n%s", esperado, df)
		}
	}
}

// npm y bun no necesitan corepack; meterlo sería una capa de más.
func TestConNpmNoSeHabilitaCorepack(t *testing.T) {
	d := Detectar(archivos(
		[]string{"package.json", "package-lock.json"},
		map[string]string{"package.json": `{"dependencies":{"express":"^4"}}`},
	))

	if strings.Contains(GenerarDockerfile(d), "corepack") {
		t.Error("habilitó corepack con npm")
	}
}

func TestDockerfileEstaticoTerminaEnNginx(t *testing.T) {
	d := Detectar(archivos(
		[]string{"package.json"},
		map[string]string{"package.json": `{
			"scripts": {"build": "vite build"},
			"devDependencies": {"vite": "^5"}
		}`},
	))

	df := GenerarDockerfile(d)
	if !strings.Contains(df, "FROM nginx:alpine") {
		t.Errorf("no sirve con nginx:\n%s", df)
	}
	if !strings.Contains(df, "/app/dist /usr/share/nginx/html") {
		t.Errorf("no copia la salida del build:\n%s", df)
	}
}

func TestSinBaseDeDatosNoHayCompose(t *testing.T) {
	d := Detectar(archivos([]string{"go.mod"}, nil))
	if GenerarCompose(d) != "" {
		t.Error("generó un compose sin hacer falta base de datos")
	}
}

func TestConBaseDeDatosElComposeLaIncluye(t *testing.T) {
	d := Detectar(archivos(
		[]string{"package.json"},
		map[string]string{"package.json": `{"dependencies":{"pg":"^8","express":"^4"}}`},
	))

	compose := GenerarCompose(d)
	for _, esperado := range []string{
		"postgres:16-alpine",
		"DATABASE_URL: postgresql://app:app@db:5432/app",
		"db-data:/var/lib/postgresql/data",
	} {
		if !strings.Contains(compose, esperado) {
			t.Errorf("falta %q en el compose:\n%s", esperado, compose)
		}
	}
}

func TestPlantillasDeBaseUsanLasVariablesDeCoolify(t *testing.T) {
	for _, tipo := range []string{"postgresql", "mysql", "mariadb", "mongodb", "redis"} {
		plantilla := PlantillaDeBase(tipo)
		if !strings.Contains(plantilla, "SERVICE_PASSWORD") {
			t.Errorf("%s no usa las variables de Coolify:\n%s", tipo, plantilla)
		}
		if !strings.Contains(plantilla, "services:") {
			t.Errorf("%s no parece un compose", tipo)
		}
	}
}

// ── Partir la dirección del repositorio ────────────────────────────────────

func TestPartirRepo(t *testing.T) {
	casos := map[string][2]string{
		"https://github.com/Romanxt66/Lab":     {"Romanxt66", "Lab"},
		"https://github.com/Romanxt66/Lab.git": {"Romanxt66", "Lab"},
		"https://github.com/Romanxt66/Lab/":    {"Romanxt66", "Lab"},
		"github.com/Romanxt66/Lab":             {"Romanxt66", "Lab"},
		"git@github.com:Romanxt66/Lab.git":     {"Romanxt66", "Lab"},
		"Romanxt66/Lab":                        {"Romanxt66", "Lab"},
		"  Romanxt66/Lab  ":                    {"Romanxt66", "Lab"},
	}

	for dado, quiero := range casos {
		owner, repo, ok := PartirRepo(dado)
		if !ok {
			t.Errorf("PartirRepo(%q) falló", dado)
			continue
		}
		if owner != quiero[0] || repo != quiero[1] {
			t.Errorf("PartirRepo(%q) = %q/%q, quería %q/%q", dado, owner, repo, quiero[0], quiero[1])
		}
	}

	for _, malo := range []string{"", "   ", "soloalgo"} {
		if _, _, ok := PartirRepo(malo); ok {
			t.Errorf("aceptó una dirección inválida: %q", malo)
		}
	}
}

// Escapar la ruta entera convertiría las barras en %2F y GitHub dejaría de
// ver los directorios.
func TestRutaCodificadaConservaLasBarras(t *testing.T) {
	if got := rutaCodificada("apps/mi app/Dockerfile"); got != "apps/mi%20app/Dockerfile" {
		t.Errorf("rutaCodificada = %q", got)
	}
}
