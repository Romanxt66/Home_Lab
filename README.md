# Home Lab

Panel de herramientas de desarrollo. Reescritura del [Lab](https://github.com/Romanxt66/Lab)
(Next.js monolítico) en dos piezas separadas:

- **`backend/`** — API JSON en Go con Gin.
- **`frontend/`** — SPA en React con Vite.

El Lab original sigue funcionando y no se toca. Este repositorio avanza módulo a
módulo hasta poder sustituirlo.

## Estado

| Módulo | Estado |
|---|---|
| Autenticación | Portado |
| Monitor de uptime | Portado |
| Los otros 13 | Pendientes |

Del Lab quedan por portar 112 de las 117 acciones y 17 de los 18 módulos.

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
