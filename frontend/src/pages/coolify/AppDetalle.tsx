import { useState } from "react";
import type { FormEvent } from "react";

import { api, ApiError } from "../../api/client";
import type { AppCoolify, CatalogoCoolify, VariableCoolify } from "../../api/tipos";
import { Alerta } from "../../ui/Alerta";
import { Codigo } from "../../ui/Codigo";
import { Confirmar } from "../../ui/Confirmar";
import { Dialogo } from "../../ui/Dialogo";
import { Icono } from "../../ui/Iconos";
import { Segmentado } from "../../ui/Segmentado";
import { Esqueletos, Vacio } from "../../ui/Vacio";
import { useDatos } from "../../ui/useDatos";
import { FichaEstado } from "./ficha";
import { Selector } from "../../ui/Selector";

type Pestana = "config" | "variables" | "registros";

interface Props {
  app: AppCoolify;
  catalogo: CatalogoCoolify | null;
  alVolver(): void;
  /** Hay que refrescar el resumen: cambió algo que se ve en la lista. */
  alCambiar(mensaje?: string): void;
}

/** Una aplicación por dentro: su configuración, sus variables y sus registros. */
export function AppDetalle({ app, catalogo, alVolver, alCambiar }: Props) {
  const [pestana, setPestana] = useState<Pestana>("config");

  return (
    <section className="subir" style={{ display: "grid", gap: "1rem" }} aria-label={`Detalle de ${app.name}`}>
      <div className="panel-cab">
        <button className="btn btn-pequeno btn-fantasma" onClick={alVolver}>
          <Icono nombre="chevronL" tamano={14} />
          Volver
        </button>
        <div className="fichas">
          <span className="ficha">{app.name}</span>
          <FichaEstado estado={app.state} etiqueta={app.stateLabel} />
        </div>
        <Segmentado<Pestana>
          etiqueta="Apartado de la aplicación"
          valor={pestana}
          opciones={[
            { valor: "config", etiqueta: "Configuración" },
            { valor: "variables", etiqueta: "Variables" },
            { valor: "registros", etiqueta: "Registros" },
          ]}
          alCambiar={setPestana}
        />
      </div>

      {pestana === "config" && <Configuracion app={app} catalogo={catalogo} alCambiar={alCambiar} />}
      {pestana === "variables" && <Variables uuid={app.uuid} />}
      {pestana === "registros" && <Registros uuid={app.uuid} />}
    </section>
  );
}

function Configuracion({ app, catalogo, alCambiar }: { app: AppCoolify; catalogo: CatalogoCoolify | null; alCambiar(m?: string): void }) {
  const [f, setF] = useState({
    name: app.name,
    description: app.description ?? "",
    domains: app.fqdn ?? "",
    gitBranch: app.gitBranch ?? "",
    buildPack: app.buildPack ?? "nixpacks",
    portsExposes: app.portsExposes,
    installCommand: app.installCommand,
    buildCommand: app.buildCommand,
    startCommand: app.startCommand,
    baseDirectory: app.baseDirectory,
  });
  const [guardando, setGuardando] = useState(false);
  const [aviso, setAviso] = useState<{ tipo: "info" | "error"; texto: string } | null>(null);
  const poner = (k: keyof typeof f) => (e: { target: { value: string } }) => setF((x) => ({ ...x, [k]: e.target.value }));

  async function guardar(e: FormEvent) {
    e.preventDefault();
    setGuardando(true);
    setAviso(null);
    try {
      const r = await api.patch<{ mensaje: string }>(`/coolify/applications/${app.uuid}`, f);
      setAviso({ tipo: "info", texto: r.mensaje });
      alCambiar();
    } catch (err) {
      setAviso({ tipo: "error", texto: err instanceof ApiError ? err.message : "No se pudo guardar." });
    } finally {
      setGuardando(false);
    }
  }

  return (
    <form className="cristal panel" onSubmit={guardar}>
      {app.gitRepository && (
        <p className="texto-apagado">
          {app.gitRepository}
          {app.projectName ? ` · ${app.projectName}` : ""}
          {app.environmentName ? ` / ${app.environmentName}` : ""}
        </p>
      )}
      <div className="fila">
        <div className="campo">
          <label htmlFor="ap-nombre">Nombre</label>
          <input id="ap-nombre" type="text" value={f.name} onChange={poner("name")} required />
        </div>
        <div className="campo">
          <label htmlFor="ap-desc">Descripción</label>
          <input id="ap-desc" type="text" value={f.description} onChange={poner("description")} />
        </div>
      </div>
      <div className="fila">
        <div className="campo">
          <label htmlFor="ap-dom">Dominio</label>
          <input id="ap-dom" type="text" className="entrada-mono" value={f.domains} onChange={poner("domains")} spellCheck={false} />
        </div>
        <div className="campo">
          <label htmlFor="ap-puerto">Puerto</label>
          <input id="ap-puerto" type="text" className="entrada-mono" value={f.portsExposes} onChange={poner("portsExposes")} spellCheck={false} />
        </div>
      </div>
      <div className="fila">
        <div className="campo">
          <label htmlFor="ap-rama">Rama</label>
          <input id="ap-rama" type="text" className="entrada-mono" value={f.gitBranch} onChange={poner("gitBranch")} spellCheck={false} />
        </div>
        <div className="campo">
          <label htmlFor="ap-bp">Forma de construirlo</label>
          <Selector id="ap-bp" value={f.buildPack} onChange={poner("buildPack")}>
            {(catalogo?.buildPacks ?? []).map((b) => <option key={b.id} value={b.id}>{b.label}</option>)}
          </Selector>
        </div>
      </div>
      <div className="campo">
        <label htmlFor="ap-dir">Subcarpeta</label>
        <input id="ap-dir" type="text" className="entrada-mono" value={f.baseDirectory} onChange={poner("baseDirectory")} spellCheck={false} />
      </div>
      <div className="fila-3">
        <div className="campo">
          <label htmlFor="ap-inst">Instalar</label>
          <input id="ap-inst" type="text" className="entrada-mono" value={f.installCommand} onChange={poner("installCommand")} spellCheck={false} />
        </div>
        <div className="campo">
          <label htmlFor="ap-build">Compilar</label>
          <input id="ap-build" type="text" className="entrada-mono" value={f.buildCommand} onChange={poner("buildCommand")} spellCheck={false} />
        </div>
        <div className="campo">
          <label htmlFor="ap-start">Arrancar</label>
          <input id="ap-start" type="text" className="entrada-mono" value={f.startCommand} onChange={poner("startCommand")} spellCheck={false} />
        </div>
      </div>
      {aviso && <Alerta tipo={aviso.tipo}>{aviso.texto}</Alerta>}
      <div className="acciones-fila">
        <button type="submit" className="btn btn-principal" disabled={guardando}>
          <Icono nombre="check" tamano={15} />
          {guardando ? "Guardando…" : "Guardar cambios"}
        </button>
      </div>
      <span className="campo-ayuda">Los cambios se aplican en el próximo despliegue.</span>
    </form>
  );
}

function Variables({ uuid }: { uuid: string }) {
  const { datos, error, cargando, recargar } = useDatos<VariableCoolify[]>(`/coolify/applications/${uuid}/envs`);
  const [editando, setEditando] = useState<VariableCoolify | "nueva" | null>(null);
  const [clave, setClave] = useState("");
  const [valor, setValor] = useState("");
  const [guardando, setGuardando] = useState(false);
  const [errorDialogo, setErrorDialogo] = useState<string | null>(null);
  const [porBorrar, setPorBorrar] = useState<VariableCoolify | null>(null);
  const [borrando, setBorrando] = useState(false);
  const [aviso, setAviso] = useState<string | null>(null);

  function abrir(v: VariableCoolify | "nueva") {
    setEditando(v);
    setClave(v === "nueva" ? "" : v.key);
    setValor(v === "nueva" ? "" : v.value);
    setErrorDialogo(null);
  }

  async function guardar() {
    setGuardando(true);
    setErrorDialogo(null);
    try {
      const cuerpo = { key: clave, value: valor, isPreview: false, isLiteral: editando !== "nueva" && editando ? editando.isLiteral : false };
      if (editando === "nueva") await api.post(`/coolify/applications/${uuid}/envs`, cuerpo);
      else await api.patch(`/coolify/applications/${uuid}/envs`, cuerpo);
      setEditando(null);
      recargar();
    } catch (e) {
      setErrorDialogo(e instanceof ApiError ? e.message : "No se pudo guardar.");
    } finally {
      setGuardando(false);
    }
  }

  async function borrar() {
    if (!porBorrar) return;
    setBorrando(true);
    try {
      await api.del(`/coolify/applications/${uuid}/envs/${porBorrar.uuid}`);
      recargar();
    } catch (e) {
      setAviso(e instanceof Error ? e.message : "No se pudo borrar.");
    } finally {
      setBorrando(false);
      setPorBorrar(null);
    }
  }

  return (
    <div className="cristal panel">
      <div className="panel-cab">
        <h2>Variables de entorno</h2>
        <button className="btn btn-pequeno" onClick={() => abrir("nueva")}>
          <Icono nombre="plus" tamano={14} />
          Añadir
        </button>
      </div>
      {error && <Alerta tipo="error">{error}</Alerta>}
      {aviso && <Alerta tipo="error" alCerrar={() => setAviso(null)}>{aviso}</Alerta>}
      {cargando && !datos ? (
        <Esqueletos cantidad={2} />
      ) : datos?.length === 0 ? (
        <Vacio icono="list" titular="Sin *variables*.">Esta aplicación no tiene variables definidas.</Vacio>
      ) : (
        <ul className="variables">
          {datos?.map((v) => (
            <li key={v.uuid}>
              <span className="variable-clave">{v.key}</span>
              <span className="variable-valor" title={v.value}>{v.value || "—"}</span>
              <span className="fichas">
                {v.isBuildTime && <span className="ficha">build</span>}
                {v.isPreview && <span className="ficha">preview</span>}
              </span>
              <button className="accion" onClick={() => abrir(v)} aria-label={`Editar ${v.key}`} title="Editar"><Icono nombre="edit" tamano={15} /></button>
              <button className="accion peligro" onClick={() => setPorBorrar(v)} aria-label={`Borrar ${v.key}`} title="Borrar"><Icono nombre="trash" tamano={15} /></button>
            </li>
          ))}
        </ul>
      )}

      {editando && (
        <Dialogo
          etiqueta={editando === "nueva" ? "nueva variable" : "editar variable"}
          titular={editando === "nueva" ? "Una variable *nueva*." : `Cambia *${editando.key}*.`}
          error={errorDialogo}
          enviando={guardando}
          alEnviar={() => void guardar()}
          alCerrar={() => setEditando(null)}
        >
          <div className="campo">
            <label htmlFor="v-clave">Nombre</label>
            <input id="v-clave" type="text" className="entrada-mono" value={clave} onChange={(e) => setClave(e.target.value)} readOnly={editando !== "nueva"} required autoFocus={editando === "nueva"} spellCheck={false} placeholder="DATABASE_URL" />
          </div>
          <div className="campo">
            <label htmlFor="v-valor">Valor</label>
            <textarea id="v-valor" className="entrada-mono" rows={3} value={valor} onChange={(e) => setValor(e.target.value)} spellCheck={false} autoFocus={editando !== "nueva"} />
          </div>
        </Dialogo>
      )}
      {porBorrar && (
        <Confirmar
          titulo={`¿Borrar ${porBorrar.key}?`}
          texto="La aplicación dejará de tener esa variable en su próximo despliegue."
          etiquetaConfirmar="Borrar"
          peligro
          trabajando={borrando}
          alConfirmar={() => void borrar()}
          alCancelar={() => setPorBorrar(null)}
        />
      )}
    </div>
  );
}

function Registros({ uuid }: { uuid: string }) {
  const [lineas, setLineas] = useState(200);
  const { datos, error, cargando, recargar } = useDatos<{ logs: string }>(`/coolify/applications/${uuid}/logs?lines=${lineas}`);

  return (
    <div style={{ display: "grid", gap: "0.8rem" }}>
      <div className="barra-herramientas">
        <Segmentado<"100" | "200" | "500">
          etiqueta="Líneas"
          valor={String(lineas) as "100" | "200" | "500"}
          opciones={[{ valor: "100", etiqueta: "100" }, { valor: "200", etiqueta: "200" }, { valor: "500", etiqueta: "500" }]}
          alCambiar={(v) => setLineas(Number(v))}
        />
        <button className="btn btn-pequeno" onClick={recargar} disabled={cargando}>
          <Icono nombre="refresh" tamano={14} />
          Actualizar
        </button>
      </div>
      {error && <Alerta tipo="error">{error}</Alerta>}
      {cargando && !datos ? <Esqueletos cantidad={1} /> : <Codigo titulo={`últimas ${lineas} líneas`} texto={datos?.logs || "Sin registros todavía."} />}
    </div>
  );
}
