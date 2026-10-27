# Home Lab

Panel de herramientas de desarrollo. Reescritura del [Lab](https://github.com/Romanxt66/Lab)
(Next.js monolítico) en dos piezas separadas:

- **`backend/`** — API JSON en Go con Gin.
- **`frontend/`** — SPA en React con Vite.

El Lab original sigue funcionando y no se toca. Este repositorio avanza módulo a
módulo hasta poder sustituirlo.

## Estado

| Módulo | Estado | Endpoints |
|---|---|---|
| Autenticación | Portado | 3 |
| Monitor de uptime | Portado | 6 |
| Notificaciones (Telegram) | Portado | 4 |
| Inventario | Portado | 14 |
| Los otros 14 | Pendientes | — |

27 de los 117 endpoints del Lab. Quedan por portar: coolify (24), db-admin (15),
finance (18), email (6), scheduler (6), n8n (5), users (4), github (4),
automations (4), calendar (3), deploygen (2), scraper (1), assistant (1) y el
acceso con Google.

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
    └── uptime/
        ├── domain.go        Lógica pura: validar, decidir caído, transiciones
        ├── service.go       Orquestación + interfaces de los puertos
        ├── postgres.go      Adaptador de base de datos
        ├── probe.go         Adaptador HTTP
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

Cubren el dominio de uptime (validación, decisión de caído, transiciones de
estado, porcentaje de disponibilidad) y la autenticación (firma de sesiones,
manipulación del token, caducidad y formatos de contraseña inválidos).

## El cron

`POST /api/cron/uptime` lanza las comprobaciones pendientes. Va protegido por
`CRON_SECRET` y, si no está configurado en producción, **no se ejecuta**: es
preferible que el cron no corra a que corra abierto.
