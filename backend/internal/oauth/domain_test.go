package oauth

import (
	"strings"
	"testing"
)

func TestValidarRedirectURI(t *testing.T) {
	hosts := []string{"claude.ai", "claude.com"}
	buenas := map[string]string{
		"https://claude.ai/api/mcp/auth_callback": "https://claude.ai/api/mcp/auth_callback",
		"https://CLAUDE.com/cb?x=1":               "https://CLAUDE.com/cb?x=1",
		"http://localhost:53124/callback":         "http://localhost:53124/callback",
		"http://127.0.0.1:8080/oauth/callback":    "http://127.0.0.1:8080/oauth/callback",
		"http://[::1]:9000/cb":                    "http://[::1]:9000/cb",
	}
	for entrada, esperada := range buenas {
		if got, ok := ValidarRedirectURI(entrada, hosts); !ok || got != esperada {
			t.Errorf("%q debía valer: %q %v", entrada, got, ok)
		}
	}
	malas := []string{
		"", "claude.ai/cb", "ftp://claude.ai/cb", "javascript:alert(1)", "data:text/html,x", "myapp://callback",
		"https://malo.example/cb",             // host no permitido
		"https://claude.ai.malo.example/cb",   // truco del subdominio
		"https://malo.example/claude.ai",      // el nombre solo en la ruta
		"http://claude.ai/cb",                 // http fuera de localhost
		"http://localhost.malo.example/cb",    // «localhost» como subdominio de otro
		"https://localhost/cb",                // https a localhost
		"https://user:clave@claude.ai/cb",     // datos de acceso en la URL
		"https://claude.ai/cb#fragmento",      // fragmento
		"https://claude.ai/cb\nSet-Cookie: x", // inyección de cabeceras
		"https://claude.ai/ cb",
		"https://" + strings.Repeat("a", 600) + ".claude.ai/cb",
		"http://0.0.0.0:80/cb", // no es loopback
		"http://192.168.1.5/cb",
	}
	for _, m := range malas {
		if _, ok := ValidarRedirectURI(m, hosts); ok {
			t.Errorf("%q debía rechazarse", m)
		}
	}
	// El comodín solo abre https, no http fuera de loopback.
	if _, ok := ValidarRedirectURI("https://cualquiera.example/cb", []string{"*"}); !ok {
		t.Error("con «*» vale cualquier host https")
	}
	if _, ok := ValidarRedirectURI("http://cualquiera.example/cb", []string{"*"}); ok {
		t.Error("«*» no permite http fuera de localhost")
	}
}

func TestCoincideRedirect(t *testing.T) {
	casos := []struct {
		registrada, pedida string
		quiere             bool
	}{
		{"https://claude.ai/cb", "https://claude.ai/cb", true},
		{"https://claude.ai/cb", "https://claude.ai/cb/", false},
		{"https://claude.ai/cb", "https://claude.ai/cb?x=1", false},
		{"https://claude.ai/cb", "https://claude.ai:444/cb", false},
		{"https://claude.ai/cb", "http://claude.ai/cb", false},
		{"https://claude.ai/cb", "https://claude.com/cb", false},
		// En loopback el puerto cambia cada vez (RFC 8252, 7.3).
		{"http://localhost:1111/cb", "http://localhost:2222/cb", true},
		{"http://127.0.0.1/cb", "http://127.0.0.1:55555/cb", true},
		{"http://localhost:1111/cb", "http://localhost:1111/otra", false},
		{"http://localhost:1111/cb", "http://127.0.0.1:1111/cb", false}, // otro host, aunque sea loopback
		{"http://localhost/cb", "http://localhost.malo.example/cb", false},
		{"https://claude.ai:1/cb", "https://claude.ai:2/cb", false}, // el puerto libre solo vale en http loopback
	}
	for _, c := range casos {
		if got := CoincideRedirect(c.registrada, c.pedida); got != c.quiere {
			t.Errorf("CoincideRedirect(%q, %q) = %v, quería %v", c.registrada, c.pedida, got, c.quiere)
		}
	}
}

// El vector de prueba del apéndice B de la RFC 7636.
func TestPKCE_vectorDeLaRFC(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if !VerificarPKCE(verifier, challenge) {
		t.Fatal("el vector oficial de la RFC debe verificarse")
	}
	if !ValidarChallenge(challenge) || !ValidarVerifier(verifier) {
		t.Error("y sus formas son válidas")
	}
	if VerificarPKCE(verifier+"x", challenge) || VerificarPKCE(strings.Repeat("a", 43), challenge) {
		t.Error("un verifier distinto no vale")
	}
	// «plain»: usar el challenge como verifier no debe funcionar.
	if VerificarPKCE(challenge, challenge) {
		t.Error("el método plain no se admite")
	}
}

func TestPKCE_formas(t *testing.T) {
	for _, v := range []string{"", strings.Repeat("a", 42), strings.Repeat("a", 129), strings.Repeat("a", 43) + "!", strings.Repeat("a", 43) + " "} {
		if ValidarVerifier(v) {
			t.Errorf("verifier %q no debe valer", v)
		}
	}
	if !ValidarVerifier(strings.Repeat("a", 43)) || !ValidarVerifier(strings.Repeat("A-._~9", 20)) {
		t.Error("estos verifiers sí")
	}
	for _, c := range []string{"", "corto", strings.Repeat("a", 44), strings.Repeat("!", 43)} {
		if ValidarChallenge(c) {
			t.Errorf("challenge %q no debe valer", c)
		}
	}
}

func TestParseScope(t *testing.T) {
	casos := map[string][2]bool{
		"":                        {false, false},
		"mcp:read":                {true, false},
		"mcp:write":               {false, true},
		"mcp:read mcp:write":      {true, true},
		"  mcp:write   mcp:read ": {true, true},
		"admin mcp:root":          {false, false},
		"mcp:readwrite":           {false, false},
	}
	for entrada, esperado := range casos {
		l, e := ParseScope(entrada)
		if l != esperado[0] || e != esperado[1] {
			t.Errorf("ParseScope(%q) = %v,%v", entrada, l, e)
		}
	}
	if ScopeDe(Lectura) != "mcp:read" || ScopeDe(Escritura) != "mcp:read mcp:write" {
		t.Error("ScopeDe")
	}
}

func TestTokens(t *testing.T) {
	a, ha, err := NuevoToken(PrefijoAcceso)
	if err != nil {
		t.Fatal(err)
	}
	if !FormaValida(a, PrefijoAcceso) || FormaValida(a, PrefijoRefresco) {
		t.Errorf("la forma depende del prefijo: %q", a)
	}
	if ha != Huella(a) || ha == a || len(ha) != 64 {
		t.Error("la huella es el SHA-256 hexadecimal, no el secreto")
	}
	if b, _, _ := NuevoToken(PrefijoAcceso); b == a {
		t.Error("dos tokens no coinciden")
	}
	for _, mal := range []string{"", "hlo_", "hlo_corto", a + "x", "hlr_" + a[4:]} {
		if FormaValida(mal, PrefijoAcceso) {
			t.Errorf("%q no debe valer", mal)
		}
	}
}

// El nombre es lo que lee la persona al decidir: no puede llevar trucos.
func TestLimpiarNombre(t *testing.T) {
	casos := map[string]string{
		"Claude":             "Claude",
		"  Mi   app  ":       "Mi app",
		"":                   "Aplicación sin nombre",
		"‮⁦   ":              "Aplicación sin nombre", // solo caracteres de dirección de texto
		"Ban‮co​ Seguro":     "Banco Seguro",          // sin invertir el texto ni ceros de ancho
		"línea1\nlínea2\x00": "línea1línea2",
	}
	for entrada, esperado := range casos {
		if got := LimpiarNombre(entrada); got != esperado {
			t.Errorf("LimpiarNombre(%q) = %q, quería %q", entrada, got, esperado)
		}
	}
	if got := []rune(LimpiarNombre(strings.Repeat("ñ", 200))); len(got) != 81 {
		t.Errorf("se recorta a 80 más la elipsis: %d", len(got))
	}
}
