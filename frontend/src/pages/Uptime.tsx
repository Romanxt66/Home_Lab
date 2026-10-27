import { useCallback, useEffect, useMemo, useState } from "react";

import { api, ApiError } from "../api/client";
import { ETIQUETAS_ESTADO } from "../api/tipos";
import type { Monitor, MonitorInput } from "../api/tipos";
import { DialogoMonitor } from "../components/DialogoMonitor";
import { Cabecera } from "../ui/Cabecera";
import { Confirmar } from "../ui/Confirmar";
import { Icono } from "../ui/Iconos";
import { Cifra } from "../ui/Cifra";
import { Resumen } from "../ui/Resumen";

type Filtro = "todos" | "up" | "down" | "pausa";

/** Qué «tono» tiene un monitor: decide color de borde, icono y ficha. */
function tonoDe(m: Monitor): "up" | "down" | "pausa" | "unknown" {
  if (!m.active) return "pausa";
  return m.lastStatus;
}

function coincide(m: Monitor, filtro: Filtro): boolean {
  switch (filtro) {
    case "up":
      return tonoDe(m) === "up";
    case "down":
      return tonoDe(m) === "down";
    case "pausa":
      return tonoDe(m) === "pausa";
    default:
      return true;
  }
}

/** Monitor de disponibilidad: la lista de URLs vigiladas y su estado. */
export function Uptime() {
  const [monitores, setMonitores] = useState<Monitor[]>([]);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filtro, setFiltro] = useState<Filtro>("todos");

  // Diálogo: null = cerrado, "nuevo" = alta, un Monitor = edición.
  const [editando, setEditando] = useState<Monitor | "nuevo" | null>(null);
  const [guardando, setGuardando] = useState(false);
  const [errorDialogo, setErrorDialogo] = useState<string | null>(null);

  // Qué monitor se está comprobando ahora mismo, para deshabilitar su botón.
  const [comprobando, setComprobando] = useState<string | null>(null);

  // Monitor que se va a eliminar, a la espera de que se confirme.
  const [porEliminar, setPorEliminar] = useState<Monitor | null>(null);
  const [eliminando, setEliminando] = useState(false);

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

  async function eliminar() {
    if (!porEliminar) return;
    setEliminando(true);
    try {
      await api.del(`/uptime/monitors/${porEliminar.id}`);
      setMonitores((lista) => lista.filter((m) => m.id !== porEliminar.id));
      setPorEliminar(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "No se pudo eliminar.");
      setPorEliminar(null);
    } finally {
      setEliminando(false);
    }
  }

  const cuentas = useMemo(
    () => ({
      total: monitores.length,
      up: monitores.filter((m) => tonoDe(m) === "up").length,
      down: monitores.filter((m) => tonoDe(m) === "down").length,
      pausa: monitores.filter((m) => tonoDe(m) === "pausa").length,
    }),
    [monitores],
  );

  const visibles = monitores.filter((m) => coincide(m, filtro));

  /** Pulsar el filtro activo lo quita: vuelve a «todos». */
  const alternar = (f: Filtro) => setFiltro((actual) => (actual === f ? "todos" : f));

  return (
    <div className="pagina">
      <Cabecera
        etiqueta="monitor de uptime"
        titular="Tus servicios, *a la vista*."
        entradilla={
          cuentas.down > 0 ? (
            <>
              {cuentas.down === 1 ? "Hay un servicio caído" : `Hay ${cuentas.down} servicios caídos`}.
              Lo primero es mirarlo.
            </>
          ) : cuentas.total > 0 ? (
            "Todo responde. Te avisaremos en cuanto algo deje de hacerlo."
          ) : (
            "Vigila cualquier URL y recibe un aviso cuando se caiga."
          )
        }
        acciones={
          <button className="btn btn-principal" onClick={() => setEditando("nuevo")}>
            <Icono nombre="plus" tamano={16} />
            Nuevo monitor
          </button>
        }
      />

      {error && (
        <div className="alerta alerta-error subir" role="alert">
          <Icono nombre="alert" tamano={16} />
          <span>{error}</span>
        </div>
      )}

      {cargando ? (
        <div className="rejilla" aria-busy="true" aria-label="Cargando monitores">
          {[0, 1, 2].map((i) => (
            <div key={i} className="cristal esqueleto" />
          ))}
        </div>
      ) : monitores.length === 0 ? (
        <div className="cristal vacio subir">
          <span className="vacio-icono">
            <Icono nombre="pulse" tamano={24} />
          </span>
          <h2>
            Aún no vigilas <em>nada</em>.
          </h2>
          <p>Añade la primera URL y el sistema comprobará su estado cada pocos minutos.</p>
          <button className="btn btn-principal" onClick={() => setEditando("nuevo")}>
            <Icono nombre="plus" tamano={16} />
            Añadir el primero
          </button>
        </div>
      ) : (
        <>
          <div className="resumenes subir" style={{ "--i": 1 } as React.CSSProperties}>
            <Resumen
              etiqueta="monitores"
              valor={cuentas.total}
              icono="list"
              seleccionado={filtro === "todos"}
              alPulsar={() => setFiltro("todos")}
            />
            <Resumen
              etiqueta="en línea"
              valor={cuentas.up}
              icono="check"
              tono="acento"
              seleccionado={filtro === "up"}
              alPulsar={() => alternar("up")}
            />
            <Resumen
              etiqueta="caídos"
              valor={cuentas.down}
              icono="alert"
              tono={cuentas.down > 0 ? "peligro" : "neutro"}
              seleccionado={filtro === "down"}
              alPulsar={() => alternar("down")}
            />
            <Resumen
              etiqueta="en pausa"
              valor={cuentas.pausa}
              icono="pause"
              tono={cuentas.pausa > 0 ? "aviso" : "neutro"}
              seleccionado={filtro === "pausa"}
              alPulsar={() => alternar("pausa")}
            />
          </div>

          {visibles.length === 0 ? (
            <div className="cristal vacio subir">
              <p>No hay monitores con ese estado ahora mismo.</p>
              <button className="btn btn-pequeno" onClick={() => setFiltro("todos")}>
                Ver todos
              </button>
            </div>
          ) : (
            <div className="rejilla">
              {visibles.map((m, i) => (
                <TarjetaMonitor
                  key={m.id}
                  monitor={m}
                  indice={i + 2}
                  comprobando={comprobando === m.id}
                  alComprobar={() => void comprobarAhora(m.id)}
                  alEditar={() => setEditando(m)}
                  alEliminar={() => setPorEliminar(m)}
                />
              ))}
            </div>
          )}
        </>
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

      {porEliminar && (
        <Confirmar
          titulo={`¿Eliminar «${porEliminar.name}»?`}
          texto="Se borrará el monitor y todo su histórico de comprobaciones."
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

interface TarjetaProps {
  monitor: Monitor;
  indice: number;
  comprobando: boolean;
  alComprobar(): void;
  alEditar(): void;
  alEliminar(): void;
}

function TarjetaMonitor({ monitor: m, indice, comprobando, alComprobar, alEditar, alEliminar }: TarjetaProps) {
  const tono = tonoDe(m);
  const icono = tono === "down" ? "alert" : tono === "pausa" ? "pause" : tono === "up" ? "check" : "clock";
  const etiquetaTono = tono === "pausa" ? "En pausa" : ETIQUETAS_ESTADO[m.lastStatus];

  // Cuánto del tiempo máximo se ha gastado: si anda cerca, es una señal
  // temprana aunque el monitor siga «en línea».
  const proporcion = m.lastResponseMs != null ? Math.min(m.lastResponseMs / m.timeoutMs, 1) : 0;
  const lenta = proporcion > 0.7;

  return (
    <article
      className={`monitor cristal subir tono-${tono}${tono === "pausa" ? " atenuado" : ""}`}
      style={{ "--i": indice } as React.CSSProperties}
      aria-label={`${m.name}: ${etiquetaTono}`}
    >
      <div className="monitor-cab">
        <span className="monitor-avatar">
          <Icono nombre={icono} tamano={18} />
        </span>
        <div className="monitor-titulo">
          <span className="monitor-nombre" title={m.name}>
            {m.name}
          </span>
          <span className="monitor-url" title={m.url}>
            {m.url}
          </span>
        </div>
        <div className="monitor-iconos">
          <button className="accion" onClick={alEditar} title="Editar" aria-label={`Editar ${m.name}`}>
            <Icono nombre="edit" tamano={16} />
          </button>
          <button className="accion peligro" onClick={alEliminar} title="Eliminar" aria-label={`Eliminar ${m.name}`}>
            <Icono nombre="trash" tamano={16} />
          </button>
        </div>
      </div>

      <div className="monitor-medida">
        <span className="etiqueta">respuesta</span>
        <div className="monitor-medida-fila">
          {m.lastResponseMs != null ? (
            <span className="monitor-valor">
              <Cifra valor={m.lastResponseMs} />
              <small>ms</small>
            </span>
          ) : (
            <span className="monitor-valor sin-dato">—</span>
          )}
          <span className="monitor-lateral">
            {m.lastStatusCode != null ? `http ${m.lastStatusCode}` : "sin código"}
          </span>
        </div>
        <div className={`barra-tiempo${lenta ? " lenta" : ""}`} aria-hidden="true">
          <i style={{ width: `${Math.max(proporcion * 100, m.lastResponseMs != null ? 3 : 0)}%` }} />
        </div>
      </div>

      <div className="fichas">
        <span className="ficha ficha-estado">
          <span className="punto" />
          {etiquetaTono}
        </span>
        <span className="ficha">
          <Icono nombre="clock" tamano={11} />
          cada {formatearIntervalo(m.intervalSeconds)}
        </span>
        <span className="ficha">{formatearRelativo(m.lastCheckedAt)}</span>
        {m.consecutiveFailures > 0 && (
          <span className="ficha" style={{ color: "var(--peligro)" }}>
            {m.consecutiveFailures} {m.consecutiveFailures === 1 ? "fallo" : "fallos"} seguidos
          </span>
        )}
      </div>

      {m.lastError && <div className="monitor-error">{m.lastError}</div>}

      {!m.active && (
        <div className="monitor-aviso">
          <Icono nombre="pause" tamano={14} />
          En pausa: no se comprueba automáticamente.
        </div>
      )}

      <div className="monitor-pie">
        <button
          className={`btn btn-pequeno${comprobando ? " girando" : ""}`}
          onClick={alComprobar}
          disabled={comprobando}
        >
          <Icono nombre="refresh" tamano={14} />
          {comprobando ? "Comprobando…" : "Comprobar ahora"}
        </button>
      </div>
    </article>
  );
}

/** 300 → «5 min», 45 → «45 s», 7200 → «2 h». */
function formatearIntervalo(segundos: number): string {
  if (segundos < 60) return `${segundos} s`;
  if (segundos < 3600) return `${Math.round(segundos / 60)} min`;
  return `${Math.round(segundos / 3600)} h`;
}

/** «hace 3 min»; lo exacto va en el atributo, no hace falta ruido. */
function formatearRelativo(iso: string | null): string {
  if (!iso) return "sin comprobar";

  const segundos = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 1000));
  if (segundos < 45) return "hace un momento";
  if (segundos < 3600) return `hace ${Math.round(segundos / 60)} min`;
  if (segundos < 86400) return `hace ${Math.round(segundos / 3600)} h`;
  return new Date(iso).toLocaleDateString("es-CO", { day: "2-digit", month: "short" });
}
