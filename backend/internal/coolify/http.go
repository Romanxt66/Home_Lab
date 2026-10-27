package coolify

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

type configDTO struct {
	ID       string `json:"id"`
	BaseURL  string `json:"baseUrl"`
	HasToken bool   `json:"hasToken"`
}

type appDTO struct {
	UUID            string  `json:"uuid"`
	Name            string  `json:"name"`
	Description     *string `json:"description"`
	FQDN            *string `json:"fqdn"`
	Status          string  `json:"status"`
	State           string  `json:"state"`
	StateLabel      string  `json:"stateLabel"`
	Healthy         *bool   `json:"healthy"`
	GitRepository   *string `json:"gitRepository"`
	GitBranch       *string `json:"gitBranch"`
	BuildPack       *string `json:"buildPack"`
	ProjectName     *string `json:"projectName"`
	EnvironmentName *string `json:"environmentName"`
	PortsExposes    string  `json:"portsExposes"`
	InstallCommand  string  `json:"installCommand"`
	BuildCommand    string  `json:"buildCommand"`
	StartCommand    string  `json:"startCommand"`
	BaseDirectory   string  `json:"baseDirectory"`
}

type recursoDTO struct {
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	State      string `json:"state"`
	StateLabel string `json:"stateLabel"`
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/config", h.config)
	g.POST("/config", h.conectar)
	g.DELETE("/config", h.desconectar)

	// El catálogo lo publica el servidor para que la interfaz no lo repita
	// por su cuenta y se desincronice.
	g.GET("/catalog", h.catalogo)

	g.GET("/overview", h.resumen)
	g.GET("/projects/:uuid/environments", h.entornos)

	g.GET("/applications/:uuid", h.app)
	g.PATCH("/applications/:uuid", h.actualizarApp)
	g.POST("/applications", h.crearApp)
	g.POST("/applications/:uuid/deploy", h.desplegar)
	g.POST("/applications/:uuid/:action", h.controlar)
	g.GET("/applications/:uuid/logs", h.registros)

	g.GET("/applications/:uuid/envs", h.variables)
	g.POST("/applications/:uuid/envs", h.crearVariable)
	g.PATCH("/applications/:uuid/envs", h.actualizarVariable)
	g.DELETE("/applications/:uuid/envs/:envUuid", h.borrarVariable)

	g.GET("/deployments", h.despliegues)
	g.POST("/deployments/:uuid/cancel", h.cancelarDespliegue)

	g.POST("/resources/:kind/:uuid/:action", h.controlarRecurso)
	g.GET("/resources/:kind/:uuid/logs", h.registrosRecurso)

	g.POST("/databases", h.crearBase)
	g.POST("/services", h.crearServicio)
	g.POST("/projects", h.crearProyecto)
	g.PATCH("/projects/:uuid", h.actualizarProyecto)
	g.DELETE("/resources/:kind/:uuid", h.borrarRecurso)
}

// ── Conexión ───────────────────────────────────────────────────────────────

func (h *Handler) config(c *gin.Context) {
	cfg, err := h.svc.Config(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if cfg == nil {
		c.JSON(http.StatusOK, nil)
		return
	}
	c.JSON(http.StatusOK, configDTO{ID: cfg.ID, BaseURL: cfg.BaseURL, HasToken: cfg.Token != ""})
}

func (h *Handler) conectar(c *gin.Context) {
	var cuerpo struct {
		BaseURL string `json:"baseUrl"`
		Token   string `json:"token"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.Conectar(c.Request.Context(), cuerpo.BaseURL, cuerpo.Token)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, configDTO{
		ID: res.Value.ID, BaseURL: res.Value.BaseURL, HasToken: res.Value.Token != "",
	})
}

func (h *Handler) desconectar(c *gin.Context) {
	if err := h.svc.Desconectar(c.Request.Context()); err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) catalogo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"databaseTypes": aOpciones(TiposDeBase),
		"buildPacks":    aOpciones(BuildPacks),
	})
}

func aOpciones(os []Opcion) []gin.H {
	salida := make([]gin.H, 0, len(os))
	for _, o := range os {
		salida = append(salida, gin.H{"id": o.ID, "label": o.Label})
	}
	return salida
}

// ── Lecturas ───────────────────────────────────────────────────────────────

func (h *Handler) resumen(c *gin.Context) {
	res, err := h.svc.Resumen(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	apps := make([]appDTO, 0, len(res.Value.Apps))
	for _, a := range res.Value.Apps {
		apps = append(apps, aAppDTO(a))
	}

	proyectos := make([]gin.H, 0, len(res.Value.Proyectos))
	for _, p := range res.Value.Proyectos {
		proyectos = append(proyectos, gin.H{"uuid": p.UUID, "name": p.Name, "description": p.Description})
	}

	servidores := make([]gin.H, 0, len(res.Value.Servidores))
	for _, s := range res.Value.Servidores {
		servidores = append(servidores, gin.H{
			"uuid": s.UUID, "name": s.Name, "ip": s.IP, "reachable": s.Reachable,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"apps":      apps,
		"projects":  proyectos,
		"servers":   servidores,
		"databases": aRecursosDTO(res.Value.Bases),
		"services":  aRecursosDTO(res.Value.Servicios),
	})
}

func (h *Handler) entornos(c *gin.Context) {
	res, err := h.svc.Entornos(c.Request.Context(), c.Param("uuid"))
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	salida := make([]gin.H, 0, len(res.Value))
	for _, e := range res.Value {
		salida = append(salida, gin.H{"uuid": e.UUID, "name": e.Name})
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) app(c *gin.Context) {
	res, err := h.svc.App(c.Request.Context(), c.Param("uuid"))
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, aAppDTO(res.Value))
}

func (h *Handler) variables(c *gin.Context) {
	res, err := h.svc.Variables(c.Request.Context(), c.Param("uuid"))
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	salida := make([]gin.H, 0, len(res.Value))
	for _, v := range res.Value {
		salida = append(salida, gin.H{
			"uuid": v.UUID, "key": v.Key, "value": v.Value,
			"isBuildTime": v.IsBuildTime, "isPreview": v.IsPreview, "isLiteral": v.IsLiteral,
		})
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) despliegues(c *gin.Context) {
	res, err := h.svc.Despliegues(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	salida := make([]gin.H, 0, len(res.Value))
	for _, d := range res.Value {
		salida = append(salida, gin.H{
			"uuid": d.UUID, "applicationName": d.ApplicationName,
			"serverName": d.ServerName, "status": string(d.Status),
			"statusLabel": EtiquetasDeDespliegue[d.Status],
			"rawStatus":   d.RawStatus, "commit": d.Commit,
			"commitMessage": d.CommitMessage, "createdAt": d.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) registros(c *gin.Context) {
	lineas, _ := strconv.Atoi(c.DefaultQuery("lines", "200"))

	res, err := h.svc.Registros(c.Request.Context(), c.Param("uuid"), lineas)
	h.responderTexto(c, res, err)
}

func (h *Handler) registrosRecurso(c *gin.Context) {
	lineas, _ := strconv.Atoi(c.DefaultQuery("lines", "200"))

	res, err := h.svc.RegistrosRecurso(c.Request.Context(),
		TipoRecurso(c.Param("kind")), c.Param("uuid"), lineas)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": res.Value})
}

func (h *Handler) responderTexto(c *gin.Context, res kernel.Result[string], err error) {
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": res.Value})
}

// ── Acciones ───────────────────────────────────────────────────────────────

func (h *Handler) desplegar(c *gin.Context) {
	res, err := h.svc.Desplegar(c.Request.Context(), c.Param("uuid"), c.Query("force") == "true")
	h.responderMensaje(c, res, err)
}

func (h *Handler) controlar(c *gin.Context) {
	res, err := h.svc.Controlar(c.Request.Context(), c.Param("uuid"), Accion(c.Param("action")))
	h.responderMensaje(c, res, err)
}

func (h *Handler) controlarRecurso(c *gin.Context) {
	res, err := h.svc.ControlarRecurso(c.Request.Context(),
		TipoRecurso(c.Param("kind")), c.Param("uuid"), Accion(c.Param("action")))
	h.responderMensaje(c, res, err)
}

func (h *Handler) cancelarDespliegue(c *gin.Context) {
	res, err := h.svc.CancelarDespliegue(c.Request.Context(), c.Param("uuid"))
	h.responderMensaje(c, res, err)
}

func (h *Handler) borrarRecurso(c *gin.Context) {
	res, err := h.svc.BorrarRecurso(c.Request.Context(),
		TipoBorrable(c.Param("kind")), c.Param("uuid"))
	h.responderMensaje(c, res, err)
}

// ── Creación y edición ─────────────────────────────────────────────────────

func (h *Handler) crearApp(c *gin.Context) {
	var cuerpo struct {
		Source          string `json:"source"`
		Name            string `json:"name"`
		ProjectUUID     string `json:"projectUuid"`
		EnvironmentName string `json:"environmentName"`
		EnvironmentUUID string `json:"environmentUuid"`
		ServerUUID      string `json:"serverUuid"`
		Domains         string `json:"domains"`
		PortsExposes    string `json:"portsExposes"`
		InstantDeploy   bool   `json:"instantDeploy"`
		BaseDirectory   string `json:"baseDirectory"`
		GitRepository   string `json:"gitRepository"`
		GitBranch       string `json:"gitBranch"`
		BuildPack       string `json:"buildPack"`
		Dockerfile      string `json:"dockerfile"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.CrearApp(c.Request.Context(), NuevaApp(cuerpo))
	h.responderMensaje(c, res, err)
}

func (h *Handler) actualizarApp(c *gin.Context) {
	var cuerpo struct {
		Name           string `json:"name"`
		Description    string `json:"description"`
		Domains        string `json:"domains"`
		GitBranch      string `json:"gitBranch"`
		BuildPack      string `json:"buildPack"`
		PortsExposes   string `json:"portsExposes"`
		InstallCommand string `json:"installCommand"`
		BuildCommand   string `json:"buildCommand"`
		StartCommand   string `json:"startCommand"`
		BaseDirectory  string `json:"baseDirectory"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.ActualizarApp(c.Request.Context(), c.Param("uuid"), ParcheApp(cuerpo))
	h.responderMensaje(c, res, err)
}

func (h *Handler) crearBase(c *gin.Context) {
	var cuerpo struct {
		Type            string `json:"type"`
		Name            string `json:"name"`
		ProjectUUID     string `json:"projectUuid"`
		EnvironmentName string `json:"environmentName"`
		EnvironmentUUID string `json:"environmentUuid"`
		ServerUUID      string `json:"serverUuid"`
		Image           string `json:"image"`
		InstantDeploy   bool   `json:"instantDeploy"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.CrearBase(c.Request.Context(), NuevaBase(cuerpo))
	h.responderMensaje(c, res, err)
}

func (h *Handler) crearServicio(c *gin.Context) {
	var cuerpo struct {
		Name            string `json:"name"`
		ProjectUUID     string `json:"projectUuid"`
		EnvironmentName string `json:"environmentName"`
		EnvironmentUUID string `json:"environmentUuid"`
		ServerUUID      string `json:"serverUuid"`
		DockerCompose   string `json:"dockerCompose"`
		InstantDeploy   bool   `json:"instantDeploy"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.CrearServicio(c.Request.Context(), NuevoServicio(cuerpo))
	h.responderMensaje(c, res, err)
}

func (h *Handler) crearProyecto(c *gin.Context) {
	var cuerpo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.CrearProyecto(c.Request.Context(), cuerpo.Name, cuerpo.Description)
	h.responderMensaje(c, res, err)
}

func (h *Handler) actualizarProyecto(c *gin.Context) {
	var cuerpo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.ActualizarProyecto(c.Request.Context(), c.Param("uuid"),
		cuerpo.Name, cuerpo.Description)
	h.responderMensaje(c, res, err)
}

// ── Variables ──────────────────────────────────────────────────────────────

func (h *Handler) leerVariable(c *gin.Context) (NuevaVariable, bool) {
	var cuerpo struct {
		Key       string `json:"key"`
		Value     string `json:"value"`
		IsPreview bool   `json:"isPreview"`
		IsLiteral bool   `json:"isLiteral"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return NuevaVariable{}, false
	}
	return NuevaVariable(cuerpo), true
}

func (h *Handler) crearVariable(c *gin.Context) {
	in, ok := h.leerVariable(c)
	if !ok {
		return
	}
	res, err := h.svc.CrearVariable(c.Request.Context(), c.Param("uuid"), in)
	h.responderMensaje(c, res, err)
}

func (h *Handler) actualizarVariable(c *gin.Context) {
	in, ok := h.leerVariable(c)
	if !ok {
		return
	}
	res, err := h.svc.ActualizarVariable(c.Request.Context(), c.Param("uuid"), in)
	h.responderMensaje(c, res, err)
}

func (h *Handler) borrarVariable(c *gin.Context) {
	res, err := h.svc.BorrarVariable(c.Request.Context(), c.Param("uuid"), c.Param("envUuid"))
	h.responderMensaje(c, res, err)
}

// ── Auxiliares ─────────────────────────────────────────────────────────────

func (h *Handler) responderMensaje(c *gin.Context, res kernel.Result[string], err error) {
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensaje": res.Value})
}

func aAppDTO(a App) appDTO {
	return appDTO{
		UUID: a.UUID, Name: a.Name, Description: a.Description, FQDN: a.FQDN,
		Status: a.Status, State: string(a.State), StateLabel: EtiquetasDeEstado[a.State],
		Healthy: a.Healthy, GitRepository: a.GitRepository, GitBranch: a.GitBranch,
		BuildPack: a.BuildPack, ProjectName: a.ProjectName,
		EnvironmentName: a.EnvironmentName, PortsExposes: a.PortsExposes,
		InstallCommand: a.InstallCommand, BuildCommand: a.BuildCommand,
		StartCommand: a.StartCommand, BaseDirectory: a.BaseDirectory,
	}
}

func aRecursosDTO(rs []Recurso) []recursoDTO {
	salida := make([]recursoDTO, 0, len(rs))
	for _, r := range rs {
		salida = append(salida, recursoDTO{
			UUID: r.UUID, Name: r.Name, Type: r.Type, Status: r.Status,
			State: string(r.State), StateLabel: EtiquetasDeEstado[r.State],
		})
	}
	return salida
}
