package workspace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/tokens"
)

type autPorToken map[string]tokens.Identidad

func (a autPorToken) Autenticar(_ context.Context, claro string) (*tokens.Identidad, error) {
	if id, ok := a[claro]; ok {
		return &id, nil
	}
	return nil, nil
}

func llamarSync(t *testing.T, motor *gin.Engine, ruta, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/sync"+ruta, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	motor.ServeHTTP(rec, req)
	return rec
}

func TestSync(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(nuevoRepo())
	ctx := context.Background()
	z := zipDe(t, map[string]string{"SKILL.md": skillMD, "assets/logo.bin": "\x00\x01\x02binario"})

	propia, _ := svc.SubirSkill(ctx, ana, z, "", Privado)
	ajena, _ := svc.SubirSkill(ctx, luis, z, "", Compartido)
	ajenaPrivada, _ := svc.SubirSkill(ctx, luis, zipDe(t, map[string]string{"SKILL.md": "---\nname: secreta\ndescription: x\n---\n"}), "", Privado)
	_, _ = svc.GuardarDoc(ctx, ana, "a.md", "# doc", "", Privado)

	motor := gin.New()
	NewSyncHandler(svc, autPorToken{
		"hl_ana":  {UsuarioID: "ana", Nombre: "Ana", Alcance: tokens.Lectura},
		"hl_luis": {UsuarioID: "luis", Nombre: "Luis", Alcance: tokens.Lectura},
	}).Register(motor.Group("/api/sync"))

	// Sin token o con uno falso: 401 con la cabecera que lo explica.
	for _, tok := range []string{"", "hl_falso"} {
		rec := llamarSync(t, motor, "/manifest", tok)
		if rec.Code != 401 || !strings.Contains(rec.Header().Get("WWW-Authenticate"), "Bearer") {
			t.Errorf("token %q: %d %v", tok, rec.Code, rec.Header())
		}
	}

	// El manifiesto solo trae lo propio: ni lo compartido por otra persona ni los documentos.
	rec := llamarSync(t, motor, "/manifest", "hl_ana")
	var m struct {
		Skills []itemSync `json:"skills"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	if rec.Code != 200 || len(m.Skills) != 1 || m.Skills[0].ID != propia.Value.ID || m.Skills[0].Version != 1 || m.Skills[0].Name != "mi-skill" {
		t.Fatalf("manifiesto de Ana: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("el manifiesto no debe cachearse: dice qué hay de nuevo")
	}

	// Descarga de lo propio: zip con los binarios dentro y la versión en una cabecera.
	rec = llamarSync(t, motor, "/skills/"+propia.Value.ID, "hl_ana")
	if rec.Code != 200 || rec.Header().Get("X-Skill-Version") != "1" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("descarga: %d %v", rec.Code, rec.Header())
	}
	leida := LeerSkill(rec.Body.Bytes())
	if !leida.OK || len(leida.Value.Archivos) != 2 {
		t.Errorf("el zip bajado debe conservar los binarios: %+v", leida)
	}

	// Lo ajeno no se baja por aquí aunque esté compartido y se conozca su id.
	for nombre, id := range map[string]string{"compartida": ajena.Value.ID, "privada": ajenaPrivada.Value.ID, "inexistente": "no-existe"} {
		if rec := llamarSync(t, motor, "/skills/"+id, "hl_ana"); rec.Code != 404 {
			t.Errorf("la skill %s de otra persona: esperaba 404 y fue %d", nombre, rec.Code)
		}
	}
	// Y cada cual baja la suya.
	if rec := llamarSync(t, motor, "/skills/"+ajena.Value.ID, "hl_luis"); rec.Code != 200 {
		t.Errorf("Luis baja la suya: %d", rec.Code)
	}
}
