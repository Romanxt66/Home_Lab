import { useCallback, useEffect, useState } from "react";

import { api, ApiError } from "../api/client";
import { ETIQUETAS_ESTADO } from "../api/tipos";
import type { Monitor, MonitorInput } from "../api/tipos";
import { DialogoMonitor } from "../components/DialogoMonitor";

/** Monitor de disponibilidad: la lista de URLs vigiladas y su estado. */
export function Uptime() {
  const [monitores, setMonitores] = useState<Monitor[]>([]);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Diálogo: null = cerrado, "nuevo" = alta, un Monitor = edición.
  const [editando, setEditando] = useState<Monitor | "nuevo" | null>(null);
  const [guardando, setGuardando] = useState(false);
  const [errorDialogo, setErrorDialogo] = useState<string | null>(null);

  // Qué monitor se está comprobando ahora mismo, para deshabilitar su botón.
  const [comprobando, setComprobando] = useState<string | null>(null);

  const cargar = useCallback(async () => {
    try {
      setMonitores(await api.get<Monitor[]>("/uptime/monitors"));
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "No se pudieron cargar los monitores.");
    } finally {
      setCargando(false);
    }
  }, []);

  useEffect(() => {
    void cargar();
  }, [cargar]);

  async function guardar(input: MonitorInput) {
    setGuardando(true);
    setErrorDialogo(null);
    try {
      if (editando && editando !== "nuevo") {
        await api.put<Monitor>(`/uptime/monitors/${editando.id}`, input);
      } else {
        await api.post<Monitor>("/uptime/monitors", input);
      }
      setEditando(null);
      await cargar();
    } catch (e) {
      // El mensaje de validación viene del dominio en Go, así que se muestra
      // tal cual: ya está escrito para que lo lea una persona.
      setErrorDialogo(e instanceof ApiError ? e.message : "No se pudo guardar.");
    } finally {
      setGuardando(false);
    }
  }

  async function comprobarAhora(id: string) {
    setComprobando(id);
    try {
      const actualizado = await api.post<Monitor>(`/uptime/monitors/${id}/check`);
      // Se reemplaza solo esa tarjeta en vez de recargar toda la lista.
      setMonitores((lista) => lista.map((m) => (m.id === id ? actualizado : m)));
    } catch (e) {
      setError(e instanceof Error ? e.message : "No se pudo comprobar.");
    } finally {
      setComprobando(null);
    }
  }

  async function eliminar(monitor: Monitor) {
    const seguro = window.confirm(
      `Se va a eliminar «${monitor.name}» y todo su histórico de comprobaciones. ` +
        "Esta acción no se puede deshacer. ¿Continuar?",
    );
    if (!seguro) return;

    try {
      await api.del(`/uptime/monitors/${monitor.id}`);
      setMonitores((lista) => lista.filter((m) => m.id !== monitor.id));
    } catch (e) {
      setError(e instanceof Error ? e.message : "No se pudo eliminar.");
    }
  }

  const caidos = monitores.filter((m) => m.lastStatus === "down").length;

  return (
    <>
      <header className="barra">
        <div>
          <h1>Monitor de uptime</h1>
          <span className="barra-sub">
            {monitores.length} monitor{monitores.length === 1 ? "" : "es"}
            {caidos > 0 && ` · ${caidos} caído${caidos === 1 ? "" : "s"}`}
          </span>
        </div>
        <button className="btn btn-principal" onClick={() => setEditando("nuevo")}>
          + Nuevo monitor
        </button>
      </header>

      {error && <div className="alerta alerta-error">{error}</div>}

      {cargando ? (
        <div className="panel">
          <div className="vacio">Cargando…</div>
        </div>
      ) : monitores.length === 0 ? (
        <div className="panel">
          <div className="vacio">
            <h3>Todavía no vigilas ninguna URL</h3>
            <p>Añade la primera y el sistema comprobará su estado cada pocos minutos.</p>
          </div>
        </div>
      ) : (
        <div className="rejilla">
          {monitores.map((m) => (
            <article key={m.id} className="monitor">
              <div className="monitor-cab">
                <div className="monitor-titulo">
                  <div className="monitor-nombre">{m.name}</div>
                  <span className="monitor-url" title={m.url}>
                    {m.url}
                  </span>
                </div>
                <span className={`estado estado-${m.lastStatus}`}>
                  {ETIQUETAS_ESTADO[m.lastStatus]}
                </span>
              </div>

              {m.lastError && <div className="monitor-error">{m.lastError}</div>}

              {!m.active && (
                <div className="monitor-pausado">
                  En pausa: no se comprueba automáticamente.
                </div>
              )}

              <div className="monitor-datos">
                <div>
                  Respuesta
                  <b>{m.lastResponseMs != null ? `${m.lastResponseMs} ms` : "—"}</b>
                </div>
                <div>
                  Código
                  <b>{m.lastStatusCode ?? "—"}</b>
                </div>
                <div>
                  Última revisión
                  <b>{formatearFecha(m.lastCheckedAt)}</b>
                </div>
                {m.consecutiveFailures > 0 && (
                  <div>
                    Fallos seguidos
                    <b>{m.consecutiveFailures}</b>
                  </div>
                )}
              </div>

              <div className="monitor-acciones">
                <button
                  className="btn btn-pequeno"
                  onClick={() => void comprobarAhora(m.id)}
                  disabled={comprobando === m.id}
                >
                  {comprobando === m.id ? "Comprobando…" : "Comprobar ahora"}
                </button>
                <button className="btn btn-pequeno" onClick={() => setEditando(m)}>
                  Editar
                </button>
                <button
                  className="btn btn-pequeno btn-peligro"
                  onClick={() => void eliminar(m)}
                >
                  Eliminar
                </button>
              </div>
            </article>
          ))}
        </div>
      )}

      {editando && (
        <DialogoMonitor
          monitor={editando === "nuevo" ? null : editando}
          guardando={guardando}
          error={errorDialogo}
          onGuardar={(input) => void guardar(input)}
          onCerrar={() => {
            setEditando(null);
            setErrorDialogo(null);
          }}
        />
      )}
    </>
  );
}

/** Fecha corta y legible; "—" si nunca se ha comprobado. */
function formatearFecha(iso: string | null): string {
  if (!iso) return "Nunca";
  return new Date(iso).toLocaleString("es-CO", {
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}
