import { useState } from "react";

import { api, ApiError } from "../../api/client";
import type { ResultadoDb } from "../../api/tipos";
import { Alerta } from "../../ui/Alerta";
import { Confirmar } from "../../ui/Confirmar";
import { Icono } from "../../ui/Iconos";
import { esNulo, textoDeCelda } from "./celdas";

/** El servidor avisa de las sentencias destructivas con este prefijo. */
const AVISO = "⚠️";

/** Editor de SQL libre. Lo destructivo pide confirmación antes de ejecutarse. */
export function ConsultaSql({ conexion, sqlInicial }: { conexion: string; sqlInicial: string }) {
  const [sql, setSql] = useState(sqlInicial);
  const [ejecutando, setEjecutando] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [resultado, setResultado] = useState<ResultadoDb | null>(null);
  const [aConfirmar, setAConfirmar] = useState<string | null>(null);

  async function ejecutar(confirmado: boolean) {
    setEjecutando(true);
    setError(null);
    setAConfirmar(null);
    try {
      setResultado(await api.post<ResultadoDb>(`/db-admin/connections/${conexion}/query`, { sql, confirmDestructive: confirmado }));
    } catch (e) {
      const mensaje = e instanceof ApiError ? e.message : "No se pudo ejecutar.";
      if (!confirmado && mensaje.startsWith(AVISO)) {
        setAConfirmar(mensaje.replace(AVISO, "").trim());
      } else {
        setResultado(null);
        setError(mensaje);
      }
    } finally {
      setEjecutando(false);
    }
  }

  return (
    <div style={{ display: "grid", gap: "0.8rem" }}>
      <form
        className="cristal panel"
        onSubmit={(e) => { e.preventDefault(); void ejecutar(false); }}
      >
        <div className="campo">
          <label htmlFor="sql-texto">Consulta</label>
          <textarea
            id="sql-texto"
            className="entrada-mono editor-sql"
            rows={6}
            value={sql}
            onChange={(e) => setSql(e.target.value)}
            onKeyDown={(e) => {
              // Ctrl/Cmd + Enter ejecuta, como en cualquier cliente de SQL.
              if ((e.ctrlKey || e.metaKey) && e.key === "Enter") {
                e.preventDefault();
                void ejecutar(false);
              }
            }}
            spellCheck={false}
            placeholder="SELECT * FROM usuarios LIMIT 10;"
          />
        </div>
        <div className="acciones-fila">
          <button type="submit" className="btn btn-principal" disabled={ejecutando || !sql.trim()}>
            <Icono nombre={ejecutando ? "refresh" : "play"} tamano={15} />
            {ejecutando ? "Ejecutando…" : "Ejecutar"}
          </button>
          <span className="texto-apagado">Ctrl + Enter</span>
        </div>
      </form>

      {error && <Alerta tipo="error">{error}</Alerta>}

      {resultado && (
        <section aria-label="Resultado" style={{ display: "grid", gap: "0.6rem" }}>
          <div className="fichas">
            <span className="ficha">{resultado.rowCount} {resultado.rowCount === 1 ? "fila" : "filas"}</span>
            <span className="ficha">{resultado.durationMs} ms</span>
          </div>
          {resultado.columns.length === 0 ? (
            <p className="texto-apagado">La sentencia se ejecutó y no devuelve filas.</p>
          ) : (
            <div className="rejilla-datos cristal" role="region" aria-label="Filas devueltas" tabIndex={0}>
              <table>
                <thead>
                  <tr>{resultado.columns.map((c, i) => <th key={i}><span className="celda-cab">{c}</span></th>)}</tr>
                </thead>
                <tbody>
                  {resultado.rows.map((fila, i) => (
                    <tr key={i}>
                      {fila.map((v, j) => (
                        <td key={j} className={esNulo(v) ? "celda-nula" : ""} title={textoDeCelda(v)}>{textoDeCelda(v)}</td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      )}

      {aConfirmar && (
        <Confirmar
          titulo="Esta sentencia es destructiva"
          texto={`${aConfirmar}. ¿La ejecuto de todos modos?`}
          etiquetaConfirmar="Ejecutar"
          peligro
          trabajando={ejecutando}
          alConfirmar={() => void ejecutar(true)}
          alCancelar={() => setAConfirmar(null)}
        />
      )}
    </div>
  );
}
