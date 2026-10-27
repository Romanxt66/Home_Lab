import { useEffect, useMemo, useState } from "react";

import { api, ApiError } from "../api/client";
import type { EjecucionTarea, Tarea, TareaInput } from "../api/tipos";
import { Cabecera } from "../ui/Cabecera";
import { Confirmar } from "../ui/Confirmar";
import { Icono } from "../ui/Iconos";
import { Interruptor } from "../ui/Interruptor";
import { Resumen } from "../ui/Resumen";
import { hace } from "../ui/tiempo";
import { DialogoTarea } from "./tareas/DialogoTarea";
import { describirCron } from "./tareas/cron";

type Filtro = "todas" | "activas" | "pausa" | "fallan";
type Aviso = { tipo: "info" | "error"; texto: string };

/** Cuántas ejecuciones se enseñan antes de pulsar «ver más». */
const RECIENTES = 6;

/** Tareas programadas: lo que se ejecuta solo, cuándo, y cómo le fue la última vez. */
export function Tareas() {
  const [datos, setDatos] = useState<{ clave: number; tareas: Tarea[]; ejecuciones: EjecucionTarea[] } | null>(null);
  const [version, setVersion] = useState(0);
  const recargar = () => setVersion((v) => v + 1);

  const [error, setError] = useState<string | null>(null);
  const [aviso, setAviso] = useState<Aviso | null>(null);
  const [filtro, setFiltro] = useState<Filtro>("todas");
  const [verTodas, setVerTodas] = useState(false);

  // Diálogo: null = cerrado, "nueva" = alta, una Tarea = edición.
  const [editando, setEditando] = useState<Tarea | "nueva" | null>(null);
  const [guardando, setGuardando] = useState(false);
  const [errorDialogo, setErrorDialogo] = useState<string | null>(null);

  const [ejecutando, setEjecutando] = useState<string | null>(null);
  const [cambiando, setCambiando] = useState<string | null>(null);
  const [porEliminar, setPorEliminar] = useState<Tarea | null>(null);
  const [eliminando, setEliminando] = useState(false);

  useEffect(() => {
    let vigente = true;

    // Las dos cosas se piden a la vez: no dependen una de otra.
    Promise.all([
      api.get<Tarea[]>("/scheduler/jobs"),
      api.get<EjecucionTarea[]>("/scheduler/runs?limit=20"),
    ])
      .then(([tareas, ejecuciones]) => {
        if (!vigente) return;
        setDatos({ clave: version, tareas, ejecuciones });
        setError(null);
      })
      .catch((e) => {
        if (!vigente) return;
        setError(e instanceof Error ? e.message : "No se pudieron cargar las tareas.");
        setDatos((d) => ({ clave: version, tareas: d?.tareas ?? [], ejecuciones: d?.ejecuciones ?? [] }));
      });

    return () => {
      vigente = false;
    };
  }, [version]);

  const tareas = datos?.tareas ?? SIN_TAREAS;
  const ejecuciones = datos?.ejecuciones ?? SIN_EJECUCIONES;

  // La última ejecución de cada tarea. La lista viene de más nueva a más
  // vieja, así que la primera que se ve de cada una es la buena.
  const ultima = useMemo(() => {
    const mapa = new Map<string, EjecucionTarea>();
    for (const e of ejecuciones) if (!mapa.has(e.jobId)) mapa.set(e.jobId, e);
    return mapa;
  }, [ejecuciones]);

  const fallaLaUltima = (t: Tarea) => ultima.get(t.id)?.status === "error";

  const cuentas = {
    total: tareas.length,
    activas: tareas.filter((t) => t.enabled).length,
    pausa: tareas.filter((t) => !t.enabled).length,
    fallan: tareas.filter(fallaLaUltima).length,
  };

  const visibles = tareas.filter((t) => {
    if (filtro === "activas") return t.enabled;
    if (filtro === "pausa") return !t.enabled;
    if (filtro === "fallan") return fallaLaUltima(t);
    return true;
  });

  async function guardar(input: TareaInput) {
    setGuardando(true);
    setErrorDialogo(null);
    try {
      if (editando && editando !== "nueva") {
        await api.put<Tarea>(`/scheduler/jobs/${editando.id}`, input);
      } else {
        await api.post<Tarea>("/scheduler/jobs", input);
      }
      setEditando(null);
      recargar();
    } catch (e) {
      // La validación (cron, parámetros) viene del servidor ya escrita para
      // leerse: se muestra tal cual.
      setErrorDialogo(e instanceof ApiError ? e.message : "No se pudo guardar.");
    } finally {
      setGuardando(false);
    }
  }

  async function alternar(t: Tarea, activa: boolean) {
    setCambiando(t.id);
    setAviso(null);
    try {
      const nueva = await api.put<Tarea>(`/scheduler/jobs/${t.id}/enabled`, { enabled: activa });
      setDatos((d) => d && { ...d, tareas: d.tareas.map((x) => (x.id === t.id ? nueva : x)) });
    } catch (e) {
      setAviso({ tipo: "error", texto: e instanceof Error ? e.message : "No se pudo cambiar." });
    } finally {
      setCambiando(null);
    }
  }

  async function ejecutarAhora(t: Tarea) {
    setEjecutando(t.id);
    setAviso(null);
    try {
      const r = await api.post<{ output: string }>(`/scheduler/jobs/${t.id}/run`);
      setAviso({ tipo: "info", texto: `«${t.name}» se ejecutó: ${r.output || "sin salida"}` });
    } catch (e) {
      setAviso({ tipo: "error", texto: `«${t.name}» falló: ${e instanceof Error ? e.message : "error desconocido"}` });
    } finally {
      setEjecutando(null);
      // Salga bien o mal, queda anotado en el historial: se vuelve a leer.
      recargar();
    }
  }

  async function eliminar() {
    if (!porEliminar) return;
    setEliminando(true);
    try {
      await api.del(`/scheduler/jobs/${porEliminar.id}`);
      setPorEliminar(null);
      setEditando(null);
      recargar();
    } catch (e) {
      setError(e instanceof Error ? e.message : "No se pudo eliminar.");
      setPorEliminar(null);
    } finally {
      setEliminando(false);
    }
  }

  const cargando = datos === null;
  const recientes = verTodas ? ejecuciones : ejecuciones.slice(0, RECIENTES);
  const alternarFiltro = (f: Filtro) => setFiltro((actual) => (actual === f ? "todas" : f));

  return (
    <div className="pagina">
      <Cabecera
        etiqueta="tareas programadas"
        titular="Lo repetitivo, *en piloto automático*."
        entradilla={
          cuentas.fallan > 0
            ? `${cuentas.fallan === 1 ? "Una tarea falló" : `${cuentas.fallan} tareas fallaron`} la última vez. Conviene mirarlo.`
            : "Deja que el sistema envíe y compruebe cosas por ti, a la hora que digas."
        }
        acciones={
          <button
            className="btn btn-principal"
            onClick={() => {
              setErrorDialogo(null);
              setEditando("nueva");
            }}
          >
            <Icono nombre="plus" tamano={16} />
            Nueva tarea
          </button>
        }
      />

      {error && (
        <div className="alerta alerta-error subir" role="alert">
          <Icono nombre="alert" tamano={16} />
          <span>{error}</span>
        </div>
      )}

      {aviso && (
        <div
          className={`alerta ${aviso.tipo === "error" ? "alerta-error" : "alerta-info"} subir`}
          role={aviso.tipo === "error" ? "alert" : "status"}
        >
          <Icono nombre={aviso.tipo === "error" ? "alert" : "check"} tamano={16} />
          <span style={{ flex: 1 }}>{aviso.texto}</span>
          <button className="accion" onClick={() => setAviso(null)} aria-label="Descartar aviso" title="Descartar">
            <Icono nombre="close" tamano={14} />
          </button>
        </div>
      )}

      {cargando ? (
        <div className="rejilla" aria-busy="true" aria-label="Cargando tareas">
          {[0, 1].map((i) => (
            <div key={i} className="cristal esqueleto" />
          ))}
        </div>
      ) : tareas.length === 0 ? (
        <div className="cristal vacio subir">
          <span className="vacio-icono">
            <Icono nombre="clock" tamano={24} />
          </span>
          <h2>
            Aún no hay <em>nada programado</em>.
          </h2>
          <p>Crea una tarea para que envíe un correo o lea una página por ti, cada cierto tiempo.</p>
          <button className="btn btn-principal" onClick={() => setEditando("nueva")}>
            <Icono nombre="plus" tamano={16} />
            Crear la primera
          </button>
        </div>
      ) : (
        <>
          <div className="resumenes subir" style={{ "--i": 1 } as React.CSSProperties}>
            <Resumen etiqueta="tareas" valor={cuentas.total} icono="list" seleccionado={filtro === "todas"} alPulsar={() => setFiltro("todas")} />
            <Resumen etiqueta="activas" valor={cuentas.activas} icono="check" tono="acento" seleccionado={filtro === "activas"} alPulsar={() => alternarFiltro("activas")} />
            <Resumen etiqueta="en pausa" valor={cuentas.pausa} icono="pause" tono={cuentas.pausa > 0 ? "aviso" : "neutro"} seleccionado={filtro === "pausa"} alPulsar={() => alternarFiltro("pausa")} />
            <Resumen etiqueta="fallan" valor={cuentas.fallan} icono="alert" tono={cuentas.fallan > 0 ? "peligro" : "neutro"} seleccionado={filtro === "fallan"} alPulsar={() => alternarFiltro("fallan")} />
          </div>

          {visibles.length === 0 ? (
            <div className="cristal vacio subir">
              <p>No hay tareas en ese estado.</p>
              <button className="btn btn-pequeno" onClick={() => setFiltro("todas")}>
                Ver todas
              </button>
            </div>
          ) : (
            <div className="rejilla">
              {visibles.map((t, i) => (
                <TarjetaTarea
                  key={t.id}
                  tarea={t}
                  indice={i + 2}
                  ultima={ultima.get(t.id)}
                  ejecutando={ejecutando === t.id}
                  cambiando={cambiando === t.id}
                  alEjecutar={() => void ejecutarAhora(t)}
                  alAlternar={(activa) => void alternar(t, activa)}
                  alEditar={() => {
                    setErrorDialogo(null);
                    setEditando(t);
                  }}
                  alEliminar={() => setPorEliminar(t)}
                />
              ))}
            </div>
          )}

          <section className="historial subir" style={{ "--i": 3 } as React.CSSProperties} aria-label="Últimas ejecuciones">
            <div className="historial-cab">
              <span className="etiqueta">últimas ejecuciones</span>
              {ejecuciones.length > RECIENTES && (
                <button className="btn btn-pequeno btn-fantasma" onClick={() => setVerTodas((v) => !v)}>
                  {verTodas ? "Ver menos" : `Ver las ${ejecuciones.length}`}
                </button>
              )}
            </div>

            {recientes.length === 0 ? (
              <p className="historial-vacio">Todavía no se ha ejecutado ninguna tarea.</p>
            ) : (
              <ul className="historial-lista cristal">
                {recientes.map((e) => (
                  <li key={e.id} className={`ejecucion ${e.status === "error" ? "con-error" : ""}`}>
                    <span className="ejecucion-punto" aria-label={e.status === "error" ? "falló" : "bien"} />
                    <span className="ejecucion-nombre">{e.jobName}</span>
                    <span className="ejecucion-salida" title={e.output ?? ""}>
                      {e.output || "sin salida"}
                    </span>
                    <span className="ejecucion-cuando">{hace(e.startedAt)}</span>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </>
      )}

      {editando && (
        <DialogoTarea
          // Se monta de nuevo en cada apertura, así el formulario siempre
          // arranca con los valores de esa tarea y no con los de la anterior.
          key={editando === "nueva" ? "nueva" : editando.id}
          tarea={editando === "nueva" ? null : editando}
          guardando={guardando}
          error={errorDialogo}
          alGuardar={(input) => void guardar(input)}
          alEliminar={editando !== "nueva" ? () => setPorEliminar(editando) : undefined}
          alCerrar={() => {
            setEditando(null);
            setErrorDialogo(null);
          }}
        />
      )}

      {porEliminar && (
        <Confirmar
          titulo={`¿Eliminar «${porEliminar.name}»?`}
          texto="Dejará de ejecutarse y se borrará también su historial de ejecuciones."
          etiquetaConfirmar="Eliminar"
          peligro
          trabajando={eliminando}
          alConfirmar={() => void eliminar()}
          alCancelar={() => setPorEliminar(null)}
        />
      )}
    </div>
  );
}

// Listas vacías compartidas: crear una nueva en cada render haría recalcular todo lo memorizado.
const SIN_TAREAS: Tarea[] = [];
const SIN_EJECUCIONES: EjecucionTarea[] = [];

interface TarjetaProps {
  tarea: Tarea;
  indice: number;
  ultima?: EjecucionTarea;
  ejecutando: boolean;
  cambiando: boolean;
  alEjecutar(): void;
  alAlternar(activa: boolean): void;
  alEditar(): void;
  alEliminar(): void;
}

function TarjetaTarea({ tarea: t, indice, ultima, ejecutando, cambiando, alEjecutar, alAlternar, alEditar, alEliminar }: TarjetaProps) {
  const falla = ultima?.status === "error";
  const tono = falla ? "peligro" : t.enabled ? "acento" : "aviso";
  const estado = falla ? "Falló la última vez" : t.enabled ? "Activa" : "En pausa";
  const frase = describirCron(t.cron);

  return (
    <article
      className={`tarjeta cristal subir tono-${tono}${t.enabled ? "" : " atenuada"}`}
      style={{ "--i": indice } as React.CSSProperties}
      aria-label={`${t.name}: ${estado}`}
    >
      <div className="tarjeta-cab">
        <span className="tarjeta-avatar">
          <Icono nombre={t.action === "email" ? "mail" : "globe"} tamano={18} />
        </span>
        <div className="tarjeta-titulo">
          <span className="tarjeta-nombre" title={t.name}>
            {t.name}
          </span>
          <span className="tarjeta-sub" title={t.cron}>
            {frase}
          </span>
        </div>
        <div className="tarjeta-iconos">
          <button className="accion" onClick={alEditar} title="Editar" aria-label={`Editar ${t.name}`}>
            <Icono nombre="edit" tamano={16} />
          </button>
          <button className="accion peligro" onClick={alEliminar} title="Eliminar" aria-label={`Eliminar ${t.name}`}>
            <Icono nombre="trash" tamano={16} />
          </button>
        </div>
      </div>

      <div className="fichas">
        <span className="ficha ficha-estado">
          <span className="punto" />
          {estado}
        </span>
        <span className="ficha">{t.action === "email" ? "envía un correo" : "lee una web"}</span>
        <span className="ficha">{ultima ? `última: ${hace(ultima.startedAt)}` : "sin ejecutar"}</span>
      </div>

      {ultima?.output && (
        <p className={`salida${falla ? " salida-error" : ""}`} title={ultima.output}>
          {ultima.output}
        </p>
      )}

      <div className="tarjeta-pie tarjeta-pie-partido">
        <button className={`btn btn-pequeno${ejecutando ? " girando" : ""}`} onClick={alEjecutar} disabled={ejecutando}>
          <Icono nombre={ejecutando ? "refresh" : "play"} tamano={13} />
          {ejecutando ? "Ejecutando…" : "Ejecutar ahora"}
        </button>
        <Interruptor activo={t.enabled} etiqueta={`Ejecutar «${t.name}» automáticamente`} deshabilitado={cambiando} alCambiar={alAlternar} />
      </div>
    </article>
  );
}
