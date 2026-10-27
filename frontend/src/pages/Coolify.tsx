import { useState } from "react";
import type { FormEvent } from "react";

import { api, ApiError } from "../api/client";
import type {
  AppCoolify, CatalogoCoolify, ConfigCoolify, DespliegueCoolify, ProyectoCoolify, RecursoCoolify, ResumenCoolify,
} from "../api/tipos";
import { Alerta } from "../ui/Alerta";
import { Cabecera } from "../ui/Cabecera";
import { Confirmar } from "../ui/Confirmar";
import { Icono } from "../ui/Iconos";
import type { NombreIcono } from "../ui/Iconos";
import { Segmentado } from "../ui/Segmentado";
import { Esqueletos, Vacio } from "../ui/Vacio";
import { useDatos } from "../ui/useDatos";
import { hace } from "../ui/tiempo";
import { AppDetalle } from "./coolify/AppDetalle";
import { DialogoNuevaApp, DialogoNuevaBase, DialogoNuevoServicio, DialogoProyecto } from "./coolify/DialogosCrear";
import { FichaEstado } from "./coolify/ficha";

type Pestana = "apps" | "recursos" | "despliegues" | "proyectos";
type Creando = "app" | "base" | "servicio" | "proyecto" | null;
type KindRecurso = "applications" | "databases" | "services" | "projects";

interface Pendiente {
  kind: KindRecurso;
  uuid: string;
  nombre: string;
}

/** Coolify: ver y manejar lo que está desplegado sin salir del panel. */
export function Coolify() {
  const config = useDatos<ConfigCoolify | null>("/coolify/config");
  const conectado = config.datos != null;

  return (
    <div className="pagina">
      <Cabecera
        etiqueta="coolify"
        titular="Lo desplegado, *a la vista*."
        entradilla="Aplicaciones, bases de datos y despliegues de tu servidor, con sus acciones a mano."
      />
      {config.error && <Alerta tipo="error">{config.error}</Alerta>}
      {config.cargando && !config.datos && config.datos !== null ? (
        <Esqueletos cantidad={2} />
      ) : conectado ? (
        <Panel config={config.datos!} alDesconectar={config.recargar} />
      ) : (
        <Conexion alConectar={config.recargar} />
      )}
    </div>
  );
}

function Conexion({ alConectar }: { alConectar(): void }) {
  const [url, setUrl] = useState("");
  const [token, setToken] = useState("");
  const [guardando, setGuardando] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function conectar(e: FormEvent) {
    e.preventDefault();
    setGuardando(true);
    setError(null);
    try {
      await api.post("/coolify/config", { baseUrl: url, token });
      setToken("");
      alConectar();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudo conectar.");
    } finally {
      setGuardando(false);
    }
  }

  return (
    <form className="cristal panel subir" style={{ "--i": 1 } as React.CSSProperties} onSubmit={conectar}>
      <h2>Conecta tu Coolify</h2>
      <p className="texto-apagado">Se necesita la dirección y un API token (en Coolify: Keys & Tokens → API tokens). El token se guarda cifrado y no se vuelve a mostrar.</p>
      <div className="campo">
        <label htmlFor="co-url">Dirección</label>
        <input id="co-url" type="url" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://coolify.midominio.com" required />
      </div>
      <div className="campo">
        <label htmlFor="co-token">API token</label>
        <input id="co-token" type="password" className="entrada-mono" value={token} onChange={(e) => setToken(e.target.value)} autoComplete="off" required />
      </div>
      {error && <Alerta tipo="error">{error}</Alerta>}
      <div className="acciones-fila">
        <button type="submit" className="btn btn-principal" disabled={guardando}>
          <Icono nombre="link" tamano={15} />
          {guardando ? "Conectando…" : "Conectar"}
        </button>
      </div>
    </form>
  );
}

function Panel({ config, alDesconectar }: { config: ConfigCoolify; alDesconectar(): void }) {
  const resumen = useDatos<ResumenCoolify>("/coolify/overview");
  const catalogo = useDatos<CatalogoCoolify>("/coolify/catalog");
  const [pestana, setPestana] = useState<Pestana>("apps");
  const despliegues = useDatos<DespliegueCoolify[]>(pestana === "despliegues" ? "/coolify/deployments" : null);

  const [abierta, setAbierta] = useState<string | null>(null);
  const [creando, setCreando] = useState<Creando>(null);
  const [proyectoEditado, setProyectoEditado] = useState<ProyectoCoolify | null>(null);
  const [porDesplegar, setPorDesplegar] = useState<AppCoolify | null>(null);
  const [porBorrar, setPorBorrar] = useState<Pendiente | null>(null);
  const [porDesconectar, setPorDesconectar] = useState(false);
  const [ocupado, setOcupado] = useState<string | null>(null);
  const [aviso, setAviso] = useState<{ tipo: "info" | "error"; texto: string } | null>(null);

  const r = resumen.datos;
  const app = abierta ? r?.apps.find((a) => a.uuid === abierta) ?? null : null;

  /** Ejecuta una acción sobre el servidor y enseña el resultado. */
  async function hacer(clave: string, f: () => Promise<unknown>) {
    setOcupado(clave);
    setAviso(null);
    try {
      const res = await f();
      setAviso({ tipo: "info", texto: (res as { mensaje?: string } | undefined)?.mensaje || "Hecho." });
      resumen.recargar();
      if (pestana === "despliegues") despliegues.recargar();
    } catch (e) {
      setAviso({ tipo: "error", texto: e instanceof ApiError ? e.message : "No se pudo completar la acción." });
    } finally {
      setOcupado(null);
      setPorDesplegar(null);
      setPorBorrar(null);
    }
  }

  const accionApp = (a: AppCoolify, accion: "start" | "stop" | "restart") =>
    hacer(`${a.uuid}:${accion}`, () => api.post(`/coolify/applications/${a.uuid}/${accion}`));
  const accionRecurso = (kind: "databases" | "services", x: RecursoCoolify, accion: "start" | "stop" | "restart") =>
    hacer(`${x.uuid}:${accion}`, () => api.post(`/coolify/resources/${kind}/${x.uuid}/${accion}`));

  async function desconectar() {
    try {
      await api.del(`/coolify/config`);
      alDesconectar();
    } catch (e) {
      setAviso({ tipo: "error", texto: e instanceof Error ? e.message : "No se pudo desconectar." });
      setPorDesconectar(false);
    }
  }

  const botonCrear: Record<Pestana, { texto: string; accion: () => void } | null> = {
    apps: { texto: "Nueva aplicación", accion: () => setCreando("app") },
    recursos: { texto: "Nueva base", accion: () => setCreando("base") },
    despliegues: null,
    proyectos: { texto: "Nuevo proyecto", accion: () => setCreando("proyecto") },
  };
  const crear = botonCrear[pestana];

  return (
    <>
      <div className="barra-herramientas subir">
        <span className="ficha" title={config.baseUrl}><Icono nombre="server" tamano={12} />{config.baseUrl.replace(/^https?:\/\//, "")}</span>
        {!app && (
          <Segmentado<Pestana>
            etiqueta="Apartado"
            valor={pestana}
            opciones={[
              { valor: "apps", etiqueta: `Aplicaciones${r ? ` · ${r.apps.length}` : ""}` },
              { valor: "recursos", etiqueta: `Bases y servicios${r ? ` · ${r.databases.length + r.services.length}` : ""}` },
              { valor: "despliegues", etiqueta: "Despliegues" },
              { valor: "proyectos", etiqueta: `Proyectos${r ? ` · ${r.projects.length}` : ""}` },
            ]}
            alCambiar={setPestana}
          />
        )}
        <span style={{ flex: 1 }} />
        <button className="btn btn-pequeno" onClick={() => { resumen.recargar(); despliegues.recargar(); }} disabled={resumen.cargando}>
          <Icono nombre="refresh" tamano={14} />
          Actualizar
        </button>
        {crear && !app && (
          <button className="btn btn-principal btn-pequeno" onClick={crear.accion}>
            <Icono nombre="plus" tamano={14} />
            {crear.texto}
          </button>
        )}
        {pestana === "recursos" && !app && (
          <button className="btn btn-pequeno" onClick={() => setCreando("servicio")}>
            <Icono nombre="plus" tamano={14} />
            Nuevo servicio
          </button>
        )}
        <button className="btn btn-pequeno btn-fantasma" onClick={() => setPorDesconectar(true)}>Desconectar</button>
      </div>

      {resumen.error && <Alerta tipo="error">{resumen.error}</Alerta>}
      {aviso && <Alerta tipo={aviso.tipo} alCerrar={() => setAviso(null)}>{aviso.texto}</Alerta>}

      {resumen.cargando && !r ? (
        <Esqueletos cantidad={3} />
      ) : r && app ? (
        <AppDetalle
          key={app.uuid}
          app={app}
          catalogo={catalogo.datos}
          alVolver={() => setAbierta(null)}
          alCambiar={() => resumen.recargar()}
        />
      ) : r && pestana === "apps" ? (
        r.apps.length === 0 ? (
          <Vacio icono="rocket" titular="Ninguna *aplicación*." accion={<button className="btn btn-principal" onClick={() => setCreando("app")}><Icono nombre="plus" tamano={16} />Crear una</button>}>
            Cuando despliegues algo en Coolify aparecerá aquí.
          </Vacio>
        ) : (
          <div className="rejilla">
            {r.apps.map((a, i) => (
              <TarjetaRecurso
                key={a.uuid}
                indice={i + 1}
                icono="rocket"
                nombre={a.name}
                sub={a.fqdn?.replace(/^https?:\/\//, "") ?? a.gitRepository?.replace(/^https?:\/\/github\.com\//, "") ?? "sin dominio"}
                estado={a.state}
                etiquetaEstado={a.stateLabel}
                ocupado={ocupado?.startsWith(a.uuid) ?? false}
                enlace={a.fqdn}
                alAbrir={() => setAbierta(a.uuid)}
                acciones={[
                  { icono: "rocket", texto: "Desplegar", alPulsar: () => setPorDesplegar(a) },
                  { icono: "refresh", texto: "Reiniciar", alPulsar: () => void accionApp(a, "restart") },
                  a.state === "stopped"
                    ? { icono: "play", texto: "Arrancar", alPulsar: () => void accionApp(a, "start") }
                    : { icono: "stop", texto: "Parar", alPulsar: () => void accionApp(a, "stop") },
                ]}
                alBorrar={() => setPorBorrar({ kind: "applications", uuid: a.uuid, nombre: a.name })}
              >
                <div className="fichas">
                  {a.buildPack && <span className="ficha">{a.buildPack}</span>}
                  {a.gitBranch && <span className="ficha">{a.gitBranch}</span>}
                  {a.projectName && <span className="ficha">{a.projectName}</span>}
                </div>
              </TarjetaRecurso>
            ))}
          </div>
        )
      ) : r && pestana === "recursos" ? (
        r.databases.length + r.services.length === 0 ? (
          <Vacio icono="database" titular="Sin *bases ni servicios*.">Aquí salen las bases de datos y los servicios compose.</Vacio>
        ) : (
          <div className="rejilla">
            {[...r.databases.map((x) => ({ x, kind: "databases" as const })), ...r.services.map((x) => ({ x, kind: "services" as const }))].map(({ x, kind }, i) => (
              <TarjetaRecurso
                key={x.uuid}
                indice={i + 1}
                icono={kind === "databases" ? "database" : "server"}
                nombre={x.name}
                sub={kind === "databases" ? x.type : "servicio compose"}
                estado={x.state}
                etiquetaEstado={x.stateLabel}
                ocupado={ocupado?.startsWith(x.uuid) ?? false}
                acciones={[
                  { icono: "refresh", texto: "Reiniciar", alPulsar: () => void accionRecurso(kind, x, "restart") },
                  x.state === "stopped"
                    ? { icono: "play", texto: "Arrancar", alPulsar: () => void accionRecurso(kind, x, "start") }
                    : { icono: "stop", texto: "Parar", alPulsar: () => void accionRecurso(kind, x, "stop") },
                ]}
                alBorrar={() => setPorBorrar({ kind, uuid: x.uuid, nombre: x.name })}
              />
            ))}
          </div>
        )
      ) : r && pestana === "proyectos" ? (
        r.projects.length === 0 ? (
          <Vacio icono="list" titular="Sin *proyectos*.">Los proyectos agrupan aplicaciones y bases.</Vacio>
        ) : (
          <div className="rejilla">
            {r.projects.map((p, i) => (
              <article key={p.uuid} className="tarjeta cristal subir" style={{ "--i": i + 1 } as React.CSSProperties}>
                <div className="tarjeta-cab">
                  <span className="tarjeta-avatar"><Icono nombre="list" tamano={18} /></span>
                  <div className="tarjeta-titulo">
                    <span className="tarjeta-nombre">{p.name}</span>
                    <span className="tarjeta-sub">{p.description || "sin descripción"}</span>
                  </div>
                  <div className="tarjeta-iconos">
                    <button className="accion" onClick={() => setProyectoEditado(p)} aria-label={`Editar ${p.name}`} title="Editar"><Icono nombre="edit" tamano={16} /></button>
                    <button className="accion peligro" onClick={() => setPorBorrar({ kind: "projects", uuid: p.uuid, nombre: p.name })} aria-label={`Borrar ${p.name}`} title="Borrar"><Icono nombre="trash" tamano={16} /></button>
                  </div>
                </div>
              </article>
            ))}
          </div>
        )
      ) : r && pestana === "despliegues" ? (
        <Despliegues
          datos={despliegues.datos}
          cargando={despliegues.cargando}
          error={despliegues.error}
          ocupado={ocupado}
          alCancelar={(d) => void hacer(`c:${d.uuid}`, () => api.post(`/coolify/deployments/${d.uuid}/cancel`))}
        />
      ) : null}

      {creando === "app" && r && <DialogoNuevaApp resumen={r} catalogo={catalogo.datos} alCerrar={() => setCreando(null)} alCrear={(m) => { setCreando(null); setAviso({ tipo: "info", texto: m }); resumen.recargar(); }} />}
      {creando === "base" && r && <DialogoNuevaBase resumen={r} catalogo={catalogo.datos} alCerrar={() => setCreando(null)} alCrear={(m) => { setCreando(null); setAviso({ tipo: "info", texto: m }); resumen.recargar(); }} />}
      {creando === "servicio" && r && <DialogoNuevoServicio resumen={r} catalogo={catalogo.datos} alCerrar={() => setCreando(null)} alCrear={(m) => { setCreando(null); setAviso({ tipo: "info", texto: m }); resumen.recargar(); }} />}
      {(creando === "proyecto" || proyectoEditado) && (
        <DialogoProyecto
          key={proyectoEditado?.uuid ?? "nuevo"}
          proyecto={proyectoEditado}
          alCerrar={() => { setCreando(null); setProyectoEditado(null); }}
          alCrear={(m) => { setCreando(null); setProyectoEditado(null); setAviso({ tipo: "info", texto: m }); resumen.recargar(); }}
        />
      )}

      {porDesplegar && (
        <Confirmar
          titulo={`¿Desplegar ${porDesplegar.name}?`}
          texto="Se construirá de nuevo desde el repositorio y se sustituirá la versión que está en marcha."
          etiquetaConfirmar="Desplegar"
          trabajando={ocupado?.startsWith(porDesplegar.uuid) ?? false}
          alConfirmar={() => void hacer(`${porDesplegar.uuid}:deploy`, () => api.post(`/coolify/applications/${porDesplegar.uuid}/deploy`))}
          alCancelar={() => setPorDesplegar(null)}
        />
      )}
      {porBorrar && (
        <Confirmar
          titulo={`¿Borrar «${porBorrar.nombre}»?`}
          texto={porBorrar.kind === "projects" ? "Solo se puede borrar un proyecto vacío." : "Se eliminará de Coolify y no se puede deshacer."}
          etiquetaConfirmar="Borrar"
          peligro
          trabajando={ocupado === `x:${porBorrar.uuid}`}
          alConfirmar={() => {
            if (abierta === porBorrar.uuid) setAbierta(null);
            void hacer(`x:${porBorrar.uuid}`, () => api.del(`/coolify/resources/${porBorrar.kind}/${porBorrar.uuid}`));
          }}
          alCancelar={() => setPorBorrar(null)}
        />
      )}
      {porDesconectar && (
        <Confirmar
          titulo="¿Desconectar Coolify?"
          texto="Se olvidará el token guardado. Tus aplicaciones en Coolify no se tocan."
          etiquetaConfirmar="Desconectar"
          peligro
          alConfirmar={() => void desconectar()}
          alCancelar={() => setPorDesconectar(false)}
        />
      )}
    </>
  );
}

interface AccionTarjeta {
  icono: NombreIcono;
  texto: string;
  alPulsar(): void;
}

function TarjetaRecurso({ indice, icono, nombre, sub, estado, etiquetaEstado, ocupado, enlace, acciones, alAbrir, alBorrar, children }: {
  indice: number;
  icono: NombreIcono;
  nombre: string;
  sub: string;
  estado: string;
  etiquetaEstado: string;
  ocupado: boolean;
  enlace?: string | null;
  acciones: AccionTarjeta[];
  alAbrir?: () => void;
  alBorrar(): void;
  children?: React.ReactNode;
}) {
  return (
    <article className="tarjeta cristal subir" style={{ "--i": indice } as React.CSSProperties} aria-label={`${nombre}: ${etiquetaEstado}`} aria-busy={ocupado}>
      <div className="tarjeta-cab">
        <span className="tarjeta-avatar"><Icono nombre={icono} tamano={18} /></span>
        <div className="tarjeta-titulo">
          <span className="tarjeta-nombre" title={nombre}>{nombre}</span>
          <span className="tarjeta-sub" title={sub}>{sub}</span>
        </div>
        <div className="tarjeta-iconos">
          {enlace && <a className="accion" href={enlace} target="_blank" rel="noreferrer" aria-label={`Abrir ${nombre}`} title="Abrir el sitio"><Icono nombre="link" tamano={16} /></a>}
          <button className="accion peligro" onClick={alBorrar} aria-label={`Borrar ${nombre}`} title="Borrar"><Icono nombre="trash" tamano={16} /></button>
        </div>
      </div>
      <FichaEstado estado={estado} etiqueta={etiquetaEstado} />
      {children}
      <div className="tarjeta-pie acciones-tarjeta">
        {alAbrir && (
          <button className="btn btn-pequeno" onClick={alAbrir}>
            <Icono nombre="edit" tamano={13} />
            Detalle
          </button>
        )}
        {acciones.map((a) => (
          <button key={a.texto} className="btn btn-pequeno btn-fantasma" onClick={a.alPulsar} disabled={ocupado} aria-label={`${a.texto} ${nombre}`}>
            <Icono nombre={a.icono} tamano={13} />
            {a.texto}
          </button>
        ))}
      </div>
    </article>
  );
}

function Despliegues({ datos, cargando, error, ocupado, alCancelar }: {
  datos: DespliegueCoolify[] | null;
  cargando: boolean;
  error: string | null;
  ocupado: string | null;
  alCancelar(d: DespliegueCoolify): void;
}) {
  if (error) return <Alerta tipo="error">{error}</Alerta>;
  if (cargando && !datos) return <Esqueletos cantidad={3} />;
  if (!datos || datos.length === 0) return <Vacio icono="clock" titular="Sin *despliegues*.">No hay ninguno en curso ni reciente.</Vacio>;

  return (
    <ul className="cristal lista-despliegues subir" aria-label="Despliegues">
      {datos.map((d) => (
        <li key={d.uuid}>
          <FichaEstado estado={d.status} etiqueta={d.statusLabel} />
          <span className="despliegue-app">
            <strong>{d.applicationName}</strong>
            <span className="texto-apagado" title={d.commitMessage}>{d.commit ? d.commit.slice(0, 7) : ""} {d.commitMessage}</span>
          </span>
          <span className="ejecucion-cuando">{hace(d.createdAt)}</span>
          {(d.status === "queued" || d.status === "in_progress") && (
            <button className="btn btn-pequeno btn-fantasma" onClick={() => alCancelar(d)} disabled={ocupado === `c:${d.uuid}`}>Cancelar</button>
          )}
        </li>
      ))}
    </ul>
  );
}
