package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/assistant"
	"github.com/Romanxt66/Home_Lab/backend/internal/automations"
	"github.com/Romanxt66/Home_Lab/backend/internal/calendar"
	"github.com/Romanxt66/Home_Lab/backend/internal/coolify"
	"github.com/Romanxt66/Home_Lab/backend/internal/dbadmin"
	"github.com/Romanxt66/Home_Lab/backend/internal/finance"
	"github.com/Romanxt66/Home_Lab/backend/internal/github"
	"github.com/Romanxt66/Home_Lab/backend/internal/inventory"
	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
	"github.com/Romanxt66/Home_Lab/backend/internal/n8n"
	"github.com/Romanxt66/Home_Lab/backend/internal/uptime"
	"github.com/Romanxt66/Home_Lab/backend/internal/users"
)

// Las herramientas del asistente viven aquí, en la raíz de composición, y no
// en el paquete assistant: son justo lo que lo conecta con todos los demás
// módulos, y si assistant los importara sería el paquete que depende de todo.
// Así assistant solo sabe de herramientas en abstracto.
//
// Cada una delega en el mismo servicio que usa la interfaz, de modo que las
// validaciones y los guardarraíles son los mismos.
//
// A propósito NO se expone: borrar nada, cambiar roles, editar credenciales
// de conexiones, ni SQL que modifique. Un asistente al que se le habla en
// lenguaje natural no debería poder hacer lo irreversible.

// Servicios agrupa de lo que dependen las herramientas.
type Servicios struct {
	Finance     *finance.Service
	Uptime      *uptime.Service
	Users       *users.Service
	Calendar    *calendar.Service
	Inventory   *inventory.Service
	Automations *automations.Service
	DBAdmin     *dbadmin.Service
	Coolify     *coolify.Service
	GitHub      *github.Service
	N8n         *n8n.Service
	// Destinos son las secciones a las que se puede llevar al usuario.
	Destinos []Destino
}

type Destino struct {
	Slug string
	Name string
}

// ── Lectura de la entrada, que llega como JSON sin tipo ────────────────────

func texto(in map[string]any, clave string) string {
	if v, ok := in[clave].(string); ok {
		return v
	}
	return ""
}

// numero acepta tanto un número como un texto numérico: los modelos a veces
// mandan "5" donde se pedía 5.
func numero(in map[string]any, clave string) float64 {
	switch v := in[clave].(type) {
	case float64:
		return v
	case string:
		n, _ := strconv.ParseFloat(v, 64)
		return n
	}
	return 0
}

func entero(in map[string]any, clave string) int { return int(numero(in, clave)) }

func booleano(in map[string]any, clave string) bool {
	switch v := in[clave].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

func hay(in map[string]any, clave string) bool {
	v, ok := in[clave]
	return ok && v != nil && v != ""
}

func enJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "Error: no se pudo preparar la respuesta."
	}
	return string(b)
}

// resultado pasa un Result a texto para el modelo: el mensaje de éxito, o
// "Error: …" con el motivo, que es lo que el modelo sabe leer y explicar.
func resultado[T any](res kernel.Result[T], err error, ok func(T) string) string {
	if err != nil {
		return "Error: ha fallado algo interno."
	}
	if !res.OK {
		return "Error: " + res.Error
	}
	return ok(res.Value)
}

func soloSuperadmin(c assistant.Contexto) string {
	if c.Role == "superadmin" {
		return ""
	}
	return "No autorizado: esta acción requiere el rol superadmin."
}

// ── El registro ────────────────────────────────────────────────────────────

func construirHerramientas(s Servicios) []assistant.Herramienta {
	vacio := `{"type":"object","properties":{}}`

	listaDeDestinos := make([]string, 0, len(s.Destinos))
	slugs := map[string]string{}
	for _, d := range s.Destinos {
		listaDeDestinos = append(listaDeDestinos, d.Slug+" ("+d.Name+")")
		slugs[d.Slug] = d.Name
	}

	return []assistant.Herramienta{
		// ── Navegación ─────────────────────────────────────────────────────
		{
			Nombre: "navigate_to_module",
			Descripcion: `Abre una herramienta en pantalla para el usuario. Úsala cuando pida «llévame a», «muéstrame», «abre»… Destinos: ` +
				strings.Join(listaDeDestinos, ", ") + ".",
			Esquema: `{"type":"object","properties":{"slug":{"type":"string","description":"Destino a abrir."}},"required":["slug"]}`,
			Ejecutar: func(_ context.Context, in map[string]any, c assistant.Contexto) string {
				slug := texto(in, "slug")
				nombre, existe := slugs[slug]
				if !existe {
					return "Error: no existe «" + slug + "». Opciones: " + strings.Join(listaDeDestinos, ", ")
				}
				c.Efectos.Navegar(slug)
				return "Abriendo «" + nombre + "» en pantalla."
			},
		},

		// ── Finanzas ───────────────────────────────────────────────────────
		{
			Nombre:      "get_finance_summary",
			Descripcion: "Resumen financiero de un mes: ingresos, gastos, neto y estado de los presupuestos. Sin mes ni año, usa el actual.",
			Esquema:     `{"type":"object","properties":{"year":{"type":"integer","description":"Año, p. ej. 2026"},"month":{"type":"integer","description":"Mes 1-12"}}}`,
			Ejecutar: func(ctx context.Context, in map[string]any, _ assistant.Contexto) string {
				ahora := time.Now()
				año, mes := ahora.Year(), int(ahora.Month())
				if hay(in, "year") {
					año = entero(in, "year")
				}
				if hay(in, "month") {
					mes = entero(in, "month")
				}

				resumen, err := s.Finance.ResumenMensual(ctx, año, mes)
				if err != nil {
					return "Error: no se pudo calcular el resumen."
				}
				presupuestos, err := s.Finance.PresupuestosConAvance(ctx, año, mes)
				if err != nil {
					return "Error: no se pudieron leer los presupuestos."
				}

				filas := make([]map[string]any, 0, len(presupuestos))
				for _, p := range presupuestos {
					filas = append(filas, map[string]any{
						"categoriaId": p.Presupuesto.CategoryID,
						"limite":      p.Avance.Limit.Float(),
						"gastado":     p.Avance.Spent.Float(),
						"estado":      string(p.Avance.Status),
					})
				}

				return enJSON(map[string]any{
					"year": año, "month": mes,
					"ingresos": resumen.Income.Float(), "gastos": resumen.Expense.Float(),
					"neto": resumen.Net.Float(), "presupuestos": filas,
				})
			},
		},
		{
			Nombre:      "list_finance_accounts",
			Descripcion: "Lista las cuentas financieras (caja, banco, tarjeta…) con su saldo actual.",
			Esquema:     vacio,
			Ejecutar: func(ctx context.Context, _ map[string]any, _ assistant.Contexto) string {
				cuentas, err := s.Finance.CuentasConSaldo(ctx, false)
				if err != nil {
					return "Error: no se pudieron leer las cuentas."
				}
				filas := make([]map[string]any, 0, len(cuentas))
				for _, c := range cuentas {
					filas = append(filas, map[string]any{
						"id": c.Cuenta.ID, "nombre": c.Cuenta.Name,
						"tipo": string(c.Cuenta.Kind), "saldo": c.Balance.Float(),
					})
				}
				return enJSON(filas)
			},
		},
		{
			Nombre:      "list_finance_categories",
			Descripcion: "Lista las categorías de movimientos (ingreso o gasto) con su id; útil antes de crear un movimiento.",
			Esquema:     vacio,
			Ejecutar: func(ctx context.Context, _ map[string]any, _ assistant.Contexto) string {
				cats, err := s.Finance.Categorias(ctx)
				if err != nil {
					return "Error: no se pudieron leer las categorías."
				}
				filas := make([]map[string]any, 0, len(cats))
				for _, c := range cats {
					filas = append(filas, map[string]any{"id": c.ID, "nombre": c.Name, "tipo": string(c.Kind)})
				}
				return enJSON(filas)
			},
		},
		{
			Nombre:      "list_finance_transactions",
			Descripcion: "Lista los movimientos financieros más recientes.",
			Esquema:     `{"type":"object","properties":{"limit":{"type":"integer","description":"Máximo de movimientos (por defecto 20)."}}}`,
			Ejecutar: func(ctx context.Context, in map[string]any, _ assistant.Contexto) string {
				limite := 20
				if hay(in, "limit") {
					limite = entero(in, "limit")
				}
				movs, err := s.Finance.Movimientos(ctx, finance.FiltroMovimientos{Limite: limite})
				if err != nil {
					return "Error: no se pudieron leer los movimientos."
				}
				filas := make([]map[string]any, 0, len(movs))
				for _, m := range movs {
					filas = append(filas, map[string]any{
						"id": m.ID, "tipo": string(m.Kind), "monto": m.Amount.Float(),
						"categoriaId": m.CategoryID, "fecha": m.OccurredAt.Format(time.RFC3339), "notas": m.Notes,
					})
				}
				return enJSON(filas)
			},
		},
		{
			Nombre:      "create_financial_transaction",
			Descripcion: "Registra un ingreso o un gasto. Necesitas el id de la cuenta (list_finance_accounts) y, si quieres, el de la categoría (list_finance_categories).",
			Esquema:     `{"type":"object","properties":{"accountId":{"type":"string"},"categoryId":{"type":"string","description":"Opcional."},"kind":{"type":"string","enum":["income","expense"]},"amount":{"type":"number","description":"Importe positivo."},"notes":{"type":"string"}},"required":["accountId","kind","amount"]}`,
			Ejecutar: func(ctx context.Context, in map[string]any, _ assistant.Contexto) string {
				tipo := finance.Gasto
				if texto(in, "kind") == "income" {
					tipo = finance.Ingreso
				}

				m := finance.Movimiento{
					AccountID: texto(in, "accountId"), Kind: tipo,
					Amount: finance.DesdeFloat(numero(in, "amount")), OccurredAt: time.Now(),
				}
				if hay(in, "categoryId") {
					c := texto(in, "categoryId")
					m.CategoryID = &c
				}
				if hay(in, "notes") {
					n := texto(in, "notes")
					m.Notes = &n
				}

				res, err := s.Finance.GuardarMovimiento(ctx, m)
				return resultado(res, err, func(v finance.Movimiento) string {
					return "Movimiento registrado (id " + v.ID + ")."
				})
			},
		},
		{
			Nombre:      "set_budget",
			Descripcion: "Fija o actualiza el límite mensual de presupuesto de una categoría.",
			Esquema:     `{"type":"object","properties":{"categoryId":{"type":"string"},"amount":{"type":"number","description":"Límite mensual, mayor que 0."}},"required":["categoryId","amount"]}`,
			Ejecutar: func(ctx context.Context, in map[string]any, _ assistant.Contexto) string {
				res, err := s.Finance.GuardarPresupuesto(ctx, finance.Presupuesto{
					CategoryID: texto(in, "categoryId"),
					Amount:     finance.DesdeFloat(numero(in, "amount")),
				})
				return resultado(res, err, func(p finance.Presupuesto) string {
					return "Presupuesto fijado en " + p.Amount.String() + " para la categoría " + p.CategoryID + "."
				})
			},
		},

		// ── Uptime ─────────────────────────────────────────────────────────
		{
			Nombre:      "list_uptime_monitors",
			Descripcion: "Estado actual de todos los monitores de uptime: en línea, caído o sin datos.",
			Esquema:     vacio,
			Ejecutar: func(ctx context.Context, _ map[string]any, _ assistant.Contexto) string {
				monitores, err := s.Uptime.List(ctx)
				if err != nil {
					return "Error: no se pudieron leer los monitores."
				}
				filas := make([]map[string]any, 0, len(monitores))
				for _, m := range monitores {
					filas = append(filas, map[string]any{
						"id": m.ID, "nombre": m.Name, "url": m.URL, "estado": string(m.LastStatus),
						"activo": m.Active, "fallosSeguidos": m.ConsecutiveFailures,
					})
				}
				return enJSON(filas)
			},
		},
		{
			Nombre:      "check_uptime_monitor_now",
			Descripcion: "Fuerza una comprobación inmediata de un monitor y devuelve el resultado.",
			Esquema:     `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`,
			Ejecutar: func(ctx context.Context, in map[string]any, _ assistant.Contexto) string {
				res, err := s.Uptime.CheckNow(ctx, texto(in, "id"))
				return resultado(res, err, func(m uptime.Monitor) string {
					salida := "«" + m.Name + "»: " + string(m.LastStatus)
					if m.LastResponseMs != nil && *m.LastResponseMs > 0 {
						salida += fmt.Sprintf(" (%d ms)", *m.LastResponseMs)
					}
					return salida + "."
				})
			},
		},
		{
			Nombre:      "toggle_uptime_monitor",
			Descripcion: "Activa o pausa un monitor de uptime.",
			Esquema:     `{"type":"object","properties":{"id":{"type":"string"},"active":{"type":"boolean"}},"required":["id","active"]}`,
			Ejecutar: func(ctx context.Context, in map[string]any, _ assistant.Contexto) string {
				detalle, err := s.Uptime.Detail(ctx, texto(in, "id"))
				if err != nil {
					return "Error: no se pudo leer el monitor."
				}
				if detalle == nil {
					return "Error: ese monitor ya no existe."
				}

				m := detalle.Monitor
				activar := booleano(in, "active")

				res, err := s.Uptime.Save(ctx, m.ID, uptime.MonitorInput{
					Name: m.Name, URL: m.URL, Method: m.Method, ExpectedStatus: m.ExpectedStatus,
					IntervalSeconds: m.IntervalSeconds, TimeoutMs: m.TimeoutMs,
					Active: activar, NotifyOnFailure: m.NotifyOnFailure,
				})
				return resultado(res, err, func(g uptime.Monitor) string {
					estado := "pausado"
					if activar {
						estado = "activado"
					}
					return "Monitor «" + g.Name + "» " + estado + "."
				})
			},
		},

		// ── Usuarios (solo superadmin) ─────────────────────────────────────
		{
			Nombre:      "list_users",
			Descripcion: "Lista los usuarios con su estado (pendiente, aprobado, rechazado) y su rol. Solo superadmin.",
			Esquema:     `{"type":"object","properties":{"onlyPending":{"type":"boolean","description":"Si es true, solo los pendientes de aprobación."}}}`,
			Ejecutar: func(ctx context.Context, in map[string]any, c assistant.Contexto) string {
				if no := soloSuperadmin(c); no != "" {
					return no
				}
				lista, err := s.Users.List(ctx)
				if err != nil {
					return "Error: no se pudieron leer los usuarios."
				}
				soloPendientes := booleano(in, "onlyPending")

				filas := make([]map[string]any, 0, len(lista))
				for _, u := range lista {
					if soloPendientes && u.Status != users.StatusPending {
						continue
					}
					filas = append(filas, map[string]any{
						"id": u.ID, "nombre": u.Name, "email": u.Email,
						"estado": string(u.Status), "rol": string(u.Role),
					})
				}
				return enJSON(filas)
			},
		},
		{
			Nombre:      "approve_pending_user",
			Descripcion: "Aprueba a un usuario pendiente para que pueda entrar. Solo superadmin.",
			Esquema:     `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`,
			Ejecutar: func(ctx context.Context, in map[string]any, c assistant.Contexto) string {
				if no := soloSuperadmin(c); no != "" {
					return no
				}
				res, err := s.Users.Approve(ctx, texto(in, "id"))
				return resultado(res, err, func(u users.User) string { return "Usuario " + u.Email + " aprobado." })
			},
		},
		{
			Nombre:      "reject_pending_user",
			Descripcion: "Rechaza a un usuario, o le revoca el acceso. Solo superadmin.",
			Esquema:     `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`,
			Ejecutar: func(ctx context.Context, in map[string]any, c assistant.Contexto) string {
				if no := soloSuperadmin(c); no != "" {
					return no
				}
				res, err := s.Users.Reject(ctx, texto(in, "id"), c.UID)
				return resultado(res, err, func(u users.User) string { return "Usuario " + u.Email + " rechazado." })
			},
		},

		// ── Calendario ─────────────────────────────────────────────────────
		{
			Nombre:      "list_upcoming_calendar_events",
			Descripcion: "Lista los eventos de los próximos N días (por defecto 7).",
			Esquema:     `{"type":"object","properties":{"days":{"type":"integer"}}}`,
			Ejecutar: func(ctx context.Context, in map[string]any, _ assistant.Contexto) string {
				dias := 7
				if hay(in, "days") {
					dias = entero(in, "days")
				}
				ahora := time.Now()
				hasta := ahora.AddDate(0, 0, dias)

				// Se piden este mes y el siguiente porque la ventana puede
				// cruzar el cambio de mes.
				var todos []calendar.Evento
				for _, desplazamiento := range []int{0, 1} {
					f := ahora.AddDate(0, desplazamiento, 0)
					eventos, err := s.Calendar.Mes(ctx, f.Year(), int(f.Month()))
					if err != nil {
						return "Error: no se pudieron leer los eventos."
					}
					todos = append(todos, eventos...)
				}

				vistos := map[string]bool{}
				proximos := make([]calendar.Evento, 0)
				for _, e := range todos {
					if vistos[e.ID] || e.Start.Before(ahora) || e.Start.After(hasta) {
						continue
					}
					vistos[e.ID] = true
					proximos = append(proximos, e)
				}
				sort.Slice(proximos, func(i, j int) bool { return proximos[i].Start.Before(proximos[j].Start) })

				filas := make([]map[string]any, 0, len(proximos))
				for _, e := range proximos {
					filas = append(filas, map[string]any{
						"id": e.ID, "titulo": e.Title, "inicio": e.Start.Format(time.RFC3339),
					})
				}
				return enJSON(filas)
			},
		},
		{
			Nombre:      "create_calendar_event",
			Descripcion: "Crea un evento en el calendario.",
			Esquema:     `{"type":"object","properties":{"title":{"type":"string"},"start":{"type":"string","description":"Fecha y hora en ISO 8601."},"allDay":{"type":"boolean"},"description":{"type":"string"},"remindMinutesBefore":{"type":"integer","description":"Minutos antes para recordar (opcional)."}},"required":["title","start"]}`,
			Ejecutar: func(ctx context.Context, in map[string]any, _ assistant.Contexto) string {
				inicio, err := time.Parse(time.RFC3339, texto(in, "start"))
				if err != nil {
					return "Error: la fecha no es válida; usa el formato ISO 8601 (2026-03-14T09:00:00Z)."
				}

				e := calendar.Evento{Title: texto(in, "title"), Start: inicio, AllDay: booleano(in, "allDay")}
				if hay(in, "description") {
					d := texto(in, "description")
					e.Description = &d
				}
				if hay(in, "remindMinutesBefore") {
					m := entero(in, "remindMinutesBefore")
					e.RemindMinutesBefore = &m
				}

				res, errSvc := s.Calendar.Crear(ctx, e)
				return resultado(res, errSvc, func(v calendar.Evento) string {
					return "Evento «" + v.Title + "» creado para " + v.Start.Format("02/01/2006 15:04") + "."
				})
			},
		},

		// ── Inventario ─────────────────────────────────────────────────────
		{
			Nombre:      "get_inventory_summary",
			Descripcion: "Lista los artículos del inventario con su cantidad actual.",
			Esquema:     `{"type":"object","properties":{"search":{"type":"string","description":"Filtrar por nombre (opcional)."}}}`,
			Ejecutar: func(ctx context.Context, in map[string]any, _ assistant.Contexto) string {
				items, err := s.Inventory.ListItems(ctx, "")
				if err != nil {
					return "Error: no se pudo leer el inventario."
				}

				busqueda := strings.ToLower(strings.TrimSpace(texto(in, "search")))
				filas := make([]map[string]any, 0, len(items))
				for _, i := range items {
					if busqueda != "" && !strings.Contains(strings.ToLower(i.Name), busqueda) {
						continue
					}
					filas = append(filas, map[string]any{
						"id": i.ID, "nombre": i.Name, "categoria": i.Category,
						"cantidad": i.Quantity, "unidad": i.Unit,
					})
				}
				return enJSON(filas)
			},
		},
		{
			Nombre:      "record_inventory_movement",
			Descripcion: "Registra un movimiento de stock de un artículo (comprado, vendido, regalado, recibido, ajuste).",
			Esquema:     `{"type":"object","properties":{"itemId":{"type":"string"},"kind":{"type":"string","enum":["bought","sold","gave_away","received","moved","adjust"]},"quantity":{"type":"number"},"notes":{"type":"string"}},"required":["itemId","kind","quantity"]}`,
			Ejecutar: func(ctx context.Context, in map[string]any, _ assistant.Contexto) string {
				mov := inventory.MovementInput{
					ItemID: texto(in, "itemId"), Kind: inventory.MovementKind(texto(in, "kind")),
					Quantity: entero(in, "quantity"), OccurredAt: time.Now(),
				}
				if hay(in, "notes") {
					n := texto(in, "notes")
					mov.Notes = &n
				}

				res, err := s.Inventory.RecordMovement(ctx, mov)
				return resultado(res, err, func(m inventory.Movement) string {
					return "Movimiento registrado (id " + m.ID + ")."
				})
			},
		},

		// ── Automatizaciones ───────────────────────────────────────────────
		{
			Nombre:      "list_automation_rules",
			Descripcion: "Lista las reglas de automatización (disparador, acciones y si están activas).",
			Esquema:     vacio,
			Ejecutar: func(ctx context.Context, _ map[string]any, _ assistant.Contexto) string {
				reglas, err := s.Automations.List(ctx)
				if err != nil {
					return "Error: no se pudieron leer las reglas."
				}
				filas := make([]map[string]any, 0, len(reglas))
				for _, r := range reglas {
					tipos := make([]string, 0, len(r.Actions))
					for _, a := range r.Actions {
						tipos = append(tipos, string(a.Type))
					}
					filas = append(filas, map[string]any{
						"id": r.ID, "nombre": r.Name, "disparador": string(r.Trigger),
						"activa": r.Active, "acciones": tipos,
					})
				}
				return enJSON(filas)
			},
		},
		{
			Nombre:      "toggle_automation_rule",
			Descripcion: "Activa o pausa una regla de automatización.",
			Esquema:     `{"type":"object","properties":{"id":{"type":"string"},"active":{"type":"boolean"}},"required":["id","active"]}`,
			Ejecutar: func(ctx context.Context, in map[string]any, _ assistant.Contexto) string {
				reglas, err := s.Automations.List(ctx)
				if err != nil {
					return "Error: no se pudieron leer las reglas."
				}

				var regla *automations.Regla
				for i := range reglas {
					if reglas[i].ID == texto(in, "id") {
						regla = &reglas[i]
						break
					}
				}
				if regla == nil {
					return "Error: esa regla ya no existe."
				}

				regla.Active = booleano(in, "active")
				res, errSvc := s.Automations.Guardar(ctx, *regla)
				return resultado(res, errSvc, func(r automations.Regla) string {
					estado := "pausada"
					if r.Active {
						estado = "activada"
					}
					return "Regla «" + r.Name + "» " + estado + "."
				})
			},
		},

		// ── Bases de datos ─────────────────────────────────────────────────
		{
			Nombre:      "list_database_connections",
			Descripcion: "Lista las conexiones a bases de datos del módulo Base de datos (nombre y si son de solo lectura).",
			Esquema:     vacio,
			Ejecutar: func(ctx context.Context, _ map[string]any, _ assistant.Contexto) string {
				conexiones, err := s.DBAdmin.Conexiones(ctx)
				if err != nil {
					return "Error: no se pudieron leer las conexiones."
				}
				filas := make([]map[string]any, 0, len(conexiones))
				for _, c := range conexiones {
					filas = append(filas, map[string]any{"id": c.ID, "nombre": c.Name, "soloLectura": c.ReadOnly})
				}
				return enJSON(map[string]any{"total": len(filas), "conexiones": filas})
			},
		},
		{
			Nombre:      "list_database_tables",
			Descripcion: "Lista las tablas de un esquema en una conexión (el id sale de list_database_connections). El esquema por defecto es «public».",
			Esquema:     `{"type":"object","properties":{"connectionId":{"type":"string"},"schema":{"type":"string","description":"Por defecto «public»."}},"required":["connectionId"]}`,
			Ejecutar: func(ctx context.Context, in map[string]any, _ assistant.Contexto) string {
				esquema := "public"
				if hay(in, "schema") {
					esquema = texto(in, "schema")
				}

				res, err := s.DBAdmin.Tablas(ctx, texto(in, "connectionId"), esquema)
				return resultado(res, err, func(ts []dbadmin.Tabla) string {
					filas := make([]map[string]any, 0, len(ts))
					for _, t := range ts {
						filas = append(filas, map[string]any{"tabla": t.Name, "tipo": t.Kind, "filasAprox": t.RowsApprox})
					}
					return enJSON(filas)
				})
			},
		},
		{
			Nombre:      "run_database_query",
			Descripcion: "Ejecuta una consulta SQL de LECTURA (SELECT) en una conexión. Cualquier sentencia que modifique o destruya se rechaza: para eso, el usuario debe usar el editor SQL del módulo.",
			Esquema:     `{"type":"object","properties":{"connectionId":{"type":"string"},"sql":{"type":"string","description":"Consulta SQL, preferiblemente con LIMIT."}},"required":["connectionId","sql"]}`,
			Ejecutar: func(ctx context.Context, in map[string]any, _ assistant.Contexto) string {
				consulta := texto(in, "sql")

				// Más estricto que el Lab: allí solo se rechazaba lo
				// destructivo, así que un INSERT o un UPDATE con WHERE
				// pasaban en una conexión con escritura, pese a que la
				// herramienta se describe como de lectura. Un asistente al
				// que se le habla en lenguaje natural no debe modificar
				// datos por esta vía, que tiene su sitio en el editor.
				if dbadmin.Analizar(consulta).Riesgo != dbadmin.Inofensiva {
					return "Error: esta herramienta solo ejecuta consultas de lectura. " +
						"Para modificar datos, el usuario debe usar el editor SQL del módulo."
				}

				res, err := s.DBAdmin.Consultar(ctx, texto(in, "connectionId"), consulta, false)
				return resultado(res, err, func(r dbadmin.Resultado) string {
					filas := r.Rows
					if len(filas) > 50 {
						filas = filas[:50]
					}
					return enJSON(map[string]any{"columnas": r.Columns, "filas": filas, "totalFilas": r.RowCount})
				})
			},
		},

		// ── Despliegues ────────────────────────────────────────────────────
		{
			Nombre:      "list_deployments_overview",
			Descripcion: "Estado de la infraestructura en Coolify: aplicaciones, bases de datos y servicios, con su estado.",
			Esquema:     vacio,
			Ejecutar: func(ctx context.Context, _ map[string]any, _ assistant.Contexto) string {
				res, err := s.Coolify.Resumen(ctx)
				return resultado(res, err, func(r coolify.Resumen) string {
					apps := make([]map[string]any, 0, len(r.Apps))
					for _, a := range r.Apps {
						apps = append(apps, map[string]any{
							"uuid": a.UUID, "nombre": a.Name, "estado": string(a.State), "url": a.FQDN,
						})
					}
					return enJSON(map[string]any{
						"aplicaciones": apps,
						"basesDeDatos": recursosParaElModelo(r.Bases),
						"servicios":    recursosParaElModelo(r.Servicios),
					})
				})
			},
		},
		{
			Nombre:      "list_recent_deployments",
			Descripcion: "Historial reciente de despliegues en Coolify y su estado.",
			Esquema:     vacio,
			Ejecutar: func(ctx context.Context, _ map[string]any, _ assistant.Contexto) string {
				res, err := s.Coolify.Despliegues(ctx)
				return resultado(res, err, func(ds []coolify.Despliegue) string {
					if len(ds) > 15 {
						ds = ds[:15]
					}
					filas := make([]map[string]any, 0, len(ds))
					for _, d := range ds {
						filas = append(filas, map[string]any{
							"uuid": d.UUID, "app": d.ApplicationName, "estado": string(d.Status),
							"commit": d.CommitMessage, "fecha": d.CreatedAt,
						})
					}
					return enJSON(filas)
				})
			},
		},
		{
			Nombre:      "deploy_application",
			Descripcion: "Lanza un despliegue de una aplicación en Coolify. Confirma con el usuario antes de usarla.",
			Esquema:     `{"type":"object","properties":{"uuid":{"type":"string","description":"UUID de la app (list_deployments_overview)."}},"required":["uuid"]}`,
			Ejecutar: func(ctx context.Context, in map[string]any, _ assistant.Contexto) string {
				res, err := s.Coolify.Desplegar(ctx, texto(in, "uuid"), false)
				return resultado(res, err, func(m string) string { return m })
			},
		},
		{
			Nombre:      "control_application",
			Descripcion: "Arranca, detiene o reinicia una aplicación en Coolify. Confirma con el usuario antes de usarla.",
			Esquema:     `{"type":"object","properties":{"uuid":{"type":"string"},"action":{"type":"string","enum":["start","stop","restart"]}},"required":["uuid","action"]}`,
			Ejecutar: func(ctx context.Context, in map[string]any, _ assistant.Contexto) string {
				res, err := s.Coolify.Controlar(ctx, texto(in, "uuid"), coolify.Accion(texto(in, "action")))
				return resultado(res, err, func(m string) string { return m })
			},
		},
		{
			Nombre:      "get_application_logs",
			Descripcion: "Devuelve las últimas líneas de registro de una aplicación en Coolify.",
			Esquema:     `{"type":"object","properties":{"uuid":{"type":"string"},"lines":{"type":"integer","description":"Número de líneas (por defecto 100)."}},"required":["uuid"]}`,
			Ejecutar: func(ctx context.Context, in map[string]any, _ assistant.Contexto) string {
				lineas := 100
				if hay(in, "lines") {
					lineas = entero(in, "lines")
				}

				res, err := s.Coolify.Registros(ctx, texto(in, "uuid"), lineas)
				return resultado(res, err, func(registros string) string {
					// Se devuelve el final, que es lo reciente, y acotado en
					// caracteres: no hay que gastar el contexto del modelo.
					r := []rune(registros)
					if len(r) > 4000 {
						r = r[len(r)-4000:]
					}
					if len(r) == 0 {
						return "(sin registros)"
					}
					return string(r)
				})
			},
		},

		// ── GitHub ─────────────────────────────────────────────────────────
		{
			Nombre:      "list_github_repos",
			Descripcion: "Lista tus repositorios de GitHub con lenguaje, estrellas y última actualización.",
			Esquema:     vacio,
			Ejecutar: func(ctx context.Context, _ map[string]any, _ assistant.Contexto) string {
				res, err := s.GitHub.Resumen(ctx)
				return resultado(res, err, func(r github.Resumen) string {
					repos := r.Repos
					if len(repos) > 40 {
						repos = repos[:40]
					}
					filas := make([]map[string]any, 0, len(repos))
					for _, x := range repos {
						filas = append(filas, map[string]any{
							"nombre": x.Name, "privado": x.IsPrivate, "lenguaje": x.Language,
							"estrellas": x.Stars, "url": x.URL,
						})
					}
					return enJSON(map[string]any{"usuario": r.Perfil.Login, "total": len(r.Repos), "repos": filas})
				})
			},
		},

		// ── n8n ────────────────────────────────────────────────────────────
		{
			Nombre:      "list_n8n_workflows",
			Descripcion: "Lista los flujos de tu instancia de n8n y si están activos.",
			Esquema:     vacio,
			Ejecutar: func(ctx context.Context, _ map[string]any, _ assistant.Contexto) string {
				res, err := s.N8n.ListarWorkflows(ctx)
				return resultado(res, err, func(ws []n8n.Workflow) string {
					filas := make([]map[string]any, 0, len(ws))
					for _, w := range ws {
						filas = append(filas, map[string]any{"id": w.ID, "nombre": w.Name, "activo": w.Active})
					}
					return enJSON(filas)
				})
			},
		},
		{
			Nombre:      "toggle_n8n_workflow",
			Descripcion: "Activa o desactiva un flujo de n8n.",
			Esquema:     `{"type":"object","properties":{"id":{"type":"string"},"active":{"type":"boolean"}},"required":["id","active"]}`,
			Ejecutar: func(ctx context.Context, in map[string]any, _ assistant.Contexto) string {
				activar := booleano(in, "active")
				res, err := s.N8n.Activar(ctx, texto(in, "id"), activar)
				return resultado(res, err, func(struct{}) string {
					if activar {
						return "Flujo activado."
					}
					return "Flujo desactivado."
				})
			},
		},
	}
}

func recursosParaElModelo(rs []coolify.Recurso) []map[string]any {
	filas := make([]map[string]any, 0, len(rs))
	for _, r := range rs {
		filas = append(filas, map[string]any{"uuid": r.UUID, "nombre": r.Name, "estado": string(r.State)})
	}
	return filas
}
