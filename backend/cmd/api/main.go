// Comando api: el servidor HTTP del Home Lab.
//
// Esta función es la raíz de composición, el equivalente al container.ts del
// Lab en Next: el único sitio donde se decide qué implementación concreta usa
// cada servicio. Los módulos solo conocen interfaces.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/auth"
	"github.com/Romanxt66/Home_Lab/backend/internal/automations"
	"github.com/Romanxt66/Home_Lab/backend/internal/calendar"
	"github.com/Romanxt66/Home_Lab/backend/internal/config"
	"github.com/Romanxt66/Home_Lab/backend/internal/database"
	"github.com/Romanxt66/Home_Lab/backend/internal/email"
	"github.com/Romanxt66/Home_Lab/backend/internal/github"
	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
	"github.com/Romanxt66/Home_Lab/backend/internal/inventory"
	"github.com/Romanxt66/Home_Lab/backend/internal/n8n"
	"github.com/Romanxt66/Home_Lab/backend/internal/notifications"
	"github.com/Romanxt66/Home_Lab/backend/internal/scheduler"
	"github.com/Romanxt66/Home_Lab/backend/internal/scraper"
	"github.com/Romanxt66/Home_Lab/backend/internal/uptime"
	"github.com/Romanxt66/Home_Lab/backend/internal/users"
	"github.com/Romanxt66/Home_Lab/backend/internal/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuración incompleta: %v", err)
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("base de datos: %v", err)
	}
	defer db.Close()

	// ── Cableado ───────────────────────────────────────────────────────────
	sessions := auth.NewSessions(cfg.SessionSecret)

	authRepo := auth.NewRepo(db)
	authHandler := auth.NewHandler(authRepo, sessions, cfg.Production)

	// Las notificaciones se cablean antes porque uptime depende de ellas:
	// cuando un monitor se cae, el aviso sale por aquí.
	notificationService := notifications.NewService(
		notifications.NewPostgresRepo(db),
		notifications.NewTelegramSender(),
	)
	notificationHandler := notifications.NewHandler(notificationService)

	inventoryService := inventory.NewService(
		inventory.NewPostgresLocationRepo(db),
		inventory.NewPostgresItemRepo(db),
		inventory.NewPostgresMovementRepo(db),
	)
	inventoryHandler := inventory.NewHandler(inventoryService)

	userHandler := users.NewHandler(users.NewService(users.NewPostgresRepo(db)))

	githubHandler := github.NewHandler(github.NewService(
		github.NewPostgresRepo(db),
		github.NewRESTCliente(),
	))

	n8nHandler := n8n.NewHandler(n8n.NewService(
		n8n.NewRESTCliente(),
		n8n.NewPostgresRepo(db),
	))

	// El orden importa: el calendario necesita las notificaciones, las
	// automatizaciones necesitan el calendario, y uptime necesita las
	// automatizaciones para que salten las reglas cuando algo se cae.
	// El calendario avisa por el mismo canal que todo lo demás.
	calendarService := calendar.NewService(
		calendar.NewPostgresRepo(db),
		notifications.NewServicioNotifier(notificationService),
	)
	calendarHandler := calendar.NewHandler(calendarService)

	// Las automatizaciones reaccionan a sucesos de otros módulos. El
	// rellenado de plantillas se le pasa como función para no hacerle
	// depender del módulo de correo, que es donde vive esa lógica.
	automationService := automations.NewService(
		automations.NewPostgresRepo(db),
		notifications.NewServicioNotifier(notificationService),
		&agendaDeAutomatizaciones{svc: calendarService},
		automations.NewHTTPWebhook(),
		email.Rellenar,
	)
	automationHandler := automations.NewHandler(automationService)

	uptimeService := uptime.NewService(
		uptime.NewPostgresMonitorRepo(db),
		uptime.NewPostgresCheckRepo(db),
		uptime.NewHTTPProber(),
		notifications.NewServicioNotifier(notificationService),
		&automatizadorDeUptime{svc: automationService},
	)
	uptimeHandler := uptime.NewHandler(uptimeService)

	cuentasGmail := email.NewPostgresCuentaRepo(db)
	gmailCfg := email.OAuthConfig{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURI:  cfg.GmailRedirectURI,
	}
	emailService := email.NewService(
		email.NewSMTPEnviador(email.SMTPConfig{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort,
			User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom,
		}),
		email.NewGmailEnviador(gmailCfg, cuentasGmail),
		email.NewPostgresPlantillaRepo(db),
		cuentasGmail,
		email.NewPostgresRegistro(db),
	)
	emailHandler := email.NewHandler(emailService)

	// Conectar un buzón es otro flujo de OAuth distinto del de entrar: aquí
	// sí se pide permiso duradero, porque hay que enviar más tarde sin el
	// usuario delante.
	var gmailOAuth *email.OAuthHandler
	if gmailCfg.Configurado() {
		gmailOAuth = email.NewOAuthHandler(gmailCfg, cuentasGmail, sessions, cfg.Production)
	}

	scraperService := scraper.NewService(
		scraper.NewHTTPDescargador(),
		scraper.NewGoqueryExtractor(),
	)
	scraperHandler := scraper.NewHandler(scraperService)

	// El planificador no sabe nada de scraping ni de correo: solo despacha
	// a quien corresponda y anota el resultado.
	schedulerService := scheduler.NewService(
		scheduler.NewPostgresJobRepo(db),
		scheduler.NewPostgresRunRepo(db),
		map[scheduler.Accion]scheduler.Ejecutor{
			scheduler.AccionEmail:  email.EjecutorDeTarea(emailService),
			scheduler.AccionScrape: scraper.EjecutorDeTarea(scraperService),
		},
	)
	schedulerHandler := scheduler.NewHandler(schedulerService)

	// Entrar con Google es opcional: sin credenciales configuradas la
	// aplicación funciona igual, solo que no se ofrece ese camino. Por eso
	// el handler puede quedarse en nil.
	googleCfg := auth.GoogleConfig{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURI:  cfg.GoogleRedirectURI,
	}
	var googleHandler *auth.GoogleHandler
	if googleCfg.Configurado() {
		googleHandler = auth.NewGoogleHandler(
			auth.NewGoogleClient(googleCfg),
			auth.NewGoogleLogin(
				authRepo,
				notifications.NewServicioNotifier(notificationService),
				&automatizadorDeAuth{svc: automationService},
			),
			sessions,
			cfg.Production,
		)
	} else {
		log.Println("acceso con Google desactivado: faltan GOOGLE_CLIENT_ID, " +
			"GOOGLE_CLIENT_SECRET o GOOGLE_LOGIN_REDIRECT_URI")
	}

	// ── Rutas ──────────────────────────────────────────────────────────────
	if cfg.Production {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), httpx.CORS(cfg.AllowedOrigins))

	r.GET("/health", func(c *gin.Context) {
		if err := db.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"estado": "sin base de datos"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"estado": "ok"})
	})

	api := r.Group("/api")

	// Públicas: iniciar y cerrar sesión, y el ida y vuelta con Google.
	publicas := api.Group("/auth")
	authHandler.Register(publicas)
	if googleHandler != nil {
		googleHandler.Register(publicas)
	}

	// La interfaz pregunta por aquí si puede enseñar el botón de Google.
	api.GET("/auth/proveedores", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"google": googleHandler != nil})
	})

	// Protegidas: todo lo demás.
	privadas := api.Group("")
	privadas.Use(httpx.RequireAuth(sessions))
	privadas.GET("/auth/me", authHandler.Me)
	uptimeHandler.Register(privadas.Group("/uptime"))
	notificationHandler.Register(privadas.Group("/notifications"))
	inventoryHandler.Register(privadas.Group("/inventory"))
	scraperHandler.Register(privadas.Group("/scraper"))
	n8nHandler.Register(privadas.Group("/n8n"))
	githubHandler.Register(privadas.Group("/github"))
	schedulerHandler.Register(privadas.Group("/scheduler"))
	emailHandler.Register(privadas.Group("/email"))
	calendarHandler.Register(privadas.Group("/calendar"))
	automationHandler.Register(privadas.Group("/automations"))
	if gmailOAuth != nil {
		gmailOAuth.Register(privadas.Group("/email"))
	}

	// Administrar cuentas es cosa del superadmin, y el grupo lo exige antes
	// de que el módulo vea la petición.
	soloAdmin := privadas.Group("")
	soloAdmin.Use(httpx.RequireRole(sessions, "superadmin"))
	userHandler.Register(soloAdmin.Group("/users"))

	// Disparador del cron. No lleva sesión porque lo llama una máquina; se
	// protege con un secreto y, si no está configurado, falla cerrado.
	cron := cronAuth(os.Getenv("CRON_SECRET"), cfg.Production)
	r.POST("/api/cron/uptime", cron, uptimeHandler.RunDue)
	r.POST("/api/cron/scheduler", cron, schedulerHandler.RunDue)

	// La interfaz, si se incrustó al compilar. En desarrollo no está y la
	// sirve Vite en su propio puerto; en producción van juntas y por eso no
	// hace falta CORS.
	if interfaz := web.Handler(); interfaz != nil {
		r.NoRoute(func(c *gin.Context) {
			// Una ruta /api que no existe es un 404 de API, no la interfaz:
			// devolver el index.html ahí confundiría a quien llama.
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "Ruta no encontrada."})
				return
			}
			interfaz.ServeHTTP(c.Writer, c.Request)
		})
		log.Println("interfaz incrustada: se sirve desde el propio binario")
	}

	// ── El reloj interno ───────────────────────────────────────────────────
	// El binario está siempre levantado, así que una gorutina basta y no hace
	// falta que nadie llame al endpoint desde fuera.
	//
	// Viene apagado a propósito: mientras el Lab siga desplegado, su propio
	// cron está ejecutando estas mismas tareas sobre la misma base, y los dos
	// a la vez mandarían cada aviso dos veces.
	ctxReloj, pararReloj := context.WithCancel(context.Background())
	defer pararReloj()

	if os.Getenv("SCHEDULER_INTERNO") == "1" {
		// La tarea de uptime se arma aquí y no dentro del módulo, para que
		// uptime no tenga que conocer al planificador.
		tareaUptime := scheduler.Tarea{
			Nombre: "monitores de uptime",
			Correr: func(ctx context.Context) error {
				resumen, err := uptimeService.RunDueChecks(ctx, time.Now())
				if err != nil {
					return err
				}
				if resumen.Down > 0 || resumen.Recovered > 0 {
					log.Printf("uptime: %d comprobados, %d caídos, %d recuperados",
						resumen.Checked, resumen.Down, resumen.Recovered)
				}
				return nil
			},
		}

		tareaRecordatorios := scheduler.Tarea{
			Nombre: "recordatorios del calendario",
			Correr: func(ctx context.Context) error {
				resumen, err := calendarService.ProcesarAvisos(ctx, time.Now())
				if err != nil {
					return err
				}
				if resumen.Enviados > 0 || resumen.Fallidos > 0 {
					log.Printf("calendario: %d recordatorios enviados, %d fallidos",
						resumen.Enviados, resumen.Fallidos)
				}
				return nil
			},
		}

		go scheduler.NewTicker(
			schedulerService.TareaDeJobs(),
			tareaUptime,
			tareaRecordatorios,
		).Arrancar(ctxReloj)
		log.Println("reloj interno encendido: tareas y uptime cada minuto")
	} else {
		log.Println("reloj interno apagado (SCHEDULER_INTERNO=1 para encenderlo)")
	}

	// ── Arranque y apagado ordenado ────────────────────────────────────────
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("API escuchando en http://localhost:%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("el servidor se detuvo: %v", err)
		}
	}()

	// Al recibir la señal de parada se deja terminar lo que esté en curso en
	// vez de cortar a mitad de una comprobación.
	parada := make(chan os.Signal, 1)
	signal.Notify(parada, syscall.SIGINT, syscall.SIGTERM)
	<-parada

	log.Println("apagando…")
	pararReloj()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("apagado forzado: %v", err)
	}
}

// cronAuth exige el secreto del cron. Sin secreto configurado, en producción
// no se ejecuta: es preferible que el cron no corra a que corra abierto.
func cronAuth(secreto string, produccion bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secreto == "" {
			if produccion {
				c.AbortWithStatusJSON(http.StatusUnauthorized,
					gin.H{"error": "CRON_SECRET no configurado"})
				return
			}
			c.Next()
			return
		}

		if c.GetHeader("Authorization") != "Bearer "+secreto {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				gin.H{"error": "No autorizado"})
			return
		}
		c.Next()
	}
}

// agendaDeAutomatizaciones adapta el calendario a lo que esperan las
// automatizaciones. Vive aquí, en la raíz de composición, para que ninguno
// de los dos módulos tenga que conocer al otro.
type agendaDeAutomatizaciones struct {
	svc *calendar.Service
}

func (a *agendaDeAutomatizaciones) CrearEvento(ctx context.Context, titulo string, descripcion *string, cuando time.Time) error {
	res, err := a.svc.Crear(ctx, calendar.Evento{
		Title:       titulo,
		Description: descripcion,
		Start:       cuando,
		AllDay:      true,
	})
	if err != nil {
		return err
	}
	if !res.OK {
		return errors.New(res.Error)
	}
	return nil
}

// automatizadorDeAuth adapta las automatizaciones a lo que espera auth, que
// no conoce el tipo Disparador. Otra pieza de pegamento que se queda aquí.
type automatizadorDeAuth struct {
	svc *automations.Service
}

func (a *automatizadorDeAuth) Disparar(ctx context.Context, disparador string, valores map[string]string) {
	a.svc.Disparar(ctx, automations.Disparador(disparador), valores)
}

// automatizadorDeUptime es el mismo pegamento que automatizadorDeAuth, para
// que uptime tampoco tenga que conocer el tipo Disparador.
type automatizadorDeUptime struct {
	svc *automations.Service
}

func (a *automatizadorDeUptime) Disparar(ctx context.Context, disparador string, valores map[string]string) {
	a.svc.Disparar(ctx, automations.Disparador(disparador), valores)
}
