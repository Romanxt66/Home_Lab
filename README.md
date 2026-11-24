# Home Lab

Panel de herramientas de desarrollo. Reescritura del [Lab](https://github.com/Romanxt66/Lab)
(Next.js monolítico) en dos piezas separadas:

- **`backend/`** — API JSON en Go con Gin.
- **`frontend/`** — SPA en React con Vite.

El Lab original sigue funcionando y no se toca. Este repositorio avanza módulo a
módulo hasta poder sustituirlo.

## Estado

El backend cubre los módulos del Lab **salvo cuatro que se descartaron a
propósito**: Inventario, Finanzas, Monitor de uptime y n8n. Su código se
quitó de este repositorio (queda en el historial de git) y sus tablas no se
tocan: el Lab original sigue usándolas.

| Módulo | Estado |
|---|---|
| Autenticación (contraseña y Google) | Portado |
| Usuarios | Portado |
| Notificaciones (Telegram) | Portado |
| Correo, calendario, tareas programadas | Portado |
| Automatizaciones | Portado |
| Scraper, GitHub | Portado |
| Coolify, administrador de bases de datos | Portado |
| Asistente (Gemini) | Portado |
| Espacio de trabajo (skills y documentos .md) | Nuevo, no existía en el Lab |

El frontend cubre ya **todos** los módulos: acceso, inicio, calendario,
automatizaciones, Coolify, administrador de bases de
datos, correo, GitHub, Telegram, scraper, tareas programadas y usuarios (solo
para administradores), más el **asistente** flotante. El webhook de n8n sigue
existiendo como *acción* de las automatizaciones; lo que se quitó es el panel
de n8n.

Un matiz: el frontend no tiene pruebas automáticas todavía (se ha verificado a
mano contra una API simulada).

## Espacio de trabajo

Biblioteca de tus **skills de Claude** y de documentos **.md** de instrucciones
(`CLAUDE.md`, reglas de proyecto, recetas), con historial de versiones.

- Subes un `.skill` o `.zip`; se valida (necesita un `SKILL.md` con `name` y
  `description`), se guarda y se puede ver por dentro, descargar o volver a una
  versión anterior. «Bajar todo» da un zip listo para copiar a `~/.claude`.
- Cada persona tiene su espacio **privado**; lo que marque como **compartido**
  lo ve y lo baja el resto, pero solo su dueño lo edita (un administrador puede
  retirarlo). Una skill es texto que Claude obedece: por eso nada ajeno llega a
  nadie sin que lo elija.
- Se guarda en PostgreSQL (`hl_workspace_item`, `_version` y `_file`). El
  servicio crea esas tablas al arrancar si no existen; si no pudiera, solo falla
  este módulo.
- Los zips se leen con cuidado: rutas que se salen de la carpeta, enlaces
  simbólicos, archivos de más de 5 MB o más de 12 MB en total se rechazan.

Para probar el SQL contra una base de verdad:
`WORKSPACE_TEST_DB="postgres://…" go test ./internal/workspace/` y
`TOKENS_TEST_DB="postgres://…" go test ./internal/tokens/` (sin esas variables
esas pruebas se saltan; las primeras vacían las tablas `hl_workspace_*`).

### Enlazarlo con Claude (servidor MCP)

El panel expone un servidor **MCP** en `POST /mcp` (Streamable HTTP, solo
herramientas) para que Claude Code lea y guarde en tu espacio sin copiar nada.

- **Entra con un token personal**, no con la cookie. Se crea en *Espacio de
  trabajo → Claude*, se enseña **una sola vez** y en la base solo queda su
  huella SHA-256. Hereda tus permisos, se revoca al instante y deja de valer si
  la cuenta deja de estar aprobada. Máximo 10 activos por persona.
- **Dos permisos:** *solo leer* (por defecto) o *leer y guardar documentos*. Un
  token de lectura ni ve la herramienta que escribe.
- **Herramientas:** `list_skills`, `get_skill`, `read_skill_file`, `list_docs`,
  `read_doc` y `save_doc`. Por defecto solo ven lo tuyo; con `include_shared`
  también lo que otras personas comparten, y entonces el texto llega marcado
  como de un tercero para que Claude no obedezca lo que haya dentro.
- **Conectar Claude Code:**
  `claude mcp add --transport http homelab https://<tu-dominio>/mcp --header "Authorization: Bearer <token>"`
- **Plugin de Claude Code** (`claude-plugin/`): trae el servidor MCP ya
  configurado, los comandos `/homelab:sync`, `/homelab:doc` y `/homelab:save`, y
  un aviso al abrir sesión cuando hay skills nuevas. Para bajar las skills con
  sus archivos binarios usa `GET /api/sync/manifest` y `/api/sync/skills/:id`
  (solo lectura, solo lo tuyo, con el mismo token). Ver `claude-plugin/README.md`.
- **Límites:** no hay OAuth, así que no se puede añadir como conector desde
  claude.ai. Se valida la cabecera `Origin` (contra ataques de *DNS rebinding*)
  y cada respuesta se corta a 100 KB.

## Diseño

La interfaz usa **Cristal monocromo** (el sistema de diseño de Alvea): vidrio
esmerilado sobre una foto difuminada, paleta blanca/negra/gris con tema claro y
oscuro, titulares con una palabra en serif cursiva y movimiento corto que se
apaga con «reducir movimiento». El kit vive en `frontend/src/design/` (la hoja
`cristal.css` y los componentes) y los estilos propios de Home Lab en
`frontend/src/app.css`, escritos solo con sus tokens. Para comprobar que se
respeta: `node <skill>/scripts/check_design.mjs frontend/src`.

## Por qué Go

El lenguaje no es el motivo: el backend compila a un único binario sin
dependencias, así que desplegar es copiar un archivo. La concurrencia también
sale barata, que importa en un monitor que comprueba muchas URLs a la vez.

## Arquitectura

Se conserva la del Lab, que era su mayor acierto: cada módulo aísla su dominio
del framework y de la base de datos.

```
backend/
├── cmd/api/main.go          Raíz de composición (el container.ts del Lab)
└── internal/
    ├── kernel/              Result<T>, sin dependencias
    ├── config/              Lectura del entorno
    ├── database/            Conexión a PostgreSQL
    ├── httpx/               CORS, autenticación, respuestas de error
    ├── auth/                Sesiones firmadas y contraseñas
    └── <módulo>/
        ├── domain.go        Lógica pura: validar, decidir, transiciones
        ├── service.go       Orquestación + interfaces de los puertos
        ├── postgres.go      Adaptador de base de datos
        └── http.go          DTOs y rutas de Gin
```

Las interfaces se declaran en `service.go`, en el lado que las consume, que es
lo idiomático en Go. El dominio no importa Gin ni la base de datos.

## La base de datos no se migra

El backend lee y escribe las **mismas tablas** que el Lab en Next, en el
esquema `lab` de `master_db`. Las dos aplicaciones pueden convivir.

Por el mismo motivo, `auth` reproduce el esquema de sesiones del Lab byte a
byte: misma cookie `lab_session`, misma firma HMAC-SHA256 y los mismos
parámetros de scrypt que usa `crypto.scryptSync` de Node. Una sesión abierta
en el Lab vale en esta API, y las contraseñas ya guardadas siguen validando.

> Cuidado: cambiar los parámetros de scrypt en `auth/password.go` invalidaría
> todas las cuentas existentes.

## En producción

Desplegado en <https://homelab.softlane.click> desde Coolify, en el mismo
servidor y el mismo proyecto que el Lab.

Es un solo contenedor: el `Dockerfile` compila la interfaz con Vite, la
incrusta en el binario con `go:embed` y lo deja sobre Alpine. No hay servidor
de Node en producción, y como la API y la interfaz comparten origen tampoco
hace falta configurar CORS: por eso `VITE_API_URL` está vacío en
`.env.production` y las llamadas son relativas.

`DATABASE_URL` y `SESSION_SECRET` son los mismos valores que usa el Lab, de
modo que las dos aplicaciones comparten base y sesiones. La única diferencia
es la cadena de conexión: Prisma escribe `?schema=lab` y lib/pq, el driver de
Go, necesita `?sslmode=disable&search_path=lab`.

El contenedor se conecta a la red `coolify` («Connect to predefined network»),
que es donde responde el nombre `master-db-central`. Sin eso no encuentra la
base.

## Cómo levantarlo

**Backend** (necesita PostgreSQL con el esquema `lab`):

```bash
cd backend
cp .env.example .env     # y rellenar DATABASE_URL y SESSION_SECRET
go run ./cmd/api
```

**Frontend**:

```bash
cd frontend
cp .env.example .env
npm install
npm run dev
```

La aplicación queda en http://localhost:5173 y la API en http://localhost:8080.

## Pruebas

```bash
cd backend && go test ./...
```

Cada módulo prueba su dominio y sus guardarraíles (permisos, validaciones,
barreras de seguridad); la autenticación cubre la firma de sesiones, la
manipulación del token y la caducidad.

## El reloj

`POST /api/cron/scheduler` lanza las tareas pendientes. Va protegido por
`CRON_SECRET` y, si no está configurado en producción, **no se ejecuta**: es
preferible que el cron no corra a que corra abierto.

El binario también trae un reloj interno (tareas programadas y recordatorios
del calendario), apagado por defecto: se enciende con `SCHEDULER_INTERNO=1`.
Hay que dejarlo apagado mientras el Lab siga desplegado, porque su cron
ejecuta las mismas tareas sobre la misma base y cada aviso saldría duplicado.
