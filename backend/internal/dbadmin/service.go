package dbadmin

import (
	"context"
	"strings"
	"sync"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Repo guarda las conexiones.
type Repo interface {
	List(ctx context.Context) ([]Conexion, error)
	Get(ctx context.Context, id string) (*Conexion, error)
	Create(ctx context.Context, c Conexion) (*Conexion, error)
	Update(ctx context.Context, c Conexion) (*Conexion, error)
	Remove(ctx context.Context, id string) error
}

// Ejecutor lanza consultas contra una base externa.
type Ejecutor interface {
	Probar(ctx context.Context, url string) kernel.Result[string]
	Consultar(ctx context.Context, url, sql string, soloLectura bool) kernel.Result[Resultado]
	ConsultarConParametros(ctx context.Context, url, sql string, params []any, soloLectura bool) kernel.Result[Resultado]
}

// Introspector lee la estructura de la base.
type Introspector interface {
	Esquemas(ctx context.Context, url string) kernel.Result[[]Esquema]
	Tablas(ctx context.Context, url, esquema string) kernel.Result[[]Tabla]
	Columnas(ctx context.Context, url, esquema, tabla string) kernel.Result[[]Columna]
	Indices(ctx context.Context, url, esquema, tabla string) kernel.Result[[]Indice]
	ClavesAjenas(ctx context.Context, url, esquema string) kernel.Result[[]ClaveAjena]
}

type Service struct {
	repo          Repo
	ejecutor      Ejecutor
	introspeccion Introspector
}

func NewService(repo Repo, ejecutor Ejecutor, introspeccion Introspector) *Service {
	return &Service{repo: repo, ejecutor: ejecutor, introspeccion: introspeccion}
}

// ── Conexiones ─────────────────────────────────────────────────────────────

func (s *Service) Conexiones(ctx context.Context) ([]Conexion, error) {
	return s.repo.List(ctx)
}

func (s *Service) Guardar(ctx context.Context, c Conexion) (kernel.Result[Conexion], error) {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return kernel.Err[Conexion]("La conexión necesita un nombre."), nil
	}

	url := ValidarURL(c.ConnectionURL)
	if !url.OK {
		return kernel.Err[Conexion](url.Error), nil
	}
	c.ConnectionURL = url.Value

	var guardada *Conexion
	var err error
	if c.ID == "" {
		guardada, err = s.repo.Create(ctx, c)
	} else {
		guardada, err = s.repo.Update(ctx, c)
	}
	if err != nil {
		return kernel.Result[Conexion]{}, err
	}
	if guardada == nil {
		return kernel.Err[Conexion]("Esa conexión ya no existe."), nil
	}

	return kernel.Ok(*guardada), nil
}

func (s *Service) Borrar(ctx context.Context, id string) error {
	return s.repo.Remove(ctx, id)
}

func (s *Service) Probar(ctx context.Context, id string) (kernel.Result[string], error) {
	conexion, err := s.repo.Get(ctx, id)
	if err != nil {
		return kernel.Result[string]{}, err
	}
	if conexion == nil {
		return kernel.Err[string]("Esa conexión ya no existe."), nil
	}
	return s.ejecutor.Probar(ctx, conexion.ConnectionURL), nil
}

// conURL resuelve la conexión y entrega su URL.
func conURL[T any](ctx context.Context, s *Service, id string, f func(string) kernel.Result[T]) (kernel.Result[T], error) {
	conexion, err := s.repo.Get(ctx, id)
	if err != nil {
		return kernel.Result[T]{}, err
	}
	if conexion == nil {
		return kernel.Err[T]("Esa conexión ya no existe."), nil
	}
	return f(conexion.ConnectionURL), nil
}

// ── Estructura ─────────────────────────────────────────────────────────────

func (s *Service) Esquemas(ctx context.Context, id string) (kernel.Result[[]Esquema], error) {
	return conURL(ctx, s, id, func(url string) kernel.Result[[]Esquema] {
		return s.introspeccion.Esquemas(ctx, url)
	})
}

func (s *Service) Tablas(ctx context.Context, id, esquema string) (kernel.Result[[]Tabla], error) {
	if v := ValidarIdentificador(esquema, "esquema"); !v.OK {
		return kernel.Err[[]Tabla](v.Error), nil
	}
	return conURL(ctx, s, id, func(url string) kernel.Result[[]Tabla] {
		return s.introspeccion.Tablas(ctx, url, esquema)
	})
}

// DetalleTabla junta columnas, índices y las claves ajenas en los dos
// sentidos. Las tres consultas van en paralelo: son independientes.
func (s *Service) DetalleTabla(ctx context.Context, id, esquema, tabla string) (kernel.Result[DetalleTabla], error) {
	if v := s.validarDestino(esquema, tabla); !v.OK {
		return kernel.Err[DetalleTabla](v.Error), nil
	}

	return conURL(ctx, s, id, func(url string) kernel.Result[DetalleTabla] {
		var (
			columnas kernel.Result[[]Columna]
			indices  kernel.Result[[]Indice]
			claves   kernel.Result[[]ClaveAjena]
			espera   sync.WaitGroup
		)

		espera.Add(3)
		go func() { defer espera.Done(); columnas = s.introspeccion.Columnas(ctx, url, esquema, tabla) }()
		go func() { defer espera.Done(); indices = s.introspeccion.Indices(ctx, url, esquema, tabla) }()
		go func() { defer espera.Done(); claves = s.introspeccion.ClavesAjenas(ctx, url, esquema) }()
		espera.Wait()

		if !columnas.OK {
			return kernel.Err[DetalleTabla](columnas.Error)
		}
		if !indices.OK {
			return kernel.Err[DetalleTabla](indices.Error)
		}
		if !claves.OK {
			return kernel.Err[DetalleTabla](claves.Error)
		}

		salientes := make([]ClaveAjena, 0)
		entrantes := make([]ClaveAjena, 0)
		for _, k := range claves.Value {
			if k.Table == tabla {
				salientes = append(salientes, k)
			}
			if k.RefTable == tabla && k.RefSchema == esquema {
				entrantes = append(entrantes, k)
			}
		}

		// A cada columna se le anota a dónde apunta, para la etiqueta de
		// clave ajena en la interfaz.
		destino := map[string]Referencia{}
		for _, k := range salientes {
			for i, col := range k.Columns {
				ref := Referencia{Schema: k.RefSchema, Table: k.RefTable}
				if i < len(k.RefColumns) {
					ref.Column = k.RefColumns[i]
				}
				destino[col] = ref
			}
		}

		columnasAnotadas := make([]Columna, 0, len(columnas.Value))
		for _, c := range columnas.Value {
			if ref, hay := destino[c.Name]; hay {
				copia := ref
				c.References = &copia
			}
			columnasAnotadas = append(columnasAnotadas, c)
		}

		return kernel.Ok(DetalleTabla{
			Columns: columnasAnotadas, Indexes: indices.Value,
			ForeignKeys: salientes, ReferencedBy: entrantes,
		})
	})
}

// Diagrama trae todo lo que necesita el dibujo de relaciones.
func (s *Service) Diagrama(ctx context.Context, id, esquema string) (kernel.Result[Diagrama], error) {
	if v := ValidarIdentificador(esquema, "esquema"); !v.OK {
		return kernel.Err[Diagrama](v.Error), nil
	}

	return conURL(ctx, s, id, func(url string) kernel.Result[Diagrama] {
		tablas := s.introspeccion.Tablas(ctx, url, esquema)
		if !tablas.OK {
			return kernel.Err[Diagrama](tablas.Error)
		}

		claves := s.introspeccion.ClavesAjenas(ctx, url, esquema)
		if !claves.OK {
			return kernel.Err[Diagrama](claves.Error)
		}

		esAjena := map[string]map[string]bool{}
		for _, k := range claves.Value {
			if esAjena[k.Table] == nil {
				esAjena[k.Table] = map[string]bool{}
			}
			for _, c := range k.Columns {
				esAjena[k.Table][c] = true
			}
		}

		// Las columnas de cada tabla se piden en paralelo: con treinta
		// tablas, en serie sería una espera larga para dibujar un diagrama.
		salida := make([]TablaDelDiagrama, len(tablas.Value))
		var espera sync.WaitGroup
		espera.Add(len(tablas.Value))

		for i, t := range tablas.Value {
			go func(i int, t Tabla) {
				defer espera.Done()

				fila := TablaDelDiagrama{Schema: esquema, Name: t.Name, Columns: []ColumnaDelDiagrama{}}

				if cols := s.introspeccion.Columnas(ctx, url, esquema, t.Name); cols.OK {
					for _, c := range cols.Value {
						fila.Columns = append(fila.Columns, ColumnaDelDiagrama{
							Name: c.Name, DataType: c.DataType,
							IsPrimaryKey: c.IsPrimaryKey,
							IsForeignKey: esAjena[t.Name][c.Name],
						})
					}
				}

				salida[i] = fila
			}(i, t)
		}
		espera.Wait()

		return kernel.Ok(Diagrama{Tables: salida, Relations: claves.Value})
	})
}

// ── Filas ──────────────────────────────────────────────────────────────────

// Hojear devuelve una página de la tabla.
func (s *Service) Hojear(ctx context.Context, id string, o OpcionesHojear) (kernel.Result[Pagina], error) {
	if v := s.validarDestino(o.Esquema, o.Tabla); !v.OK {
		return kernel.Err[Pagina](v.Error), nil
	}
	if o.OrdenarPor != "" {
		if v := ValidarIdentificador(o.OrdenarPor, "nombre de columna"); !v.OK {
			return kernel.Err[Pagina](v.Error), nil
		}
	}

	return conURL(ctx, s, id, func(url string) kernel.Result[Pagina] {
		limite := AcotarLimite(o.Limite)
		o.Limite = limite

		sentencia := ConstruirHojear(o)

		// Siempre en solo lectura: hojear no puede modificar nada, pase lo
		// que pase con la configuración de la conexión.
		res := s.ejecutor.ConsultarConParametros(ctx, url, sentencia.Texto, sentencia.Parametros, true)
		if !res.OK {
			return kernel.Err[Pagina](res.Error)
		}

		// Se pidió una fila de más: si llegó, hay página siguiente.
		hayMas := len(res.Value.Rows) > limite
		if hayMas {
			res.Value.Rows = res.Value.Rows[:limite]
			res.Value.RowCount = int64(limite)
		}

		desde := o.Desde
		if desde < 0 {
			desde = 0
		}

		return kernel.Ok(Pagina{
			Resultado: res.Value, HasMore: hayMas, Limit: limite, Offset: desde,
		})
	})
}

func (s *Service) Insertar(ctx context.Context, id, esquema, tabla string, valores map[string]any) (kernel.Result[Resultado], error) {
	return s.modificarFila(ctx, id, esquema, tabla, func() kernel.Result[Sentencia] {
		return ConstruirInsertar(esquema, tabla, valores)
	})
}

func (s *Service) Actualizar(ctx context.Context, id, esquema, tabla string, valores, clave map[string]any) (kernel.Result[Resultado], error) {
	return s.modificarFila(ctx, id, esquema, tabla, func() kernel.Result[Sentencia] {
		return ConstruirActualizar(esquema, tabla, valores, clave)
	})
}

func (s *Service) BorrarFila(ctx context.Context, id, esquema, tabla string, clave map[string]any) (kernel.Result[Resultado], error) {
	return s.modificarFila(ctx, id, esquema, tabla, func() kernel.Result[Sentencia] {
		return ConstruirBorrar(esquema, tabla, clave)
	})
}

// modificarFila es el guardarraíl común de las tres escrituras.
func (s *Service) modificarFila(ctx context.Context, id, esquema, tabla string, construir func() kernel.Result[Sentencia]) (kernel.Result[Resultado], error) {
	if v := s.validarDestino(esquema, tabla); !v.OK {
		return kernel.Err[Resultado](v.Error), nil
	}

	conexion, err := s.repo.Get(ctx, id)
	if err != nil {
		return kernel.Result[Resultado]{}, err
	}
	if conexion == nil {
		return kernel.Err[Resultado]("Esa conexión ya no existe."), nil
	}
	if conexion.ReadOnly {
		return kernel.Err[Resultado](mensajeSoloLectura(conexion.Name)), nil
	}

	sentencia := construir()
	if !sentencia.OK {
		return kernel.Err[Resultado](sentencia.Error), nil
	}

	return s.ejecutor.ConsultarConParametros(ctx, conexion.ConnectionURL,
		sentencia.Value.Texto, sentencia.Value.Parametros, false), nil
}

func (s *Service) validarDestino(esquema, tabla string) kernel.Result[struct{}] {
	if v := ValidarIdentificador(esquema, "esquema"); !v.OK {
		return kernel.Err[struct{}](v.Error)
	}
	if v := ValidarIdentificador(tabla, "nombre de tabla"); !v.OK {
		return kernel.Err[struct{}](v.Error)
	}
	return kernel.Ok(struct{}{})
}

func mensajeSoloLectura(nombre string) string {
	return "La conexión «" + nombre + "» está en modo solo lectura. " +
		"Desactívalo si de verdad quieres modificar datos."
}

// ── Consulta libre ─────────────────────────────────────────────────────────

// Consultar ejecuta el SQL que escriba el usuario.
//
// Dos barreras, y en este orden: primero se pide confirmación para lo
// destructivo, y luego se comprueba el modo solo lectura. Así, en una
// conexión de solo lectura, un DROP dice que hay que confirmar antes de
// decir que no se puede: quien lo escribió se entera de las dos cosas.
func (s *Service) Consultar(ctx context.Context, id, sql string, confirmado bool) (kernel.Result[Resultado], error) {
	if strings.TrimSpace(sql) == "" {
		return kernel.Err[Resultado]("Escribe una consulta."), nil
	}

	conexion, err := s.repo.Get(ctx, id)
	if err != nil {
		return kernel.Result[Resultado]{}, err
	}
	if conexion == nil {
		return kernel.Err[Resultado]("Esa conexión ya no existe."), nil
	}

	analisis := Analizar(sql)

	if analisis.Riesgo == Destructiva && !confirmado {
		que := "La sentencia " + analisis.Palabra
		if analisis.SinFiltro {
			que = analisis.Palabra + " sin WHERE afecta a TODAS las filas"
		}
		return kernel.Err[Resultado]("⚠️ " + que + ". Confírmalo para ejecutarla."), nil
	}

	if conexion.ReadOnly && analisis.Riesgo != Inofensiva {
		return kernel.Err[Resultado](mensajeSoloLectura(conexion.Name)), nil
	}

	return s.ejecutor.Consultar(ctx, conexion.ConnectionURL, sql, conexion.ReadOnly), nil
}
