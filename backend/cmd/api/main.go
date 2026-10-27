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
	"github.com/Romanxt66/Home_Lab/backend/internal/config"
	"github.com/Romanxt66/Home_Lab/backend/internal/database"
	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
	"github.com/Romanxt66/Home_Lab/backend/internal/inventory"
	"github.com/Romanxt66/Home_Lab/backend/internal/notifications"
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

	authHandler := auth.NewHandler(auth.NewRepo(db), sessions, cfg.Production)

	// Las notificaciones se cablean antes porque uptime depende de ellas:
	// cuando un monitor se cae, el aviso sale por aquí.
	notificationService := notifications.NewService(
		notifications.NewPostgresRepo(db),
		notifications.NewTelegramSender(),
	)
	notificationHandler := notifications.NewHandler(notificationService)

	uptimeService := uptime.NewService(
		uptime.NewPostgresMonitorRepo(db),
		uptime.NewPostgresCheckRepo(db),
		uptime.NewHTTPProber(),
		notifications.NewServicioNotifier(notificationService),
	)
	uptimeHandler := uptime.NewHandler(uptimeService)

	inventoryService := inventory.NewService(
		inventory.NewPostgresLocationRepo(db),
		inventory.NewPostgresItemRepo(db),
		inventory.NewPostgresMovementRepo(db),
	)
	inventoryHandler := inventory.NewHandler(inventoryService)

	userHandler := users.NewHandler(users.NewService(users.NewPostgresRepo(db)))

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

	// Públicas: iniciar y cerrar sesión.
	authHandler.Register(api.Group("/auth"))

	// Protegidas: todo lo demás.
	privadas := api.Group("")
	privadas.Use(httpx.RequireAuth(sessions))
	privadas.GET("/auth/me", authHandler.Me)
	uptimeHandler.Register(privadas.Group("/uptime"))
	notificationHandler.Register(privadas.Group("/notifications"))
	inventoryHandler.Register(privadas.Group("/inventory"))

	// Administrar cuentas es cosa del superadmin, y el grupo lo exige antes
	// de que el módulo vea la petición.
	soloAdmin := privadas.Group("")
	soloAdmin.Use(httpx.RequireRole(sessions, "superadmin"))
	userHandler.Register(soloAdmin.Group("/users"))

	// Disparador del cron. No lleva sesión porque lo llama una máquina; se
	// protege con un secreto y, si no está configurado, falla cerrado.
	r.POST("/api/cron/uptime", cronAuth(os.Getenv("CRON_SECRET"), cfg.Production), uptimeHandler.RunDue)

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
