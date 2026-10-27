package coolify

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

const tiempoLimite = 30 * time.Second

// RESTCliente habla con la API v1 de Coolify, autenticándose con Bearer.
type RESTCliente struct {
	client *http.Client
}

func NewRESTCliente() *RESTCliente {
	return &RESTCliente{client: &http.Client{Timeout: tiempoLimite}}
}

// ── Lecturas ───────────────────────────────────────────────────────────────

func (r *RESTCliente) Ping(ctx context.Context, c Credenciales) kernel.Result[string] {
	cuerpo, res := r.pedir(ctx, c, http.MethodGet, "/version", nil)
	if !res.OK {
		return kernel.Err[string](res.Error)
	}

	// /version devuelve la versión como texto entrecomillado, no como JSON.
	version := strings.Trim(strings.TrimSpace(string(cuerpo)), `"`)
	if version == "" {
		version = "ok"
	}
	return kernel.Ok(version)
}

// Resumen trae todo el panel de una vez.
//
// Las aplicaciones son lo imprescindible; el resto se pide en paralelo y se
// deja vacío si falla, porque una instancia puede no exponer algún tipo de
// recurso y eso no debe tumbar la pantalla entera.
func (r *RESTCliente) Resumen(ctx context.Context, c Credenciales) kernel.Result[Resumen] {
	var apps []appCruda
	if res := r.leerJSON(ctx, c, "/applications", &apps); !res.OK {
		return kernel.Err[Resumen](res.Error)
	}

	var (
		proyectos  []proyectoCrudo
		servidores []servidorCrudo
		bases      []recursoCrudo
		servicios  []recursoCrudo
		espera     sync.WaitGroup
	)

	paralelo := []struct {
		ruta    string
		destino any
	}{
		{"/projects", &proyectos},
		{"/servers", &servidores},
		{"/databases", &bases},
		{"/services", &servicios},
	}

	espera.Add(len(paralelo))
	for _, p := range paralelo {
		go func(ruta string, destino any) {
			defer espera.Done()
			_ = r.leerJSON(ctx, c, ruta, destino)
		}(p.ruta, p.destino)
	}
	espera.Wait()

	resumen := Resumen{
		Apps:       make([]App, 0, len(apps)),
		Proyectos:  make([]Proyecto, 0, len(proyectos)),
		Servidores: make([]Servidor, 0, len(servidores)),
		Bases:      make([]Recurso, 0, len(bases)),
		Servicios:  make([]Recurso, 0, len(servicios)),
	}

	for _, a := range apps {
		resumen.Apps = append(resumen.Apps, a.aDominio())
	}
	for _, p := range proyectos {
		resumen.Proyectos = append(resumen.Proyectos, Proyecto{
			UUID: p.UUID, Name: p.Name, Description: p.Description,
		})
	}
	for _, s := range servidores {
		resumen.Servidores = append(resumen.Servidores, Servidor{
			UUID: s.UUID, Name: s.Name, IP: s.IP, Reachable: s.Reachable,
		})
	}
	for _, b := range bases {
		resumen.Bases = append(resumen.Bases, b.aDominio("database"))
	}
	for _, s := range servicios {
		resumen.Servicios = append(resumen.Servicios, s.aDominio("service"))
	}

	return kernel.Ok(resumen)
}

func (r *RESTCliente) Entornos(ctx context.Context, c Credenciales, proyectoUUID string) kernel.Result[[]Entorno] {
	var crudos []struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	}
	if res := r.leerJSON(ctx, c, "/projects/"+url.PathEscape(proyectoUUID)+"/environments", &crudos); !res.OK {
		return kernel.Err[[]Entorno](res.Error)
	}

	salida := make([]Entorno, 0, len(crudos))
	for _, e := range crudos {
		nombre := e.Name
		if nombre == "" {
			nombre = "production"
		}
		salida = append(salida, Entorno{UUID: e.UUID, Name: nombre})
	}
	return kernel.Ok(salida)
}

func (r *RESTCliente) App(ctx context.Context, c Credenciales, uuid string) kernel.Result[App] {
	var cruda appCruda
	if res := r.leerJSON(ctx, c, "/applications/"+url.PathEscape(uuid), &cruda); !res.OK {
		return kernel.Err[App](res.Error)
	}
	return kernel.Ok(cruda.aDominio())
}

func (r *RESTCliente) Variables(ctx context.Context, c Credenciales, uuid string) kernel.Result[[]Variable] {
	var crudas []variableCruda
	if res := r.leerJSON(ctx, c, "/applications/"+url.PathEscape(uuid)+"/envs", &crudas); !res.OK {
		return kernel.Err[[]Variable](res.Error)
	}

	salida := make([]Variable, 0, len(crudas))
	for _, v := range crudas {
		salida = append(salida, Variable{
			UUID: v.UUID, Key: v.Key, Value: v.Value,
			IsBuildTime: v.IsBuildTime, IsPreview: v.IsPreview, IsLiteral: v.IsLiteral,
		})
	}
	return kernel.Ok(salida)
}

func (r *RESTCliente) Despliegues(ctx context.Context, c Credenciales) kernel.Result[[]Despliegue] {
	cuerpo, res := r.pedir(ctx, c, http.MethodGet, "/deployments", nil)
	if !res.OK {
		return kernel.Err[[]Despliegue](res.Error)
	}

	// Según la versión llega la lista suelta o envuelta en {"deployments": …}.
	var lista []despliegueCrudo
	if err := json.Unmarshal(cuerpo, &lista); err != nil {
		var envuelto struct {
			Deployments []despliegueCrudo `json:"deployments"`
		}
		if err := json.Unmarshal(cuerpo, &envuelto); err != nil {
			return kernel.Err[[]Despliegue]("Coolify devolvió una respuesta que no se entiende.")
		}
		lista = envuelto.Deployments
	}

	salida := make([]Despliegue, 0, len(lista))
	for _, d := range lista {
		salida = append(salida, d.aDominio())
	}
	return kernel.Ok(salida)
}

func (r *RESTCliente) Registros(ctx context.Context, c Credenciales, uuid string, lineas int) kernel.Result[string] {
	return r.registrosDe(ctx, c, "/applications/"+url.PathEscape(uuid), lineas)
}

func (r *RESTCliente) RegistrosRecurso(ctx context.Context, c Credenciales, tipo TipoRecurso, uuid string, lineas int) kernel.Result[string] {
	return r.registrosDe(ctx, c, "/"+string(tipo)+"/"+url.PathEscape(uuid), lineas)
}

// registrosDe lee la respuesta, que unas veces es texto suelto y otras un
// objeto {"logs": …}.
func (r *RESTCliente) registrosDe(ctx context.Context, c Credenciales, base string, lineas int) kernel.Result[string] {
	cuerpo, res := r.pedir(ctx, c, http.MethodGet,
		base+"/logs?lines="+strconv.Itoa(lineas), nil)
	if !res.OK {
		return kernel.Err[string](res.Error)
	}

	var envuelto struct {
		Logs string `json:"logs"`
	}
	if err := json.Unmarshal(cuerpo, &envuelto); err == nil && envuelto.Logs != "" {
		return kernel.Ok(envuelto.Logs)
	}

	var texto string
	if err := json.Unmarshal(cuerpo, &texto); err == nil {
		return kernel.Ok(texto)
	}

	return kernel.Ok(string(cuerpo))
}

// ── Acciones ───────────────────────────────────────────────────────────────

func (r *RESTCliente) Desplegar(ctx context.Context, c Credenciales, uuid string, forzar bool) kernel.Result[string] {
	ruta := "/deploy?uuid=" + url.QueryEscape(uuid) + "&force=" + strconv.FormatBool(forzar)
	return r.accion(ctx, c, http.MethodPost, ruta, nil, "Despliegue encolado.")
}

func (r *RESTCliente) Controlar(ctx context.Context, c Credenciales, uuid string, a Accion) kernel.Result[string] {
	ruta := "/applications/" + url.PathEscape(uuid) + "/" + string(a)
	return r.accion(ctx, c, http.MethodPost, ruta, nil, "Aplicación "+participio(a)+".")
}

func (r *RESTCliente) ControlarRecurso(ctx context.Context, c Credenciales, tipo TipoRecurso, uuid string, a Accion) kernel.Result[string] {
	ruta := "/" + string(tipo) + "/" + url.PathEscape(uuid) + "/" + string(a)
	return r.accion(ctx, c, http.MethodPost, ruta, nil, "Recurso "+participio(a)+".")
}

func (r *RESTCliente) CancelarDespliegue(ctx context.Context, c Credenciales, uuid string) kernel.Result[string] {
	ruta := "/deployments/" + url.PathEscape(uuid) + "/cancel"
	return r.accion(ctx, c, http.MethodPost, ruta, nil, "Despliegue cancelado.")
}

func participio(a Accion) string {
	switch a {
	case Arrancar:
		return "iniciada"
	case Parar:
		return "detenida"
	default:
		return "reiniciada"
	}
}

// ── Creación y edición ─────────────────────────────────────────────────────

func (r *RESTCliente) CrearApp(ctx context.Context, c Credenciales, in NuevaApp) kernel.Result[string] {
	cuerpo := map[string]any{
		"project_uuid":     in.ProjectUUID,
		"server_uuid":      in.ServerUUID,
		"environment_name": in.EnvironmentName,
		"environment_uuid": in.EnvironmentUUID,
		"instant_deploy":   in.InstantDeploy,
	}

	if n := strings.TrimSpace(in.Name); n != "" {
		cuerpo["name"] = n
	}
	if d := strings.TrimSpace(in.Domains); d != "" {
		cuerpo["domains"] = d
	}
	if b := strings.Trim(strings.TrimSpace(in.BaseDirectory), "/"); b != "" {
		cuerpo["base_directory"] = "/" + b
	}

	// ports_exposes es obligatorio en los dos extremos. Omitirlo cuando está
	// vacío es justo lo que produce un "Validation failed." pelado, así que
	// se manda siempre con el mismo valor por defecto que usa el formulario.
	puertos := strings.TrimSpace(in.PortsExposes)
	if puertos == "" {
		puertos = "3000"
	}
	cuerpo["ports_exposes"] = puertos

	ruta := "/applications/public"
	if in.Source == "dockerfile" {
		ruta = "/applications/dockerfile"
		cuerpo["dockerfile"] = in.Dockerfile
	} else {
		cuerpo["git_repository"] = strings.TrimSpace(in.GitRepository)
		cuerpo["git_branch"] = strings.TrimSpace(in.GitBranch)
		cuerpo["build_pack"] = in.BuildPack
	}

	return r.accion(ctx, c, http.MethodPost, ruta, cuerpo, "Aplicación creada.")
}

func (r *RESTCliente) ActualizarApp(ctx context.Context, c Credenciales, uuid string, p ParcheApp) kernel.Result[string] {
	cuerpo := map[string]any{
		"name":            strings.TrimSpace(p.Name),
		"description":     p.Description,
		"domains":         strings.TrimSpace(p.Domains),
		"git_branch":      strings.TrimSpace(p.GitBranch),
		"build_pack":      p.BuildPack,
		"ports_exposes":   strings.TrimSpace(p.PortsExposes),
		"install_command": p.InstallCommand,
		"build_command":   p.BuildCommand,
		"start_command":   p.StartCommand,
		"base_directory":  p.BaseDirectory,
	}

	ruta := "/applications/" + url.PathEscape(uuid)
	return r.accion(ctx, c, http.MethodPatch, ruta, cuerpo, "Aplicación actualizada.")
}

func (r *RESTCliente) CrearBase(ctx context.Context, c Credenciales, in NuevaBase) kernel.Result[string] {
	cuerpo := map[string]any{
		"server_uuid":      in.ServerUUID,
		"project_uuid":     in.ProjectUUID,
		"environment_name": in.EnvironmentName,
		"environment_uuid": in.EnvironmentUUID,
		"instant_deploy":   in.InstantDeploy,
	}
	if n := strings.TrimSpace(in.Name); n != "" {
		cuerpo["name"] = n
	}
	if i := strings.TrimSpace(in.Image); i != "" {
		cuerpo["image"] = i
	}

	ruta := "/databases/" + url.PathEscape(in.Type)
	return r.accion(ctx, c, http.MethodPost, ruta, cuerpo, "Base de datos creada.")
}

func (r *RESTCliente) CrearServicio(ctx context.Context, c Credenciales, in NuevoServicio) kernel.Result[string] {
	cuerpo := map[string]any{
		"project_uuid":       in.ProjectUUID,
		"server_uuid":        in.ServerUUID,
		"environment_name":   in.EnvironmentName,
		"environment_uuid":   in.EnvironmentUUID,
		"docker_compose_raw": in.DockerCompose,
		"instant_deploy":     in.InstantDeploy,
	}
	if n := strings.TrimSpace(in.Name); n != "" {
		cuerpo["name"] = n
	}

	return r.accion(ctx, c, http.MethodPost, "/services", cuerpo, "Servicio creado.")
}

func (r *RESTCliente) CrearProyecto(ctx context.Context, c Credenciales, nombre, descripcion string) kernel.Result[string] {
	cuerpo := map[string]any{"name": nombre, "description": descripcion}
	return r.accion(ctx, c, http.MethodPost, "/projects", cuerpo, "Proyecto creado.")
}

func (r *RESTCliente) ActualizarProyecto(ctx context.Context, c Credenciales, uuid, nombre, descripcion string) kernel.Result[string] {
	cuerpo := map[string]any{"name": nombre, "description": descripcion}
	ruta := "/projects/" + url.PathEscape(uuid)
	return r.accion(ctx, c, http.MethodPatch, ruta, cuerpo, "Proyecto actualizado.")
}

func (r *RESTCliente) BorrarRecurso(ctx context.Context, c Credenciales, tipo TipoBorrable, uuid string) kernel.Result[string] {
	ruta := "/" + string(tipo) + "/" + url.PathEscape(uuid)
	return r.accion(ctx, c, http.MethodDelete, ruta, nil, "Eliminado.")
}

// ── Variables de entorno ───────────────────────────────────────────────────

func cuerpoDeVariable(in NuevaVariable) map[string]any {
	return map[string]any{
		"key":        in.Key,
		"value":      in.Value,
		"is_preview": in.IsPreview,
		"is_literal": in.IsLiteral,
	}
}

func (r *RESTCliente) CrearVariable(ctx context.Context, c Credenciales, uuid string, in NuevaVariable) kernel.Result[string] {
	ruta := "/applications/" + url.PathEscape(uuid) + "/envs"
	return r.accion(ctx, c, http.MethodPost, ruta, cuerpoDeVariable(in), "Variable creada.")
}

func (r *RESTCliente) ActualizarVariable(ctx context.Context, c Credenciales, uuid string, in NuevaVariable) kernel.Result[string] {
	ruta := "/applications/" + url.PathEscape(uuid) + "/envs"
	return r.accion(ctx, c, http.MethodPatch, ruta, cuerpoDeVariable(in), "Variable actualizada.")
}

func (r *RESTCliente) BorrarVariable(ctx context.Context, c Credenciales, uuid, varUUID string) kernel.Result[string] {
	ruta := "/applications/" + url.PathEscape(uuid) + "/envs/" + url.PathEscape(varUUID)
	return r.accion(ctx, c, http.MethodDelete, ruta, nil, "Variable eliminada.")
}

// ── Fontanería ─────────────────────────────────────────────────────────────

// accion ejecuta una llamada que cambia algo y devuelve el mensaje de
// Coolify, o uno por defecto si no manda ninguno.
func (r *RESTCliente) accion(ctx context.Context, c Credenciales, metodo, ruta string, cuerpo map[string]any, porDefecto string) kernel.Result[string] {
	datos, res := r.pedir(ctx, c, metodo, ruta, cuerpo)
	if !res.OK {
		return kernel.Err[string](res.Error)
	}

	var respuesta struct {
		Message string `json:"message"`
		UUID    string `json:"uuid"`
	}
	if err := json.Unmarshal(datos, &respuesta); err == nil {
		if respuesta.Message != "" {
			return kernel.Ok(respuesta.Message)
		}
		if respuesta.UUID != "" {
			return kernel.Ok(strings.TrimSuffix(porDefecto, ".") + " (" + respuesta.UUID + ").")
		}
	}

	return kernel.Ok(porDefecto)
}

func (r *RESTCliente) leerJSON(ctx context.Context, c Credenciales, ruta string, destino any) kernel.Result[struct{}] {
	cuerpo, res := r.pedir(ctx, c, http.MethodGet, ruta, nil)
	if !res.OK {
		return res
	}
	if err := json.Unmarshal(cuerpo, destino); err != nil {
		return kernel.Err[struct{}]("Coolify devolvió una respuesta que no se entiende.")
	}
	return kernel.Ok(struct{}{})
}

func (r *RESTCliente) pedir(ctx context.Context, c Credenciales, metodo, ruta string, cuerpo map[string]any) ([]byte, kernel.Result[struct{}]) {
	var lector io.Reader
	if cuerpo != nil {
		datos, err := json.Marshal(cuerpo)
		if err != nil {
			return nil, kernel.Err[struct{}]("No se pudo preparar la petición.")
		}
		lector = bytes.NewReader(datos)
	}

	req, err := http.NewRequestWithContext(ctx, metodo, c.BaseURL+"/api/v1"+ruta, lector)
	if err != nil {
		return nil, kernel.Err[struct{}]("No se pudo preparar la petición a Coolify.")
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	if cuerpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := r.client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "deadline exceeded") || strings.Contains(err.Error(), "Client.Timeout") {
			return nil, kernel.Err[struct{}]("Coolify tardó más de 30 segundos en responder.")
		}
		return nil, kernel.Err[struct{}]("No se pudo conectar con Coolify.")
	}
	defer res.Body.Close()

	datos, _ := io.ReadAll(io.LimitReader(res.Body, 32<<20))

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, kernel.Err[struct{}](ExplicarError(res.StatusCode, datos))
	}

	return datos, kernel.Ok(struct{}{})
}

// ── Las formas que manda Coolify ───────────────────────────────────────────
//
// Todos los campos son opcionales a propósito: Coolify ha cambiado esta
// respuesta entre versiones y un campo que falta no debe tumbar la pantalla.

type appCruda struct {
	UUID           string  `json:"uuid"`
	Name           string  `json:"name"`
	Description    *string `json:"description"`
	FQDN           *string `json:"fqdn"`
	Status         string  `json:"status"`
	GitRepository  *string `json:"git_repository"`
	GitBranch      *string `json:"git_branch"`
	BuildPack      *string `json:"build_pack"`
	PortsExposes   string  `json:"ports_exposes"`
	InstallCommand string  `json:"install_command"`
	BuildCommand   string  `json:"build_command"`
	StartCommand   string  `json:"start_command"`
	BaseDirectory  string  `json:"base_directory"`
	Environment    *struct {
		Name    string `json:"name"`
		Project *struct {
			Name string `json:"name"`
		} `json:"project"`
	} `json:"environment"`
}

func (a appCruda) aDominio() App {
	estado, sano := InterpretarEstado(a.Status)

	app := App{
		UUID: a.UUID, Name: a.Name, Description: a.Description, FQDN: a.FQDN,
		Status: a.Status, State: estado, Healthy: sano,
		GitRepository: a.GitRepository, GitBranch: a.GitBranch,
		BuildPack: a.BuildPack, PortsExposes: a.PortsExposes,
		InstallCommand: a.InstallCommand, BuildCommand: a.BuildCommand,
		StartCommand: a.StartCommand, BaseDirectory: a.BaseDirectory,
	}

	if a.Environment != nil {
		if a.Environment.Name != "" {
			nombre := a.Environment.Name
			app.EnvironmentName = &nombre
		}
		if a.Environment.Project != nil && a.Environment.Project.Name != "" {
			proyecto := a.Environment.Project.Name
			app.ProjectName = &proyecto
		}
	}

	return app
}

type proyectoCrudo struct {
	UUID        string  `json:"uuid"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type servidorCrudo struct {
	UUID      string  `json:"uuid"`
	Name      string  `json:"name"`
	IP        *string `json:"ip"`
	Reachable *bool   `json:"is_reachable"`
}

type recursoCrudo struct {
	UUID   string `json:"uuid"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (r recursoCrudo) aDominio(tipo string) Recurso {
	estado, _ := InterpretarEstado(r.Status)
	return Recurso{UUID: r.UUID, Name: r.Name, Type: tipo, Status: r.Status, State: estado}
}

type variableCruda struct {
	UUID        string `json:"uuid"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	IsBuildTime bool   `json:"is_build_time"`
	IsPreview   bool   `json:"is_preview"`
	IsLiteral   bool   `json:"is_literal"`
}

type despliegueCrudo struct {
	UUID            string  `json:"deployment_uuid"`
	ApplicationName string  `json:"application_name"`
	ServerName      *string `json:"server_name"`
	Status          string  `json:"status"`
	Commit          *string `json:"commit"`
	CommitMessage   *string `json:"commit_message"`
	CreatedAt       *string `json:"created_at"`
}

func (d despliegueCrudo) aDominio() Despliegue {
	return Despliegue{
		UUID: d.UUID, ApplicationName: d.ApplicationName, ServerName: d.ServerName,
		Status: InterpretarDespliegue(d.Status), RawStatus: d.Status,
		Commit: d.Commit, CommitMessage: d.CommitMessage, CreatedAt: d.CreatedAt,
	}
}
