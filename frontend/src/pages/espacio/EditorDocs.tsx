import { useEffect, useMemo, useState } from "react";

import { api, ApiError, urlApi } from "../../api/client";
import type { DetalleEspacio, ElementoEspacio } from "../../api/tipos";
import { Alerta } from "../../ui/Alerta";
import { Confirmar } from "../../ui/Confirmar";
import { Dialogo } from "../../ui/Dialogo";
import { Icono } from "../../ui/Iconos";
import { Interruptor } from "../../ui/Interruptor";
import { Esqueletos, Vacio } from "../../ui/Vacio";
import { useDatos } from "../../ui/useDatos";
import { hace } from "../../ui/tiempo";

interface Props {
  docs: ElementoEspacio[];
  cargando: boolean;
  esAdmin: boolean;
  alCambiar(): void;
}

const PLANTILLA = "# Título\n\nEscribe aquí las instrucciones.\n";

/** Documentos .md: lista agrupada por carpetas a la izquierda y editor a la derecha. */
export function EditorDocs({ docs, cargando, esAdmin, alCambiar }: Props) {
  const [id, setId] = useState<string | null>(null);
  const elegido = docs.find((d) => d.id === id) ?? null;

  const [texto, setTexto] = useState("");
  const [original, setOriginal] = useState("");
  const [cargado, setCargado] = useState<string | null>(null); // a qué doc pertenece `texto`
  const [mensaje, setMensaje] = useState("");
  const [guardando, setGuardando] = useState(false);
  const [aviso, setAviso] = useState<{ tipo: "info" | "error"; texto: string } | null>(null);
  const [pendiente, setPendiente] = useState<string | null | undefined>(undefined); // cambio de doc con cambios sin guardar
  const [nuevo, setNuevo] = useState(false);
  const [porBorrar, setPorBorrar] = useState(false);
  const [verHistorial, setVerHistorial] = useState(false);

  const detalle = useDatos<DetalleEspacio>(elegido && verHistorial ? `/workspace/items/${elegido.id}` : null);
  const sucio = cargado === elegido?.id && texto !== original;

  // Trae el texto al elegir un documento, o cuando otra persona lo ha cambiado (nueva versión).
  useEffect(() => {
    if (!elegido) return;
    let vigente = true;
    api
      .get<{ content: string }>(`/workspace/items/${elegido.id}/file?${new URLSearchParams({ path: elegido.name })}`)
      .then((r) => {
        if (!vigente) return;
        setTexto(r.content);
        setOriginal(r.content);
        setCargado(elegido.id);
      })
      .catch((e) => vigente && setAviso({ tipo: "error", texto: e instanceof Error ? e.message : "No se pudo abrir." }));
    return () => {
      vigente = false;
    };
  }, [elegido?.id, elegido?.version]); // eslint-disable-line react-hooks/exhaustive-deps

  function elegir(destino: string | null) {
    if (sucio && destino !== id) setPendiente(destino);
    else {
      setId(destino);
      setAviso(null);
    }
  }

  async function guardar() {
    if (!elegido) return;
    setGuardando(true);
    setAviso(null);
    try {
      await api.post("/workspace/docs", { path: elegido.name, content: texto, message: mensaje, visibility: elegido.visibility });
      setOriginal(texto);
      setMensaje("");
      setAviso({ tipo: "info", texto: "Guardado." });
      alCambiar();
    } catch (e) {
      setAviso({ tipo: "error", texto: e instanceof ApiError ? e.message : "No se pudo guardar." });
    } finally {
      setGuardando(false);
    }
  }

  async function cambiarVisibilidad(compartir: boolean) {
    if (!elegido) return;
    try {
      await api.put(`/workspace/items/${elegido.id}/visibility`, { visibility: compartir ? "shared" : "private" });
      alCambiar();
    } catch (e) {
      setAviso({ tipo: "error", texto: e instanceof Error ? e.message : "No se pudo cambiar." });
    }
  }

  async function borrar() {
    if (!elegido) return;
    try {
      await api.del(`/workspace/items/${elegido.id}`);
      setId(null);
      setCargado(null);
      alCambiar();
    } catch (e) {
      setAviso({ tipo: "error", texto: e instanceof Error ? e.message : "No se pudo eliminar." });
    } finally {
      setPorBorrar(false);
    }
  }

  async function verVersion(numero: number) {
    if (!elegido) return;
    try {
      const r = await api.get<{ content: string }>(
        `/workspace/items/${elegido.id}/file?${new URLSearchParams({ path: elegido.name, version: String(numero) })}`,
      );
      setTexto(r.content);
      setAviso({ tipo: "info", texto: `Se cargó la versión ${numero} en el editor. Guarda para convertirla en la actual.` });
    } catch (e) {
      setAviso({ tipo: "error", texto: e instanceof Error ? e.message : "No se pudo abrir esa versión." });
    }
  }

  // Árbol: carpeta → documentos.
  const grupos = useMemo(() => {
    const m = new Map<string, ElementoEspacio[]>();
    for (const d of docs) {
      const i = d.name.lastIndexOf("/");
      const carpeta = i < 0 ? "" : d.name.slice(0, i);
      m.set(carpeta, [...(m.get(carpeta) ?? []), d]);
    }
    return [...m.entries()].sort(([a], [b]) => a.localeCompare(b));
  }, [docs]);

  const soyDueno = elegido?.mine ?? false;

  return (
    <div className="editor-docs">
      <aside className="cr-glass editor-docs-lista" aria-label="Documentos">
        <button className="cr-btn cr-btn-primary cr-btn-sm" onClick={() => setNuevo(true)}>
          <Icono nombre="plus" tamano={14} />
          Nuevo documento
        </button>
        {cargando && docs.length === 0 ? (
          <Esqueletos cantidad={2} />
        ) : docs.length === 0 ? (
          <p className="texto-apagado">Aún no tienes documentos. Crea el primero, por ejemplo un CLAUDE.md.</p>
        ) : (
          grupos.map(([carpeta, lista]) => (
            <div key={carpeta} className="editor-docs-grupo">
              {carpeta && (
                <span className="etiqueta editor-docs-carpeta">
                  <Icono nombre="folder" tamano={12} />
                  {carpeta}
                </span>
              )}
              <ul>
                {lista.map((d) => (
                  <li key={d.id}>
                    <button className={d.id === id ? "activa" : ""} onClick={() => elegir(d.id)} title={d.name}>
                      <Icono nombre="file" tamano={13} />
                      <span>{d.name.slice(d.name.lastIndexOf("/") + 1)}</span>
                      {d.visibility === "shared" && <span className="lista-tablas-n">{d.mine ? "compartido" : d.ownerName}</span>}
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          ))
        )}
      </aside>

      <section className="editor-docs-panel" aria-label="Editor">
        {!elegido ? (
          <Vacio icono="file" titular="Elige un *documento*.">Selecciona uno de la lista o crea uno nuevo para editarlo aquí.</Vacio>
        ) : (
          <>
            <div className="barra-herramientas">
              <span className="ficha" title={elegido.name}><Icono nombre="file" tamano={12} />{elegido.name}</span>
              <span className="ficha">v{elegido.version}</span>
              {!soyDueno && <span className="ficha">de {elegido.ownerName} · solo lectura</span>}
              <span style={{ flex: 1 }} />
              {soyDueno && (
                <label className="casilla" style={{ minHeight: 0 }}>
                  Compartido
                  <Interruptor activo={elegido.visibility === "shared"} etiqueta="Compartir este documento" alCambiar={(v) => void cambiarVisibilidad(v)} />
                </label>
              )}
              <a className="cr-btn cr-btn-glass cr-btn-sm" href={urlApi(`/workspace/items/${elegido.id}/download`)} download>
                <Icono nombre="download" tamano={13} />
                Bajar
              </a>
              {(soyDueno || esAdmin) && (
                <button className="cr-btn cr-btn-ghost cr-btn-danger cr-btn-sm" onClick={() => setPorBorrar(true)}>
                  <Icono nombre="trash" tamano={13} />
                  Eliminar
                </button>
              )}
            </div>

            {aviso && <Alerta tipo={aviso.tipo} alCerrar={() => setAviso(null)}>{aviso.texto}</Alerta>}

            <textarea
              className="entrada-mono editor-docs-texto"
              value={texto}
              onChange={(e) => setTexto(e.target.value)}
              readOnly={!soyDueno}
              spellCheck={false}
              aria-label={`Contenido de ${elegido.name}`}
            />

            {soyDueno && (
              <div className="editor-docs-pie">
                <input
                  type="text"
                  value={mensaje}
                  onChange={(e) => setMensaje(e.target.value)}
                  maxLength={200}
                  placeholder="Qué cambió (opcional)"
                  aria-label="Qué cambió"
                />
                <button className="cr-btn cr-btn-primary" onClick={() => void guardar()} disabled={guardando || !sucio}>
                  <Icono nombre="check" tamano={15} />
                  {guardando ? "Guardando…" : sucio ? "Guardar versión" : "Sin cambios"}
                </button>
              </div>
            )}

            <button className="cr-btn cr-btn-ghost cr-btn-sm" style={{ justifySelf: "start" }} onClick={() => setVerHistorial((v) => !v)} aria-expanded={verHistorial}>
              <Icono nombre="clock" tamano={13} />
              {verHistorial ? "Ocultar historial" : "Historial"}
            </button>
            {verHistorial && (
              <ul className="historial-versiones" aria-label="Versiones">
                {detalle.cargando && !detalle.datos && <li className="texto-apagado">Cargando…</li>}
                {detalle.datos?.versions.map((v) => (
                  <li key={v.number}>
                    <span className="ficha">v{v.number}</span>
                    <span className="historial-texto">
                      <span className="label">{v.message || "Sin mensaje"}</span>
                      <span className="caption cr-muted">{v.authorName} · {hace(v.createdAt)}</span>
                    </span>
                    <button className="cr-btn cr-btn-ghost cr-btn-sm" onClick={() => void verVersion(v.number)}>
                      Cargar
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </>
        )}
      </section>

      {nuevo && (
        <DialogoNuevoDoc
          alCerrar={() => setNuevo(false)}
          alCrear={(d) => {
            setNuevo(false);
            alCambiar();
            setId(d.id);
          }}
        />
      )}
      {porBorrar && elegido && (
        <Confirmar
          titulo={`¿Eliminar «${elegido.name}»?`}
          texto="Se borra el documento con todo su historial. No se puede deshacer."
          etiquetaConfirmar="Eliminar"
          peligro
          alConfirmar={() => void borrar()}
          alCancelar={() => setPorBorrar(false)}
        />
      )}
      {pendiente !== undefined && (
        <Confirmar
          titulo="Hay cambios sin guardar"
          texto="Si cambias de documento se pierden. ¿Descartarlos?"
          etiquetaConfirmar="Descartar"
          peligro
          alConfirmar={() => { setId(pendiente); setAviso(null); setPendiente(undefined); }}
          alCancelar={() => setPendiente(undefined)}
        />
      )}
    </div>
  );
}

function DialogoNuevoDoc({ alCerrar, alCrear }: { alCerrar(): void; alCrear(d: ElementoEspacio): void }) {
  const [ruta, setRuta] = useState("");
  const [compartir, setCompartir] = useState(false);
  const [enviando, setEnviando] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function crear() {
    setEnviando(true);
    setError(null);
    try {
      const limpia = ruta.trim().replace(/^\/+/, "");
      const final = limpia.toLowerCase().endsWith(".md") ? limpia : `${limpia}.md`;
      alCrear(await api.post<ElementoEspacio>("/workspace/docs", { path: final, content: PLANTILLA, visibility: compartir ? "shared" : "private" }));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "No se pudo crear.");
    } finally {
      setEnviando(false);
    }
  }

  return (
    <Dialogo etiqueta="nuevo documento" titular="Un documento *nuevo*." etiquetaEnviar="Crear" error={error} enviando={enviando} alEnviar={() => void crear()} alCerrar={alCerrar}>
      <div className="campo">
        <label htmlFor="nd-ruta">Ruta</label>
        <input id="nd-ruta" type="text" className="entrada-mono" value={ruta} onChange={(e) => setRuta(e.target.value)} placeholder="proyectos/homelab/CLAUDE.md" required autoFocus spellCheck={false} />
        <span className="campo-ayuda">Las carpetas se separan con «/». Si no acaba en .md, se lo añado.</span>
      </div>
      <label className="casilla">
        <input type="checkbox" checked={compartir} onChange={(e) => setCompartir(e.target.checked)} />
        Compartirlo con el resto de personas
      </label>
    </Dialogo>
  );
}
