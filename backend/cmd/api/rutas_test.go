package main

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/assistant"
	"github.com/Romanxt66/Home_Lab/backend/internal/auth"
	"github.com/Romanxt66/Home_Lab/backend/internal/automations"
	"github.com/Romanxt66/Home_Lab/backend/internal/calendar"
	"github.com/Romanxt66/Home_Lab/backend/internal/coolify"
	"github.com/Romanxt66/Home_Lab/backend/internal/dbadmin"
	"github.com/Romanxt66/Home_Lab/backend/internal/email"
	"github.com/Romanxt66/Home_Lab/backend/internal/github"
	"github.com/Romanxt66/Home_Lab/backend/internal/notifications"
	"github.com/Romanxt66/Home_Lab/backend/internal/scheduler"
	"github.com/Romanxt66/Home_Lab/backend/internal/scraper"
	"github.com/Romanxt66/Home_Lab/backend/internal/users"
)

// Gin decide el árbol de rutas AL REGISTRARLAS y entra en pánico si dos se
// pisan, por ejemplo dos comodines distintos en la misma posición. Eso pasa
// al arrancar el proceso, no al llamar a la ruta: sin esta prueba, un choque
// entre dos módulos se descubriría en producción, con el contenedor en
// bucle de reinicio.
//
// Los handlers se construyen con servicios nil a propósito: registrar rutas
// no los toca, y así la prueba no necesita base de datos.
func TestNingunaRutaChocaConOtra(t *testing.T) {
	gin.SetMode(gin.TestMode)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("hay rutas que se pisan: %v", r)
		}
	}()

	r := gin.New()
	api := r.Group("/api")

	publicas := api.Group("/auth")
	auth.NewHandler(nil, nil, false).Register(publicas)
	auth.NewGoogleHandler(nil, nil, nil, false).Register(publicas)
	api.GET("/auth/proveedores", func(*gin.Context) {})

	privadas := api.Group("")
	privadas.GET("/auth/me", func(*gin.Context) {})

	notifications.NewHandler(nil).Register(privadas.Group("/notifications"))
	scraper.NewHandler(nil).Register(privadas.Group("/scraper"))
	scheduler.NewHandler(nil).Register(privadas.Group("/scheduler"))
	email.NewHandler(nil).Register(privadas.Group("/email"))
	email.NewOAuthHandler(email.OAuthConfig{}, nil, nil, false).Register(privadas.Group("/email"))
	calendar.NewHandler(nil).Register(privadas.Group("/calendar"))
	automations.NewHandler(nil).Register(privadas.Group("/automations"))
	github.NewHandler(nil).Register(privadas.Group("/github"))
	coolify.NewHandler(nil).Register(privadas.Group("/coolify"))
	dbadmin.NewHandler(nil).Register(privadas.Group("/db-admin"))
	assistant.NewHandler(nil).Register(privadas.Group("/assistant"))
	users.NewHandler(nil).Register(privadas.Group("/users"))

	r.POST("/api/cron/scheduler", func(*gin.Context) {})

	rutas := r.Routes()
	if len(rutas) < 40 {
		t.Errorf("solo se registraron %d rutas; ¿falta cablear algún módulo?", len(rutas))
	}

	// Ninguna ruta puede estar duplicada con el mismo método.
	vistas := map[string]bool{}
	for _, ru := range rutas {
		clave := ru.Method + " " + ru.Path
		if vistas[clave] {
			t.Errorf("ruta duplicada: %s", clave)
		}
		vistas[clave] = true
	}

	t.Logf("%d rutas registradas sin conflictos", len(rutas))
}
