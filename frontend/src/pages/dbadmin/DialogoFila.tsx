import { useState } from "react";

import { api, ApiError } from "../../api/client";
import type { ColumnaDb } from "../../api/tipos";
import { Dialogo } from "../../ui/Dialogo";
import { esNulo, esTextoLargo, textoDeCelda } from "./celdas";
import { Selector } from "../../ui/Selector";

type Modo = "valor" | "nulo" | "omitir";

interface Campo {
  texto: string;
  modo: Modo;
  /** Lo que había al abrir: solo se envía lo que cambió. */
  inicial: { texto: string; modo: Modo };
}

interface Props {
  conexion: string;
  schema: string;
  tabla: string;
  columnas: ColumnaDb[];
  columnasResultado: string[];
  /** null: fila nueva. */
  fila: unknown[] | null;
  alCerrar(): void;
  alGuardar(): void;
}

/**
 * Un campo vacío de una columna que admite NULL se guarda como NULL: para un
 * número o una fecha «vacío» no tiene otro sentido. En un texto solo se
 * hace al insertar; si se borra el contenido al editar, se quiere la cadena vacía.
 */
function vaciaEsNula(c: ColumnaDb, nueva: boolean): boolean {
  if (!c.isNullable) return false;
  return nueva || !/char|text/i.test(c.dataType);
}

/** Alta y edición de una fila. Al editar solo viajan los campos que cambiaron. */
export function DialogoFila({ conexion, schema, tabla, columnas, columnasResultado, fila, alCerrar, alGuardar }: Props) {
  const nueva = fila === null;

  const [campos, setCampos] = useState<Record<string, Campo>>(() =>
    Object.fromEntries(
      columnas.map((c) => {
        const v = nueva ? undefined : fila[columnasResultado.indexOf(c.name)];
        // En una fila nueva, lo que tiene valor por defecto se deja sin tocar.
        const modo: Modo = nueva ? (c.default !== null ? "omitir" : "valor") : esNulo(v) ? "nulo" : "valor";
        const texto = nueva || esNulo(v) ? "" : textoDeCelda(v);
        return [c.name, { texto, modo, inicial: { texto, modo } }];
      }),
    ),
  );
  const [enviando, setEnviando] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const poner = (n: string, parche: Partial<Campo>) => setCampos((m) => ({ ...m, [n]: { ...m[n], ...parche } }));

  async function guardar() {
    const valores: Record<string, unknown> = {};
    for (const c of columnas) {
      const f = campos[c.name];
      if (f.modo === "omitir") continue;
      const cambio = f.modo !== f.inicial.modo || f.texto !== f.inicial.texto;
      if (!nueva && !cambio) continue;
      valores[c.name] = f.modo === "nulo" || (f.texto === "" && vaciaEsNula(c, nueva)) ? null : f.texto;
    }
    if (!nueva && Object.keys(valores).length === 0) {
      alCerrar();
      return;
    }

    setEnviando(true);
    setError(null);
    try {
      const base = { schema, table: tabla, values: valores };
      if (nueva) {
        await api.post(`/db-admin/connections/${conexion}/rows`, base);
      } else {
        const key = Object.fromEntries(
          columnas.filter((c) => c.isPrimaryKey).map((c) => [c.name, fila[columnasResultado.indexOf(c.name)]]),
        );
        await api.put(`/db-admin/connections/${conexion}/rows`, { ...base, key });
      }
      alGuardar();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "No se pudo guardar la fila.");
    } finally {
      setEnviando(false);
    }
  }

  return (
    <Dialogo
      etiqueta={`${schema}.${tabla}`}
      titular={nueva ? "Una fila *nueva*." : "Edita *la fila*."}
      error={error}
      enviando={enviando}
      alEnviar={() => void guardar()}
      alCerrar={alCerrar}
    >
      {columnas.map((c) => {
        const f = campos[c.name];
        const id = `fila-${c.name}`;
        const bloqueada = f.modo !== "valor";
        return (
          <div className="campo" key={c.name}>
            <label htmlFor={id}>
              {c.name} <span className="campo-tipo">{c.dataType}{c.isPrimaryKey ? " · clave" : ""}{!c.isNullable ? " · obligatorio" : ""}</span>
            </label>
            <div className="campo-fila">
              {esTextoLargo(c.dataType) ? (
                <textarea id={id} className="entrada-mono" rows={3} value={f.texto} disabled={bloqueada} onChange={(e) => poner(c.name, { texto: e.target.value })} spellCheck={false} />
              ) : (
                <input id={id} type="text" className="entrada-mono" value={f.texto} disabled={bloqueada} onChange={(e) => poner(c.name, { texto: e.target.value })} spellCheck={false} placeholder={bloqueada ? (f.modo === "nulo" ? "NULL" : `por defecto: ${c.default}`) : ""} />
              )}
              <Selector
                aria-label={`Valor de ${c.name}`}
                value={f.modo}
                onChange={(e) => poner(c.name, { modo: e.target.value as Modo })}
              >
                <option value="valor">Valor</option>
                {c.isNullable && <option value="nulo">NULL</option>}
                {nueva && c.default !== null && <option value="omitir">Por defecto</option>}
              </Selector>
            </div>
          </div>
        );
      })}
    </Dialogo>
  );
}
