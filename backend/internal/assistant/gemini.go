package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

const (
	apiGemini       = "https://generativelanguage.googleapis.com/v1beta/models"
	maxTokensSalida = 1024
	tiempoLimite    = 30 * time.Second

	// 429 y 503 son transitorios en el plan gratuito: reintentar casi
	// siempre funciona. El resto de errores no mejoran por insistir.
	maxIntentos = 3
	esperaBase  = 700 * time.Millisecond
)

type GeminiConfig struct {
	APIKey string
	Model  string
}

func (c GeminiConfig) Configurado() bool { return c.APIKey != "" }

// GeminiCliente habla con generateContent por REST, sin SDK.
type GeminiCliente struct {
	cfg    GeminiConfig
	client *http.Client
	ids    atomic.Int64
	// espera existe para poder acortar los reintentos en las pruebas.
	espera func(time.Duration)
	base   string
}

func NewGeminiCliente(cfg GeminiConfig) *GeminiCliente {
	if cfg.Model == "" {
		cfg.Model = "gemini-flash-latest"
	}
	return &GeminiCliente{
		cfg: cfg, client: &http.Client{Timeout: tiempoLimite},
		espera: time.Sleep, base: apiGemini,
	}
}

// ── La forma de Gemini en la red ───────────────────────────────────────────

type parteGemini struct {
	Text             string           `json:"text,omitempty"`
	FunctionCall     *llamadaGemini   `json:"functionCall,omitempty"`
	FunctionResponse *respuestaGemini `json:"functionResponse,omitempty"`
	// Los modelos con razonamiento exigen recibir esto de vuelta tal cual.
	ThoughtSignature string `json:"thoughtSignature,omitempty"`
}

type llamadaGemini struct {
	Name string         `json:"name"`
	ID   string         `json:"id,omitempty"`
	Args map[string]any `json:"args,omitempty"`
}

type respuestaGemini struct {
	Name     string         `json:"name"`
	ID       string         `json:"id,omitempty"`
	Response map[string]any `json:"response"`
}

type contenidoGemini struct {
	Role  string        `json:"role"`
	Parts []parteGemini `json:"parts"`
}

type cuerpoGemini struct {
	Candidates []struct {
		Content      contenidoGemini `json:"content"`
		FinishReason string          `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ── Traducción ─────────────────────────────────────────────────────────────

func rolGemini(r Rol) string {
	if r == RolAsistente {
		return "model"
	}
	return "user"
}

// aParte traduce un bloque. La firma, si la hay, va como hermana del bloque
// al que acompañaba: si se pierde, Gemini rechaza el siguiente turno con
// herramientas.
func aParte(b Bloque) parteGemini {
	switch b.Tipo {
	case BloqueHerramienta:
		return parteGemini{
			FunctionCall:     &llamadaGemini{Name: b.Nombre, ID: b.ID, Args: b.Entrada},
			ThoughtSignature: b.Firma,
		}
	case BloqueResultado:
		return parteGemini{FunctionResponse: &respuestaGemini{
			Name: b.Nombre, ID: b.IDDeUso,
			Response: map[string]any{"content": b.Contenido},
		}}
	default:
		return parteGemini{Text: b.Texto, ThoughtSignature: b.Firma}
	}
}

func aContenidos(mensajes []MensajeLLM) []contenidoGemini {
	salida := make([]contenidoGemini, 0, len(mensajes))
	for _, m := range mensajes {
		c := contenidoGemini{Role: rolGemini(m.Role)}

		if len(m.Bloques) == 0 {
			c.Parts = []parteGemini{{Text: m.Texto}}
		} else {
			for _, b := range m.Bloques {
				c.Parts = append(c.Parts, aParte(b))
			}
		}

		salida = append(salida, c)
	}
	return salida
}

func (g *GeminiCliente) aBloque(p parteGemini) (Bloque, bool) {
	switch {
	case p.FunctionCall != nil:
		id := p.FunctionCall.ID
		if id == "" {
			// Gemini no siempre manda identificador y hace falta uno para
			// emparejar la petición con su resultado.
			id = "llamada_" + strconv.FormatInt(g.ids.Add(1), 10)
		}
		entrada := p.FunctionCall.Args
		if entrada == nil {
			entrada = map[string]any{}
		}
		return Bloque{
			Tipo: BloqueHerramienta, ID: id, Nombre: p.FunctionCall.Name,
			Entrada: entrada, Firma: p.ThoughtSignature,
		}, true

	case p.Text != "":
		return Bloque{Tipo: BloqueTexto, Texto: p.Text, Firma: p.ThoughtSignature}, true
	}

	return Bloque{}, false
}

// ── Envío ──────────────────────────────────────────────────────────────────

func (g *GeminiCliente) Enviar(ctx context.Context, p PeticionLLM) kernel.Result[RespuestaLLM] {
	if !g.cfg.Configurado() {
		return kernel.Err[RespuestaLLM](
			"El asistente no está configurado. Define GEMINI_API_KEY en las variables de entorno.")
	}

	cuerpo := map[string]any{
		"systemInstruction": map[string]any{"parts": []map[string]string{{"text": p.Sistema}}},
		"contents":          aContenidos(p.Mensajes),
		"generationConfig":  map[string]any{"maxOutputTokens": maxTokensSalida},
	}

	if len(p.Herramientas) > 0 {
		declaraciones := make([]map[string]any, 0, len(p.Herramientas))
		for _, h := range p.Herramientas {
			declaraciones = append(declaraciones, map[string]any{
				"name": h.Nombre, "description": h.Descripcion,
				"parameters": json.RawMessage(h.Esquema),
			})
		}
		cuerpo["tools"] = []map[string]any{{"functionDeclarations": declaraciones}}
	}

	datos, err := json.Marshal(cuerpo)
	if err != nil {
		return kernel.Err[RespuestaLLM]("No se pudo preparar la petición al asistente.")
	}

	var ultima kernel.Result[RespuestaLLM]
	for intento := 0; intento < maxIntentos; intento++ {
		res, reintentable := g.intentar(ctx, datos)
		if res.OK || !reintentable {
			return res
		}

		ultima = res
		if intento < maxIntentos-1 {
			// Espera exponencial: 700 ms, 1,4 s…
			g.espera(esperaBase * time.Duration(1<<intento))
		}
	}

	return ultima
}

func (g *GeminiCliente) intentar(ctx context.Context, datos []byte) (kernel.Result[RespuestaLLM], bool) {
	destino := fmt.Sprintf("%s/%s:generateContent", g.base, g.cfg.Model)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, destino, bytes.NewReader(datos))
	if err != nil {
		return kernel.Err[RespuestaLLM]("No se pudo preparar la petición al asistente."), false
	}
	// La clave va en una cabecera y no en la URL: las URL acaban en los
	// registros de proxies y servidores.
	req.Header.Set("x-goog-api-key", g.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := g.client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "deadline exceeded") || strings.Contains(err.Error(), "Client.Timeout") {
			return kernel.Err[RespuestaLLM]("El asistente tardó más de 30 segundos en responder."), false
		}
		return kernel.Err[RespuestaLLM]("No se pudo conectar con el asistente."), true
	}
	defer res.Body.Close()

	crudo, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))

	var cuerpo cuerpoGemini
	_ = json.Unmarshal(crudo, &cuerpo)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		reintentable := res.StatusCode == http.StatusTooManyRequests ||
			res.StatusCode == http.StatusServiceUnavailable

		if reintentable {
			return kernel.Err[RespuestaLLM](
				"El asistente está saturado ahora mismo. Inténtalo de nuevo en un momento."), true
		}

		mensaje := cuerpo.Error.Message
		if mensaje == "" {
			mensaje = fmt.Sprintf("la API de Gemini respondió HTTP %d.", res.StatusCode)
		}
		return kernel.Err[RespuestaLLM]("Asistente: " + mensaje), false
	}

	if cuerpo.PromptFeedback.BlockReason != "" {
		return kernel.Err[RespuestaLLM](
			"Gemini bloqueó la respuesta (" + cuerpo.PromptFeedback.BlockReason + ")."), false
	}

	var bloques []Bloque
	usaHerramientas := false

	if len(cuerpo.Candidates) > 0 {
		for _, parte := range cuerpo.Candidates[0].Content.Parts {
			if b, ok := g.aBloque(parte); ok {
				bloques = append(bloques, b)
				if b.Tipo == BloqueHerramienta {
					usaHerramientas = true
				}
			}
		}
	}

	return kernel.Ok(RespuestaLLM{Bloques: bloques, UsaHerramientas: usaHerramientas}), false
}
