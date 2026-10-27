import type { ClaveAjenaDb, DetalleTablaDb } from "../../api/tipos";
import { Icono } from "../../ui/Iconos";

interface Props {
  detalle: DetalleTablaDb;
  alIrA(schema: string, tabla: string): void;
}

/** Columnas, índices y claves ajenas de una tabla. */
export function Estructura({ detalle, alIrA }: Props) {
  return (
    <div style={{ display: "grid", gap: "1rem" }}>
      <section className="cristal rejilla-datos" aria-label="Columnas" tabIndex={0}>
        <table>
          <thead>
            <tr><th>#</th><th>columna</th><th>tipo</th><th>nulo</th><th>por defecto</th><th>referencia</th></tr>
          </thead>
          <tbody>
            {detalle.columns.map((c) => (
              <tr key={c.name}>
                <td>{c.ordinalPosition}</td>
                <td>
                  <span className="nombre-columna">
                    {c.isPrimaryKey && <Icono nombre="star" tamano={11} />}
                    {c.name}
                  </span>
                </td>
                <td className="celda-mono">{c.dataType}</td>
                <td>{c.isNullable ? "sí" : "no"}</td>
                <td className="celda-mono" title={c.default ?? ""}>{c.default ?? "—"}</td>
                <td>
                  {c.references ? (
                    <button className="enlace-tabla" onClick={() => alIrA(c.references!.schema, c.references!.table)}>
                      {c.references.table}.{c.references.column}
                    </button>
                  ) : "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <section className="cristal panel" aria-label="Índices">
        <h2>Índices</h2>
        {detalle.indexes.length === 0 ? (
          <p className="texto-apagado">Esta tabla no tiene índices.</p>
        ) : (
          <ul className="lista-simple">
            {detalle.indexes.map((i) => (
              <li key={i.name}>
                <span className="fichas">
                  <span className="ficha">{i.name}</span>
                  {i.isPrimary && <span className="ficha">primaria</span>}
                  {i.isUnique && !i.isPrimary && <span className="ficha">única</span>}
                </span>
                <code>{i.definition}</code>
              </li>
            ))}
          </ul>
        )}
      </section>

      <div className="dos-columnas">
        <Claves titulo="Apunta a" vacio="No tiene claves ajenas." claves={detalle.foreignKeys} destino="ref" alIrA={alIrA} />
        <Claves titulo="Le apuntan" vacio="Ninguna otra tabla depende de ésta." claves={detalle.referencedBy} destino="origen" alIrA={alIrA} />
      </div>
    </div>
  );
}

function Claves({ titulo, vacio, claves, destino, alIrA }: {
  titulo: string;
  vacio: string;
  claves: ClaveAjenaDb[];
  /** «ref»: se enseña la tabla a la que se apunta; «origen»: la que apunta. */
  destino: "ref" | "origen";
  alIrA(schema: string, tabla: string): void;
}) {
  return (
    <section className="cristal panel" aria-label={titulo}>
      <h2>{titulo}</h2>
      {claves.length === 0 ? (
        <p className="texto-apagado">{vacio}</p>
      ) : (
        <ul className="lista-simple">
          {claves.map((k) => {
            const tabla = destino === "ref" ? k.refTable : k.table;
            const esquema = destino === "ref" ? k.refSchema : k.schema;
            return (
              <li key={k.constraintName + k.table}>
                <button className="enlace-tabla" onClick={() => alIrA(esquema, tabla)}>{tabla}</button>
                <code>{k.columns.join(", ")} → {k.refColumns.join(", ")}</code>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
