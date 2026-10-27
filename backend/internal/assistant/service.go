package assistant

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// MaxRondas es el tope duro del bucle de herramientas: frena a un modelo que
// se enrede encadenando llamadas sin llegar nunca a contestar.
const MaxRondas = 6

const instruccionDelSistema = `Eres el asistente integrado de «Home Lab», un panel personal de herramientas y automatizaciones (calendario, correo, tareas programadas, bases de datos, despliegues, GitHub, usuarios, automatizaciones). Respondes en español, de forma breve y directa.

Usa las herramientas disponibles para contestar con datos reales en vez de inventar cifras. Si necesitas un identificador (de aplicación, conexión, regla…) que no tienes, llama primero a la herramienta de listado correspondiente.

Cuando el usuario pida ver, abrir o ir a una sección, usa navigate_to_module para llevarlo allí y confírmalo en una frase corta.

Antes de ejecutar una acción que modifique datos (crear, aprobar, rechazar, activar o desactivar algo), confirma en tu respuesta anterior qué vas a hacer exactamente, salvo que el usuario ya haya sido explícito e inequívoco. Después de ejecutarla, di claramente qué se hizo.

Si una herramienta devuelve un error de autorización, explícaselo al usuario sin rodeos y no lo intentes de otra forma.`

// Cliente es el modelo de lenguaje.
type Cliente interface {
	Enviar(ctx context.Context, p PeticionLLM) kernel.Result[RespuestaLLM]
}

// Contexto es lo que cada herramienta necesita saber de quién pregunta.
type Contexto struct {
	UID  string
	Role string
	// Efectos son cosas que una herramienta pide que haga el navegador, en
	// lugar de hacerlas ella en el servidor.
	Efectos *Efectos
}

// Efectos acumula lo que las herramientas piden al navegador.
type Efectos struct {
	mu       sync.Mutex
	navegarA string
}

func (e *Efectos) Navegar(destino string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.navegarA = destino
}

func (e *Efectos) Destino() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.navegarA
}

// Herramienta es algo que el modelo puede pedir que se ejecute.
type Herramienta struct {
	Nombre      string
	Descripcion string
	Esquema     string // JSON Schema como texto
	// Ejecutar devuelve el texto que se le entrega al modelo como resultado.
	// Nunca devuelve error: lo que falle se cuenta en el propio texto, que es
	// lo que el modelo sabe leer.
	Ejecutar func(ctx context.Context, entrada map[string]any, c Contexto) string
}

type Service struct {
	cliente      Cliente
	herramientas []Herramienta
}

func NewService(cliente Cliente, herramientas []Herramienta) *Service {
	return &Service{cliente: cliente, herramientas: herramientas}
}

// Conversar ejecuta el bucle: envía la conversación, ejecuta las herramientas
// que el modelo pida, le devuelve los resultados y repite hasta que conteste
// con texto o se llegue al tope de rondas.
func (s *Service) Conversar(ctx context.Context, historial []Mensaje, mensaje string, uid, rol string) kernel.Result[Respuesta] {
	mensaje = strings.TrimSpace(mensaje)
	if mensaje == "" {
		return kernel.Err[Respuesta]("Escribe un mensaje.")
	}
	if len([]rune(mensaje)) > MaxLongitudDeMensaje {
		return kernel.Err[Respuesta]("El mensaje es demasiado largo.")
	}

	efectos := &Efectos{}
	cx := Contexto{UID: uid, Role: rol, Efectos: efectos}

	mensajes := make([]MensajeLLM, 0, len(historial)+1)
	for _, m := range RecortarHistorial(historial) {
		mensajes = append(mensajes, MensajeLLM{Role: m.Role, Texto: m.Content})
	}
	mensajes = append(mensajes, MensajeLLM{Role: RolUsuario, Texto: mensaje})

	definiciones := make([]DefinicionDeHerramienta, 0, len(s.herramientas))
	for _, h := range s.herramientas {
		definiciones = append(definiciones, DefinicionDeHerramienta{
			Nombre: h.Nombre, Descripcion: h.Descripcion, Esquema: []byte(h.Esquema),
		})
	}

	var usadas []string

	for ronda := 0; ronda < MaxRondas; ronda++ {
		res := s.cliente.Enviar(ctx, PeticionLLM{
			Sistema: instruccionDelSistema, Mensajes: mensajes, Herramientas: definiciones,
		})
		if !res.OK {
			return kernel.Err[Respuesta](res.Error)
		}

		if !res.Value.UsaHerramientas {
			texto := textoDe(res.Value.Bloques)
			if texto == "" {
				texto = "(sin respuesta)"
			}
			return kernel.Ok(Respuesta{
				Reply: texto, ToolsUsed: usadas, NavigateTo: efectos.Destino(),
			})
		}

		mensajes = append(mensajes, MensajeLLM{Role: RolAsistente, Bloques: res.Value.Bloques})

		peticiones := peticionesDe(res.Value.Bloques)
		resultados := s.ejecutarEnParalelo(ctx, peticiones, cx)

		bloques := make([]Bloque, 0, len(resultados))
		for i, r := range resultados {
			if r.conocida {
				usadas = append(usadas, peticiones[i].Nombre)
			}
			bloques = append(bloques, Bloque{
				Tipo: BloqueResultado, IDDeUso: peticiones[i].ID,
				Nombre: peticiones[i].Nombre, Contenido: r.texto,
			})
		}
		mensajes = append(mensajes, MensajeLLM{Role: RolUsuario, Bloques: bloques})
	}

	return kernel.Err[Respuesta](
		"El asistente encadenó demasiadas herramientas sin llegar a responder. Inténtalo de nuevo.")
}

type resultadoDeHerramienta struct {
	texto    string
	conocida bool
}

// ejecutarEnParalelo lanza a la vez las herramientas que el modelo pidió en
// el mismo turno, que son independientes entre sí. El orden de los
// resultados se conserva: el modelo los empareja por posición e identificador.
func (s *Service) ejecutarEnParalelo(ctx context.Context, peticiones []Bloque, cx Contexto) []resultadoDeHerramienta {
	salida := make([]resultadoDeHerramienta, len(peticiones))

	var espera sync.WaitGroup
	espera.Add(len(peticiones))

	for i, p := range peticiones {
		go func(i int, p Bloque) {
			defer espera.Done()

			// Una herramienta que entre en pánico no puede tumbar el
			// servidor entero: se convierte en texto para el modelo.
			defer func() {
				if r := recover(); r != nil {
					salida[i] = resultadoDeHerramienta{
						texto: fmt.Sprintf("Error interno ejecutando %s.", p.Nombre), conocida: true,
					}
				}
			}()

			h := s.buscar(p.Nombre)
			if h == nil {
				salida[i] = resultadoDeHerramienta{texto: "Herramienta desconocida: " + p.Nombre}
				return
			}

			salida[i] = resultadoDeHerramienta{
				texto: h.Ejecutar(ctx, p.Entrada, cx), conocida: true,
			}
		}(i, p)
	}

	espera.Wait()
	return salida
}

func (s *Service) buscar(nombre string) *Herramienta {
	for i := range s.herramientas {
		if s.herramientas[i].Nombre == nombre {
			return &s.herramientas[i]
		}
	}
	return nil
}

func peticionesDe(bloques []Bloque) []Bloque {
	var salida []Bloque
	for _, b := range bloques {
		if b.Tipo == BloqueHerramienta {
			salida = append(salida, b)
		}
	}
	return salida
}

func textoDe(bloques []Bloque) string {
	var trozos []string
	for _, b := range bloques {
		if b.Tipo == BloqueTexto {
			trozos = append(trozos, b.Texto)
		}
	}
	return strings.TrimSpace(strings.Join(trozos, "\n"))
}
