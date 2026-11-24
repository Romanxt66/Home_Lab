package workspace

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/auth"
)

// El mismo envoltorio que RequireAuth: deja la sesión en el contexto con la clave
// que lee httpx.CurrentUser.
func conSesion(uid, rol string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("usuario", &auth.SessionPayload{UID: uid, Email: uid + "@x.com", Role: rol})
		c.Next()
	}
}

func servidor(t *testing.T) (*gin.Engine, func(uid, rol, metodo, ruta string, cuerpo *bytes.Buffer, tipo string) *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := baseDePrueba(t)
	h := NewHandler(NewService(NewPostgresRepo(db)))

	motor := gin.New()
	ws := motor.Group("/api/workspace", func(c *gin.Context) {
		// La persona de la petición va en cabeceras de prueba.
		conSesion(c.GetHeader("X-Uid"), c.GetHeader("X-Rol"))(c)
	})
	h.Register(ws)

	llamar := func(uid, rol, metodo, ruta string, cuerpo *bytes.Buffer, tipo string) *httptest.ResponseRecorder {
		if cuerpo == nil {
			cuerpo = &bytes.Buffer{}
		}
		req := httptest.NewRequest(metodo, "/api/workspace"+ruta, cuerpo)
		req.Header.Set("X-Uid", uid)
		req.Header.Set("X-Rol", rol)
		if tipo != "" {
			req.Header.Set("Content-Type", tipo)
		}
		rec := httptest.NewRecorder()
		motor.ServeHTTP(rec, req)
		return rec
	}
	return motor, llamar
}

func multipartDe(t *testing.T, datos []byte, campos map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "x.skill")
	_, _ = fw.Write(datos)
	for k, v := range campos {
		_ = mw.WriteField(k, v)
	}
	_ = mw.Close()
	return &buf, mw.FormDataContentType()
}

func TestHTTP_flujoCompleto(t *testing.T) {
	_, llamar := servidor(t)

	// Subir. Si hay una skill real a mano, se usa ella; si no, una generada.
	datos := zipDe(t, map[string]string{"mi-skill/SKILL.md": skillMD, "mi-skill/assets/a.txt": "x"})
	if ruta := os.Getenv("WORKSPACE_TEST_SKILL"); ruta != "" {
		b, err := os.ReadFile(ruta)
		if err != nil {
			t.Fatal(err)
		}
		datos = b
	}
	cuerpo, tipo := multipartDe(t, datos, map[string]string{"message": "primera", "visibility": "private"})
	rec := llamar("ana", "user", http.MethodPost, "/skills", cuerpo, tipo)
	if rec.Code != 200 {
		t.Fatalf("subir: %d %s", rec.Code, rec.Body)
	}
	var item elementoDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &item)
	if item.Kind != "skill" || !item.Mine || item.Version != 1 || item.Visibility != "private" {
		t.Fatalf("elemento: %+v", item)
	}

	// Luis no la ve (404, igual que si no existiera) hasta que Ana la comparte.
	if rec := llamar("luis", "user", http.MethodGet, "/items/"+item.ID, nil, ""); rec.Code != 404 {
		t.Errorf("privada, Luis: esperaba 404 y fue %d", rec.Code)
	}
	if rec := llamar("luis", "user", http.MethodGet, "/items/"+item.ID+"/download", nil, ""); rec.Code != 404 {
		t.Errorf("descarga de lo privado: esperaba 404 y fue %d", rec.Code)
	}
	if rec := llamar("luis", "user", http.MethodPut, "/items/"+item.ID+"/visibility", bytes.NewBufferString(`{"visibility":"shared"}`), "application/json"); rec.Code != 404 {
		t.Errorf("Luis no puede ni saber que existe: %d", rec.Code)
	}
	if rec := llamar("ana", "user", http.MethodPut, "/items/"+item.ID+"/visibility", bytes.NewBufferString(`{"visibility":"shared"}`), "application/json"); rec.Code != 204 {
		t.Fatalf("compartir: %d %s", rec.Code, rec.Body)
	}

	// Compartida: Luis la ve y la baja, pero no la toca.
	if rec := llamar("luis", "user", http.MethodGet, "/items/"+item.ID, nil, ""); rec.Code != 200 {
		t.Fatalf("compartida, Luis: %d", rec.Code)
	}
	if rec := llamar("luis", "user", http.MethodPut, "/items/"+item.ID+"/visibility", bytes.NewBufferString(`{"visibility":"private"}`), "application/json"); rec.Code != 403 {
		t.Errorf("Luis cambiando lo de Ana: esperaba 403 y fue %d", rec.Code)
	}
	if rec := llamar("luis", "user", http.MethodDelete, "/items/"+item.ID, nil, ""); rec.Code != 403 {
		t.Errorf("Luis borrando lo de Ana: esperaba 403 y fue %d", rec.Code)
	}

	// Descargar: sale con cabeceras seguras y el zip se puede volver a subir.
	rec = llamar("luis", "user", http.MethodGet, "/items/"+item.ID+"/download", nil, "")
	if rec.Code != 200 || rec.Header().Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(rec.Header().Get("Content-Disposition"), ".skill") {
		t.Fatalf("descarga: %d %v", rec.Code, rec.Header())
	}
	if relee := LeerSkill(rec.Body.Bytes()); !relee.OK {
		t.Errorf("el zip descargado debe volver a leerse: %s", relee.Error)
	}

	// Vista previa de SKILL.md y rechazo de rutas que no existen.
	rec = llamar("ana", "user", http.MethodGet, "/items/"+item.ID+"/file?path=SKILL.md", nil, "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Mi skill") && os.Getenv("WORKSPACE_TEST_SKILL") == "" {
		t.Errorf("vista previa: %d %s", rec.Code, rec.Body)
	}
	if rec := llamar("ana", "user", http.MethodGet, "/items/"+item.ID+"/file?path=../../etc/passwd", nil, ""); rec.Code != 404 {
		t.Errorf("ruta inexistente: esperaba 404 y fue %d", rec.Code)
	}

	// Un id que no es un uuid no rompe nada.
	if rec := llamar("ana", "user", http.MethodGet, "/items/no-es-uuid", nil, ""); rec.Code != 404 {
		t.Errorf("id raro: esperaba 404 y fue %d", rec.Code)
	}

	// Descarga masiva: el zip lleva la skill de Ana en skills/.
	rec = llamar("luis", "user", http.MethodGet, "/download?scope=all", nil, "")
	if rec.Code != 200 {
		t.Fatalf("descarga masiva: %d %s", rec.Code, rec.Body)
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil || len(zr.File) == 0 {
		t.Fatalf("zip masivo: %v", err)
	}
	for _, f := range zr.File {
		if strings.Contains(f.Name, "..") || strings.HasPrefix(f.Name, "/") {
			t.Errorf("ruta peligrosa en el zip: %s", f.Name)
		}
	}

	// Un administrador puede retirarla.
	if rec := llamar("root", "superadmin", http.MethodDelete, "/items/"+item.ID, nil, ""); rec.Code != 204 {
		t.Errorf("admin borrando: %d", rec.Code)
	}
}

func TestHTTP_documentosYErrores(t *testing.T) {
	_, llamar := servidor(t)

	rec := llamar("ana", "user", http.MethodPost, "/docs", bytes.NewBufferString(`{"path":"proyectos/x/CLAUDE.md","content":"# Reglas\nuna"}`), "application/json")
	if rec.Code != 200 {
		t.Fatalf("crear doc: %d %s", rec.Code, rec.Body)
	}
	var d elementoDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &d)
	if d.Kind != "doc" || d.Description != "Reglas" {
		t.Errorf("doc: %+v", d)
	}

	// Rutas malas → 400 con un mensaje legible.
	for _, mala := range []string{`{"path":"../x.md","content":"a"}`, `{"path":"notas.txt","content":"a"}`, `{"path":"","content":"a"}`} {
		rec := llamar("ana", "user", http.MethodPost, "/docs", bytes.NewBufferString(mala), "application/json")
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), "error") {
			t.Errorf("%s → %d %s", mala, rec.Code, rec.Body)
		}
	}

	// El documento se baja como .md.
	rec = llamar("ana", "user", http.MethodGet, "/items/"+d.ID+"/download", nil, "")
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), "CLAUDE.md") || !strings.Contains(rec.Body.String(), "una") {
		t.Errorf("descarga del doc: %d %v", rec.Code, rec.Header())
	}

	// Un zip malo y un tipo de listado inválido.
	cuerpo, tipo := multipartDe(t, []byte("no soy un zip"), nil)
	if rec := llamar("ana", "user", http.MethodPost, "/skills", cuerpo, tipo); rec.Code != 400 {
		t.Errorf("zip malo: esperaba 400 y fue %d", rec.Code)
	}
	if rec := llamar("ana", "user", http.MethodGet, "/items?kind=otra", nil, ""); rec.Code != 400 {
		t.Errorf("kind inválido: %d", rec.Code)
	}
	if rec := llamar("ana", "user", http.MethodPost, "/skills", bytes.NewBufferString("sin multipart"), "text/plain"); rec.Code != 400 {
		t.Errorf("sin archivo: %d", rec.Code)
	}
}
