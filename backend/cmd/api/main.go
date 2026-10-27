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
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/auth"
	"github.com/Romanxt66/Home_Lab/backend/internal/config"
	"github.com/Romanxt66/Home_Lab/backend/internal/database"
	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
	"github.com/Romanxt66/Home_Lab/backend/internal/uptime"
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

	uptimeService := uptime.NewService(
		uptime.NewPostgresMonitorRepo(db),
		uptime.NewPostgresCheckRepo(db),
		uptime.NewHTTPProber(),
		&uptime.LogNotifier{Log: log.Printf},
	)
	uptimeHandler := uptime.NewHandler(uptimeService)

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

	// Disparador del cron. No lleva sesión porque lo llama una máquina; se
	// protege con un secreto y, si no está configurado, falla cerrado.
	r.POST("/api/cron/uptime", cronAuth(os.Getenv("CRON_SECRET"), cfg.Production), uptimeHandler.RunDue)

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
