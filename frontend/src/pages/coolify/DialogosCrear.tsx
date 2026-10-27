import { useState } from "react";

import { api, ApiError } from "../../api/client";
import type { CatalogoCoolify, EntornoCoolify, ResumenCoolify } from "../../api/tipos";
import { Dialogo } from "../../ui/Dialogo";
import { useDatos } from "../../ui/useDatos";

interface Base {
  resumen: ResumenCoolify;
  catalogo: CatalogoCoolify | null;
  alCerrar(): void;
  /** Se llama con el mensaje del servidor cuando se creó. */
  alCrear(mensaje: string): void;
}

/** Envía y gestiona el estado de «enviando» y el error de un diálogo de creación. */
function useEnvio(alCrear: (m: string) => void) {
  const [enviando, setEnviando] = useState(false);
  const [error, setError] = useState<string | null>(null);
  async function enviar(ruta: string, cuerpo: unknown) {
    setEnviando(true);
    setError(null);
    try {
      const r = await api.post<{ mensaje: string }>(ruta, cuerpo);
      alCrear(r.mensaje);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "No se pudo crear.");
    } finally {
      setEnviando(false);
    }
  }
  return { enviando, error, enviar };
}

/** Proyecto, entorno y servidor: lo mismo se pide al crear casi cualquier cosa. */
function Destino({ resumen, proyecto, setProyecto, entorno, setEntorno, servidor, setServidor, soloServidorYProyecto = false }: {
  resumen: ResumenCoolify;
  proyecto: string; setProyecto(v: string): void;
  entorno: string; setEntorno(v: string): void;
  servidor: string; setServidor(v: string): void;
  soloServidorYProyecto?: boolean;
}) {
  const entornos = useDatos<EntornoCoolify[]>(proyecto ? `/coolify/projects/${proyecto}/environments` : null);
  return (
    <>
      <div className="fila">
        <div className="campo">
          <label htmlFor="cc-proy">Proyecto</label>
          <select id="cc-proy" value={proyecto} onChange={(e) => { setProyecto(e.target.value); setEntorno(""); }} required>
            <option value="">Elige uno…</option>
            {resumen.projects.map((p) => <option key={p.uuid} value={p.uuid}>{p.name}</option>)}
          </select>
        </div>
        {!soloServidorYProyecto && (
          <div className="campo">
            <label htmlFor="cc-ent">Entorno</label>
            <select id="cc-ent" value={entorno} onChange={(e) => setEntorno(e.target.value)} required disabled={!proyecto}>
              <option value="">{entornos.cargando ? "Cargando…" : "Elige uno…"}</option>
              {entornos.datos?.map((x) => <option key={x.uuid} value={x.uuid}>{x.name}</option>)}
            </select>
          </div>
        )}
      </div>
      <div className="campo">
        <label htmlFor="cc-srv">Servidor</label>
        <select id="cc-srv" value={servidor} onChange={(e) => setServidor(e.target.value)} required>
          <option value="">Elige uno…</option>
          {resumen.servers.map((s) => <option key={s.uuid} value={s.uuid}>{s.name} · {s.ip}{s.reachable ? "" : " (sin conexión)"}</option>)}
        </select>
      </div>
    </>
  );
}

export function DialogoNuevaApp({ resumen, catalogo, alCerrar, alCrear }: Base) {
  const { enviando, error, enviar } = useEnvio(alCrear);
  const [origen, setOrigen] = useState<"public" | "dockerfile">("public");
  const [nombre, setNombre] = useState("");
  const [proyecto, setProyecto] = useState("");
  const [entorno, setEntorno] = useState("");
  const [servidor, setServidor] = useState(resumen.servers[0]?.uuid ?? "");
  const [repo, setRepo] = useState("");
  const [rama, setRama] = useState("main");
  const [buildPack, setBuildPack] = useState("nixpacks");
  const [puerto, setPuerto] = useState("3000");
  const [dominio, setDominio] = useState("");
  const [carpeta, setCarpeta] = useState("");
  const [dockerfile, setDockerfile] = useState("");
  const [desplegar, setDesplegar] = useState(true);

  const entornoNombre = ""; // Coolify acepta el uuid; el nombre queda vacío.

  return (
    <Dialogo
      etiqueta="nueva aplicación"
      titular="Una app *nueva*."
      etiquetaEnviar="Crear"
      error={error}
      enviando={enviando}
      alCerrar={alCerrar}
      alEnviar={() => void enviar("/coolify/applications", {
        source: origen, name: nombre, projectUuid: proyecto, environmentUuid: entorno, environmentName: entornoNombre,
        serverUuid: servidor, domains: dominio, portsExposes: puerto, instantDeploy: desplegar,
        baseDirectory: carpeta, gitRepository: repo, gitBranch: rama, buildPack, dockerfile,
      })}
    >
      <div className="campo">
        <span className="etiqueta">de dónde sale</span>
        <div className="segmentado" role="radiogroup" aria-label="Origen">
          <button type="button" role="radio" aria-checked={origen === "public"} className={origen === "public" ? "seleccionado" : ""} onClick={() => setOrigen("public")}>Repositorio público</button>
          <button type="button" role="radio" aria-checked={origen === "dockerfile"} className={origen === "dockerfile" ? "seleccionado" : ""} onClick={() => setOrigen("dockerfile")}>Dockerfile pegado</button>
        </div>
      </div>
      <div className="campo">
        <label htmlFor="na-nombre">Nombre</label>
        <input id="na-nombre" type="text" value={nombre} onChange={(e) => setNombre(e.target.value)} required autoFocus />
      </div>
      <Destino resumen={resumen} proyecto={proyecto} setProyecto={setProyecto} entorno={entorno} setEntorno={setEntorno} servidor={servidor} setServidor={setServidor} />

      {origen === "public" ? (
        <>
          <div className="campo">
            <label htmlFor="na-repo">Repositorio</label>
            <input id="na-repo" type="url" value={repo} onChange={(e) => setRepo(e.target.value)} placeholder="https://github.com/usuario/proyecto" required />
          </div>
          <div className="fila">
            <div className="campo">
              <label htmlFor="na-rama">Rama</label>
              <input id="na-rama" type="text" className="entrada-mono" value={rama} onChange={(e) => setRama(e.target.value)} spellCheck={false} />
            </div>
            <div className="campo">
              <label htmlFor="na-bp">Forma de construirlo</label>
              <select id="na-bp" value={buildPack} onChange={(e) => setBuildPack(e.target.value)}>
                {(catalogo?.buildPacks ?? []).map((b) => <option key={b.id} value={b.id}>{b.label}</option>)}
              </select>
            </div>
          </div>
          <div className="campo">
            <label htmlFor="na-dir">Subcarpeta</label>
            <input id="na-dir" type="text" className="entrada-mono" value={carpeta} onChange={(e) => setCarpeta(e.target.value)} placeholder="raíz del repositorio" spellCheck={false} />
          </div>
        </>
      ) : (
        <div className="campo">
          <label htmlFor="na-df">Dockerfile</label>
          <textarea id="na-df" className="entrada-mono" rows={8} value={dockerfile} onChange={(e) => setDockerfile(e.target.value)} spellCheck={false} required />
        </div>
      )}

      <div className="fila">
        <div className="campo">
          <label htmlFor="na-puerto">Puerto</label>
          <input id="na-puerto" type="text" inputMode="numeric" className="entrada-mono" value={puerto} onChange={(e) => setPuerto(e.target.value)} />
        </div>
        <div className="campo">
          <label htmlFor="na-dom">Dominio</label>
          <input id="na-dom" type="text" className="entrada-mono" value={dominio} onChange={(e) => setDominio(e.target.value)} placeholder="https://app.midominio.com" spellCheck={false} />
        </div>
      </div>
      <label className="casilla">
        <input type="checkbox" checked={desplegar} onChange={(e) => setDesplegar(e.target.checked)} />
        Desplegar nada más crearla
      </label>
    </Dialogo>
  );
}

export function DialogoNuevaBase({ resumen, catalogo, alCerrar, alCrear }: Base) {
  const { enviando, error, enviar } = useEnvio(alCrear);
  const [tipo, setTipo] = useState("postgresql");
  const [nombre, setNombre] = useState("");
  const [proyecto, setProyecto] = useState("");
  const [entorno, setEntorno] = useState("");
  const [servidor, setServidor] = useState(resumen.servers[0]?.uuid ?? "");
  const [desplegar, setDesplegar] = useState(true);

  return (
    <Dialogo
      etiqueta="nueva base de datos"
      titular="Una base *nueva*."
      etiquetaEnviar="Crear"
      error={error}
      enviando={enviando}
      alCerrar={alCerrar}
      alEnviar={() => void enviar("/coolify/databases", {
        type: tipo, name: nombre, projectUuid: proyecto, environmentUuid: entorno, serverUuid: servidor, instantDeploy: desplegar,
      })}
    >
      <div className="fila">
        <div className="campo">
          <label htmlFor="nb-tipo">Tipo</label>
          <select id="nb-tipo" value={tipo} onChange={(e) => setTipo(e.target.value)}>
            {(catalogo?.databaseTypes ?? []).map((t) => <option key={t.id} value={t.id}>{t.label}</option>)}
          </select>
        </div>
        <div className="campo">
          <label htmlFor="nb-nombre">Nombre</label>
          <input id="nb-nombre" type="text" value={nombre} onChange={(e) => setNombre(e.target.value)} autoFocus />
        </div>
      </div>
      <Destino resumen={resumen} proyecto={proyecto} setProyecto={setProyecto} entorno={entorno} setEntorno={setEntorno} servidor={servidor} setServidor={setServidor} />
      <label className="casilla">
        <input type="checkbox" checked={desplegar} onChange={(e) => setDesplegar(e.target.checked)} />
        Arrancarla nada más crearla
      </label>
    </Dialogo>
  );
}

export function DialogoNuevoServicio({ resumen, alCerrar, alCrear }: Base) {
  const { enviando, error, enviar } = useEnvio(alCrear);
  const [nombre, setNombre] = useState("");
  const [compose, setCompose] = useState("");
  const [proyecto, setProyecto] = useState("");
  const [entorno, setEntorno] = useState("");
  const [servidor, setServidor] = useState(resumen.servers[0]?.uuid ?? "");
  const [desplegar, setDesplegar] = useState(true);

  return (
    <Dialogo
      etiqueta="nuevo servicio"
      titular="Un servicio *compose*."
      etiquetaEnviar="Crear"
      error={error}
      enviando={enviando}
      alCerrar={alCerrar}
      alEnviar={() => void enviar("/coolify/services", {
        name: nombre, dockerCompose: compose, projectUuid: proyecto, environmentUuid: entorno, serverUuid: servidor, instantDeploy: desplegar,
      })}
    >
      <div className="campo">
        <label htmlFor="ns-nombre">Nombre</label>
        <input id="ns-nombre" type="text" value={nombre} onChange={(e) => setNombre(e.target.value)} required autoFocus />
      </div>
      <Destino resumen={resumen} proyecto={proyecto} setProyecto={setProyecto} entorno={entorno} setEntorno={setEntorno} servidor={servidor} setServidor={setServidor} />
      <div className="campo">
        <label htmlFor="ns-compose">docker-compose.yml</label>
        <textarea id="ns-compose" className="entrada-mono" rows={9} value={compose} onChange={(e) => setCompose(e.target.value)} spellCheck={false} required />
      </div>
      <label className="casilla">
        <input type="checkbox" checked={desplegar} onChange={(e) => setDesplegar(e.target.checked)} />
        Arrancarlo nada más crearlo
      </label>
    </Dialogo>
  );
}

export function DialogoProyecto({ proyecto, alCerrar, alCrear }: { proyecto: { uuid: string; name: string; description: string | null } | null; alCerrar(): void; alCrear(mensaje: string): void }) {
  const { enviando, error, enviar } = useEnvio(alCrear);
  const [nombre, setNombre] = useState(proyecto?.name ?? "");
  const [descripcion, setDescripcion] = useState(proyecto?.description ?? "");
  const [editando, setEditando] = useState(false);
  const [errorEdicion, setErrorEdicion] = useState<string | null>(null);

  async function editar() {
    if (!proyecto) return;
    setEditando(true);
    setErrorEdicion(null);
    try {
      const r = await api.patch<{ mensaje: string }>(`/coolify/projects/${proyecto.uuid}`, { name: nombre, description: descripcion });
      alCrear(r.mensaje);
    } catch (e) {
      setErrorEdicion(e instanceof ApiError ? e.message : "No se pudo guardar.");
    } finally {
      setEditando(false);
    }
  }

  return (
    <Dialogo
      etiqueta={proyecto ? "editar proyecto" : "nuevo proyecto"}
      titular={proyecto ? "Renombra el *proyecto*." : "Un proyecto *nuevo*."}
      etiquetaEnviar={proyecto ? "Guardar" : "Crear"}
      error={proyecto ? errorEdicion : error}
      enviando={proyecto ? editando : enviando}
      alCerrar={alCerrar}
      alEnviar={() => (proyecto ? void editar() : void enviar("/coolify/projects", { name: nombre, description: descripcion }))}
    >
      <div className="campo">
        <label htmlFor="pr-nombre">Nombre</label>
        <input id="pr-nombre" type="text" value={nombre} onChange={(e) => setNombre(e.target.value)} required autoFocus />
      </div>
      <div className="campo">
        <label htmlFor="pr-desc">Descripción</label>
        <input id="pr-desc" type="text" value={descripcion} onChange={(e) => setDescripcion(e.target.value)} />
      </div>
    </Dialogo>
  );
}
