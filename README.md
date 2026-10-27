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
| Scraper, GitHub, generador de despliegues | Portado |
| Coolify, administrador de bases de datos | Portado |
| Asistente (Gemini) | Portado |

El frontend tiene el acceso, el inicio, el **calendario** y la gestión de
**usuarios** (solo para administradores); las pantallas de
los demás módulos están por hacer. El webhook de n8n sigue existiendo como *acción*
de las automatizaciones; lo que se quitó es el panel de n8n.

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
