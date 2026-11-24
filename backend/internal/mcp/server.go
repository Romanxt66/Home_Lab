// Package mcp es un servidor del Model Context Protocol por HTTP («Streamable
// HTTP», sin flujos del servidor): un solo endpoint al que el cliente manda
// peticiones JSON-RPC y recibe la respuesta en el mismo cuerpo.
//
// Solo ofrece herramientas (tools). No sabe nada de skills ni de documentos:
// quien lo monta le da la lista de herramientas y quién puede usarlas.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"unicode/utf8"
)

// Versiones del protocolo que se entienden, de la más nueva a la más antigua.
var versiones = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

// MaxBytesRespuesta limita lo que una herramienta devuelve al modelo: un
// documento enorme se corta, con aviso, en vez de llenarle el contexto.
const MaxBytesRespuesta = 100_000

// Caller es la persona en cuyo nombre se llama.
type Caller struct {
	UserID   string
	Name     string
	Admin    bool
	CanWrite bool
}

// Tool es una herramienta que el modelo puede invocar.
type Tool struct {
	Name        string
	Description string
	// Schema es el JSON Schema de los argumentos.
	Schema json.RawMessage
	// ReadOnly: no cambia nada. Las que no lo son solo las ven y usan los
	// tokens con permiso de escritura.
	ReadOnly bool
	Handler  func(ctx context.Context, who Caller, args json.RawMessage) (string, error)
}

// ToolError es un fallo que el modelo debe leer y puede corregir («no existe
// esa skill»). Cualquier otro error se registra y se le da un mensaje genérico.
type ToolError string

func (e ToolError) Error() string { return string(e) }

// Server resuelve las peticiones JSON-RPC.
type Server struct {
	name, version, instructions string
	tools                       []Tool
	index                       map[string]Tool
}

func NewServer(name, version, instructions string, tools []Tool) *Server {
	idx := make(map[string]Tool, len(tools))
	for _, t := range tools {
		idx[t.Name] = t
	}
	return &Server{name: name, version: version, instructions: instructions, tools: tools, index: idx}
}

// ── JSON-RPC ───────────────────────────────────────────────────────────────

type peticion struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type respuesta struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *errorRPC       `json:"error,omitempty"`
}

type errorRPC struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Códigos de error de JSON-RPC.
const (
	codigoJSONInvalido      = -32700
	codigoPeticionInvalida  = -32600
	codigoMetodoDesconocido = -32601
	codigoParametros        = -32602
)

func fallo(id json.RawMessage, codigo int, msg string) []byte {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	b, _ := json.Marshal(respuesta{JSONRPC: "2.0", ID: id, Error: &errorRPC{Code: codigo, Message: msg}})
	return b
}

func exito(id json.RawMessage, resultado any) []byte {
	b, _ := json.Marshal(respuesta{JSONRPC: "2.0", ID: id, Result: resultado})
	return b
}

// Handle procesa un cuerpo JSON-RPC. Devuelve la respuesta, o nil si era una
// notificación (no se contesta: el HTTP responde 202 sin cuerpo).
func (s *Server) Handle(ctx context.Context, who Caller, cuerpo []byte) []byte {
	cuerpo = trimEspacios(cuerpo)
	if len(cuerpo) > 0 && cuerpo[0] == '[' {
		// Los lotes se quitaron del protocolo en 2025-06-18 y complican sin aportar.
		return fallo(nil, codigoPeticionInvalida, "Los lotes de peticiones no están soportados: manda una petición por llamada.")
	}
	var p peticion
	if err := json.Unmarshal(cuerpo, &p); err != nil {
		return fallo(nil, codigoJSONInvalido, "El cuerpo no es JSON válido.")
	}
	if p.JSONRPC != "2.0" || p.Method == "" {
		return fallo(p.ID, codigoPeticionInvalida, "No es una petición JSON-RPC 2.0 válida.")
	}
	if len(p.ID) == 0 { // notificación
		return nil
	}

	switch p.Method {
	case "initialize":
		return exito(p.ID, s.inicializar(p.Params))
	case "ping":
		return exito(p.ID, struct{}{})
	case "tools/list":
		return exito(p.ID, map[string]any{"tools": s.listar(who)})
	case "tools/call":
		return s.llamar(ctx, who, p)
	}
	return fallo(p.ID, codigoMetodoDesconocido, "Método no soportado: "+p.Method)
}

func (s *Server) inicializar(params json.RawMessage) map[string]any {
	var in struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(params, &in)
	// Se responde con la versión del cliente si se conoce; si no, con la más nueva.
	negociada := versiones[0]
	for _, v := range versiones {
		if v == in.ProtocolVersion {
			negociada = v
		}
	}
	return map[string]any{
		"protocolVersion": negociada,
		"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
		"serverInfo":      map[string]any{"name": s.name, "version": s.version},
		"instructions":    s.instructions,
	}
}

func (s *Server) listar(who Caller) []map[string]any {
	salida := make([]map[string]any, 0, len(s.tools))
	for _, t := range s.tools {
		if !t.ReadOnly && !who.CanWrite {
			continue // un token de solo lectura ni ve las herramientas que escriben
		}
		salida = append(salida, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": json.RawMessage(t.Schema),
			"annotations": map[string]any{"readOnlyHint": t.ReadOnly, "destructiveHint": false, "openWorldHint": false},
		})
	}
	return salida
}

func (s *Server) llamar(ctx context.Context, who Caller, p peticion) []byte {
	var in struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(p.Params, &in); err != nil || in.Name == "" {
		return fallo(p.ID, codigoParametros, "Falta el nombre de la herramienta.")
	}
	t, ok := s.index[in.Name]
	if !ok {
		return fallo(p.ID, codigoParametros, "No existe la herramienta «"+in.Name+"».")
	}
	if !t.ReadOnly && !who.CanWrite {
		return exito(p.ID, resultado("Este token es de solo lectura: crea otro con permiso de escritura para guardar cosas.", true))
	}
	if len(in.Arguments) == 0 {
		in.Arguments = json.RawMessage("{}")
	}

	texto, err := ejecutar(ctx, t, who, in.Arguments)
	if err != nil {
		var te ToolError
		if errors.As(err, &te) {
			return exito(p.ID, resultado(string(te), true))
		}
		log.Printf("mcp: la herramienta %s falló: %v", t.Name, err)
		return exito(p.ID, resultado("Ocurrió un error inesperado.", true))
	}
	return exito(p.ID, resultado(texto, false))
}

// ejecutar protege al servidor de un pánico dentro de una herramienta.
func ejecutar(ctx context.Context, t Tool, who Caller, args json.RawMessage) (texto string, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("mcp: pánico en la herramienta %s: %v", t.Name, r)
			err = errors.New("pánico en la herramienta")
		}
	}()
	return t.Handler(ctx, who, args)
}

func resultado(texto string, esError bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": Recortar(texto)}},
		"isError": esError,
	}
}

// Recortar corta un texto largo por un límite de UTF-8 y avisa de que se cortó.
func Recortar(s string) string {
	if len(s) <= MaxBytesRespuesta {
		return s
	}
	corte := MaxBytesRespuesta
	for corte > 0 && !utf8.RuneStart(s[corte]) {
		corte--
	}
	return s[:corte] + "\n\n[… respuesta recortada: pasaba de 100 KB. Pide un archivo más pequeño.]"
}

func trimEspacios(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\n' || b[0] == '\r' || b[0] == '\t') {
		b = b[1:]
	}
	return b
}
