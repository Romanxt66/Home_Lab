import { useEffect, useState } from "react";
import type { FormEvent } from "react";

import type { HttpMethod, Monitor, MonitorInput } from "../api/tipos";
import { Icono } from "../ui/Iconos";

/** Valores de partida para un monitor nuevo. */
const NUEVO: MonitorInput = {
  name: "",
  url: "",
  method: "GET",
  expectedStatus: 0,
  intervalSeconds: 300,
  timeoutMs: 10000,
  active: true,
  notifyOnFailure: true,
};

interface Props {
  /** Si viene, se edita; si no, se crea. */
  monitor: Monitor | null;
  guardando: boolean;
  error: string | null;
  onGuardar(input: MonitorInput): void;
  onCerrar(): void;
}

export function DialogoMonitor({ monitor, guardando, error, onGuardar, onCerrar }: Props) {
  const [form, setForm] = useState<MonitorInput>(NUEVO);

  useEffect(() => {
    setForm(monitor ? { ...monitor } : NUEVO);
  }, [monitor]);

  // Cerrar con Escape: en un diálogo es lo que todo el mundo intenta primero.
  useEffect(() => {
    function alPulsar(e: KeyboardEvent) {
      if (e.key === "Escape") onCerrar();
    }
    window.addEventListener("keydown", alPulsar);
    return () => window.removeEventListener("keydown", alPulsar);
  }, [onCerrar]);

  function cambiar<K extends keyof MonitorInput>(campo: K, valor: MonitorInput[K]) {
    setForm((f) => ({ ...f, [campo]: valor }));
  }

  function alEnviar(e: FormEvent) {
    e.preventDefault();
    onGuardar(form);
  }

  return (
    <div className="fondo-dialogo" onMouseDown={onCerrar}>
      {/* Parar la propagación: un clic dentro del diálogo no debe cerrarlo. */}
      <div
        className="dialogo"
        role="dialog"
        aria-modal="true"
        aria-labelledby="dialogo-titulo"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <form onSubmit={alEnviar}>
          <div className="dialogo-cab">
            <div className="dialogo-cab-texto">
              <span className="etiqueta">{monitor ? "editar" : "nuevo"}</span>
              <h2 id="dialogo-titulo" className="dialogo-titular">
                {monitor ? (
                  <>
                    Ajusta <em>este monitor</em>.
                  </>
                ) : (
                  <>
                    Vigila <em>algo nuevo</em>.
                  </>
                )}
              </h2>
            </div>
            <button type="button" className="accion" onClick={onCerrar} aria-label="Cerrar" title="Cerrar">
              <Icono nombre="close" tamano={18} />
            </button>
          </div>

          <div className="dialogo-cuerpo">
            {error && (
              <div className="alerta alerta-error" role="alert">
                <Icono nombre="alert" tamano={16} />
                <span>{error}</span>
              </div>
            )}

            <div className="campo">
              <label htmlFor="name">Nombre</label>
              <input
                id="name"
                type="text"
                value={form.name}
                onChange={(e) => cambiar("name", e.target.value)}
                placeholder="Mi sitio web"
                required
              />
            </div>

            <div className="campo">
              <label htmlFor="url">URL</label>
              <input
                id="url"
                type="url"
                value={form.url}
                onChange={(e) => cambiar("url", e.target.value)}
                placeholder="https://midominio.com"
                required
              />
            </div>

            <div className="fila">
              <div className="campo">
                <label htmlFor="method">Método</label>
                <select
                  id="method"
                  value={form.method}
                  onChange={(e) => cambiar("method", e.target.value as HttpMethod)}
                >
                  <option value="GET">GET</option>
                  <option value="HEAD">HEAD</option>
                </select>
              </div>

              <div className="campo">
                <label htmlFor="expectedStatus">Código esperado</label>
                <input
                  id="expectedStatus"
                  type="number"
                  min={0}
                  max={599}
                  value={form.expectedStatus}
                  onChange={(e) => cambiar("expectedStatus", Number(e.target.value))}
                />
                <span className="campo-ayuda">0 = vale cualquier 2xx o 3xx</span>
              </div>
            </div>

            <div className="fila">
              <div className="campo">
                <label htmlFor="intervalSeconds">Cada cuánto (segundos)</label>
                <input
                  id="intervalSeconds"
                  type="number"
                  min={30}
                  value={form.intervalSeconds}
                  onChange={(e) => cambiar("intervalSeconds", Number(e.target.value))}
                />
                <span className="campo-ayuda">Mínimo 30</span>
              </div>

              <div className="campo">
                <label htmlFor="timeoutMs">Timeout (ms)</label>
                <input
                  id="timeoutMs"
                  type="number"
                  min={1000}
                  max={60000}
                  step={500}
                  value={form.timeoutMs}
                  onChange={(e) => cambiar("timeoutMs", Number(e.target.value))}
                />
                <span className="campo-ayuda">Entre 1000 y 60000</span>
              </div>
            </div>

            <div className="campo">
              <label className="casilla">
                <input
                  type="checkbox"
                  checked={form.active}
                  onChange={(e) => cambiar("active", e.target.checked)}
                />
                Vigilar automáticamente
              </label>
            </div>

            <div className="campo">
              <label className="casilla">
                <input
                  type="checkbox"
                  checked={form.notifyOnFailure}
                  onChange={(e) => cambiar("notifyOnFailure", e.target.checked)}
                />
                Avisar cuando se caiga o se recupere
              </label>
            </div>
          </div>

          <div className="dialogo-pie">
            <button type="button" className="btn" onClick={onCerrar}>
              Cancelar
            </button>
            <button type="submit" className="btn btn-principal" disabled={guardando}>
              {guardando ? "Guardando…" : "Guardar"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
