import { useState } from "react";

import { api, ApiError } from "../../api/client";
import type { DetalleTablaDb, ResultadoDb } from "../../api/tipos";
import { Alerta } from "../../ui/Alerta";
import { Confirmar } from "../../ui/Confirmar";
import { Icono } from "../../ui/Iconos";
import { Esqueletos, Vacio } from "../../ui/Vacio";
import { useDatos } from "../../ui/useDatos";
import { DialogoFila } from "./DialogoFila";
import { esNulo, textoDeCelda } from "./celdas";

const POR_PAGINA = 50;

interface Props {
  conexion: string;
  schema: string;
  tabla: string;
  detalle: DetalleTablaDb | null;
  soloLectura: boolean;
}

/** Las filas de una tabla, con orden, paginación y edición por clave primaria. */
export function Datos({ conexion, schema, tabla, detalle, soloLectura }: Props) {
  const [pagina, setPagina] = useState(0);
  const [orden, setOrden] = useState<{ col: string; dir: "asc" | "desc" } | null>(null);

  const consulta = new URLSearchParams({
    schema, table: tabla, limit: String(POR_PAGINA), offset: String(pagina * POR_PAGINA),
    ...(orden ? { orderBy: orden.col, direction: orden.dir } : {}),
  });
  const filas = useDatos<ResultadoDb>(`/db-admin/connections/${conexion}/rows?${consulta}`);

  const [editando, setEditando] = useState<{ fila: unknown[] | null } | null>(null);
  const [porBorrar, setPorBorrar] = useState<unknown[] | null>(null);
  const [trabajando, setTrabajando] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const columnas = detalle?.columns ?? [];
  const claves = columnas.filter((c) => c.isPrimaryKey).map((c) => c.name);
  // Sin clave primaria no hay forma segura de señalar una fila: solo se puede insertar.
  const editable = !soloLectura && claves.length > 0;
  const r = filas.datos;

  const claveDeFila = (fila: unknown[]) =>
    Object.fromEntries(claves.map((k) => [k, fila[r!.columns.indexOf(k)]]));

  async function borrar() {
    if (!porBorrar) return;
    setTrabajando(true);
    setError(null);
    try {
      await api.delConCuerpo(`/db-admin/connections/${conexion}/rows`, { schema, table: tabla, key: claveDeFila(porBorrar) });
      filas.recargar();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "No se pudo borrar.");
    } finally {
      setTrabajando(false);
      setPorBorrar(null);
    }
  }

  function ordenarPor(col: string) {
    setPagina(0);
    setOrden((o) => (o?.col !== col ? { col, dir: "asc" } : o.dir === "asc" ? { col, dir: "desc" } : null));
  }

  return (
    <div style={{ display: "grid", gap: "0.8rem" }}>
      <div className="barra-herramientas">
        {r && <span className="ficha">{r.rowCount} {r.rowCount === 1 ? "fila" : "filas"} aquí · {r.durationMs} ms</span>}
        {!soloLectura && claves.length === 0 && detalle && <span className="ficha">sin clave primaria: solo se puede insertar</span>}
        <span style={{ flex: 1 }} />
        <button className="btn btn-pequeno" onClick={filas.recargar} disabled={filas.cargando}>
          <Icono nombre="refresh" tamano={14} />
          Actualizar
        </button>
        {!soloLectura && (
          <button className="btn btn-pequeno btn-principal" onClick={() => setEditando({ fila: null })} disabled={!detalle}>
            <Icono nombre="plus" tamano={14} />
            Nueva fila
          </button>
        )}
      </div>

      {(filas.error || error) && <Alerta tipo="error" alCerrar={error ? () => setError(null) : undefined}>{filas.error ?? error}</Alerta>}

      {filas.cargando && !r ? (
        <Esqueletos cantidad={3} />
      ) : r && r.rows.length === 0 ? (
        <Vacio icono="database" titular={pagina > 0 ? "No hay *más filas*." : "Tabla *vacía*."}>
          {pagina > 0 ? "Esta página ya no tiene datos." : "Todavía no hay ninguna fila."}
        </Vacio>
      ) : r ? (
        <div className="rejilla-datos cristal" role="region" aria-label={`Filas de ${tabla}`} tabIndex={0}>
          <table>
            <thead>
              <tr>
                {editable && <th aria-label="Acciones" />}
                {r.columns.map((c) => (
                  <th key={c} aria-sort={orden?.col === c ? (orden.dir === "asc" ? "ascending" : "descending") : "none"}>
                    <button onClick={() => ordenarPor(c)} title="Ordenar">
                      {c}
                      {claves.includes(c) && <Icono nombre="star" tamano={10} />}
                      {orden?.col === c && <span aria-hidden>{orden.dir === "asc" ? "↑" : "↓"}</span>}
                    </button>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {r.rows.map((fila, i) => (
                <tr key={i}>
                  {editable && (
                    <td className="celda-acciones">
                      <button className="accion" onClick={() => setEditando({ fila })} aria-label={`Editar la fila ${pagina * POR_PAGINA + i + 1}`} title="Editar"><Icono nombre="edit" tamano={14} /></button>
                      <button className="accion peligro" onClick={() => setPorBorrar(fila)} aria-label={`Borrar la fila ${pagina * POR_PAGINA + i + 1}`} title="Borrar"><Icono nombre="trash" tamano={14} /></button>
                    </td>
                  )}
                  {fila.map((v, j) => (
                    <td key={j} className={esNulo(v) ? "celda-nula" : ""} title={textoDeCelda(v)}>{textoDeCelda(v)}</td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}

      {r && (pagina > 0 || r.hasMore) && (
        <div className="barra-herramientas" style={{ justifyContent: "center" }}>
          <button className="btn btn-pequeno" onClick={() => setPagina((p) => p - 1)} disabled={pagina === 0}>
            <Icono nombre="chevronL" tamano={14} />
            Anterior
          </button>
          <span className="ficha">filas {pagina * POR_PAGINA + 1}–{pagina * POR_PAGINA + r.rowCount}</span>
          <button className="btn btn-pequeno" onClick={() => setPagina((p) => p + 1)} disabled={!r.hasMore}>
            Siguiente
            <Icono nombre="chevronR" tamano={14} />
          </button>
        </div>
      )}

      {editando && detalle && (
        <DialogoFila
          conexion={conexion}
          schema={schema}
          tabla={tabla}
          columnas={detalle.columns}
          columnasResultado={r?.columns ?? []}
          fila={editando.fila}
          alCerrar={() => setEditando(null)}
          alGuardar={() => { setEditando(null); filas.recargar(); }}
        />
      )}
      {porBorrar && (
        <Confirmar
          titulo="¿Borrar esta fila?"
          texto={`Se eliminará de ${schema}.${tabla} (${claves.map((k) => `${k} = ${textoDeCelda(claveDeFila(porBorrar)[k])}`).join(", ")}). No se puede deshacer.`}
          etiquetaConfirmar="Borrar"
          peligro
          trabajando={trabajando}
          alConfirmar={() => void borrar()}
          alCancelar={() => setPorBorrar(null)}
        />
      )}
    </div>
  );
}
