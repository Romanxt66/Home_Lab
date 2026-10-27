// Package assistant es el asistente conversacional: recibe una pregunta,
// decide qué herramientas necesita, las ejecuta y contesta con datos reales.
package assistant

import "encoding/json"

// Rol de un turno de la conversación.
type Rol string

const (
	RolUsuario   Rol = "user"
	RolAsistente Rol = "assistant"
)

// Mensaje es un turno en texto plano: lo único que ve el navegador. Los
// intercambios con herramientas se quedan dentro del bucle del servidor.
type Mensaje struct {
	Role    Rol    `json:"role"`
	Content string `json:"content"`
}

// Respuesta es lo que se devuelve al navegador.
type Respuesta struct {
	Reply      string
	ToolsUsed  []string
	NavigateTo string
}

const (
	MaxMensajesDeHistorial = 20
	MaxLongitudDeMensaje   = 4000
)

// RecortarHistorial se queda con los turnos más recientes y acota cada uno.
//
// Corta por caracteres y no por bytes, para no partir una letra acentuada
// por la mitad, y descarta cualquier rol que no sea de usuario o asistente:
// el historial llega del navegador, así que no se puede fiar de lo que traiga.
func RecortarHistorial(historial []Mensaje) []Mensaje {
	if len(historial) > MaxMensajesDeHistorial {
		historial = historial[len(historial)-MaxMensajesDeHistorial:]
	}

	salida := make([]Mensaje, 0, len(historial))
	for _, m := range historial {
		if m.Role != RolUsuario && m.Role != RolAsistente {
			continue
		}

		contenido := []rune(m.Content)
		if len(contenido) > MaxLongitudDeMensaje {
			contenido = contenido[:MaxLongitudDeMensaje]
		}
		salida = append(salida, Mensaje{Role: m.Role, Content: string(contenido)})
	}

	return salida
}

// ── Lo que se intercambia con el modelo ────────────────────────────────────
//
// Una forma neutral, para que cualquier proveedor pueda traducirla sin que su
// formato de red se filtre al resto del módulo.

type TipoBloque string

const (
	BloqueTexto       TipoBloque = "text"
	BloqueHerramienta TipoBloque = "tool_use"
	BloqueResultado   TipoBloque = "tool_result"
)

// Bloque es un trozo de un mensaje: texto, una petición de herramienta o el
// resultado de una.
type Bloque struct {
	Tipo TipoBloque

	Texto string

	// Para BloqueHerramienta.
	ID      string
	Nombre  string
	Entrada map[string]any

	// Para BloqueResultado.
	IDDeUso   string
	Contenido string

	// Firma opaca que el proveedor entrega y exige recibir de vuelta tal cual
	// en el turno siguiente (las «thought signatures» de Gemini 3). Este
	// módulo no la lee ni la construye: solo la hace viajar.
	Firma string
}

type MensajeLLM struct {
	Role    Rol
	Texto   string // si es solo texto
	Bloques []Bloque
}

type DefinicionDeHerramienta struct {
	Nombre      string
	Descripcion string
	Esquema     json.RawMessage
}

type PeticionLLM struct {
	Sistema      string
	Mensajes     []MensajeLLM
	Herramientas []DefinicionDeHerramienta
}

type RespuestaLLM struct {
	Bloques []Bloque
	// UsaHerramientas indica que el modelo pide ejecutar algo antes de
	// contestar.
	UsaHerramientas bool
}
