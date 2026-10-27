import type { DiagramaDb } from "../../api/tipos";
import { Alerta } from "../../ui/Alerta";
import { Esqueletos, Vacio } from "../../ui/Vacio";
import { useDatos } from "../../ui/useDatos";

interface Props {
  conexion: string;
  schema: string;
  alIrA(schema: string, tabla: string): void;
}

/**
 * Cómo se enlazan las tablas del esquema. Es una lista agrupada por tabla y no
 * un dibujo con flechas: dice lo mismo, se lee en cualquier pantalla y no hace
 * falta una librería de grafos.
 */
export function Relaciones({ conexion, schema, alIrA }: Props) {
  const { datos, error, cargando } = useDatos<DiagramaDb>(`/db-admin/connections/${conexion}/diagram?schema=${encodeURIComponent(schema)}`);

  if (error) return <Alerta tipo="error">{error}</Alerta>;
  if (cargando && !datos) return <Esqueletos cantidad={3} />;
  if (!datos) return null;

  const porTabla = new Map<string, DiagramaDb["relations"]>();
  for (const r of datos.relations) porTabla.set(r.table, [...(porTabla.get(r.table) ?? []), r]);
  const conRelaciones = datos.tables.filter((t) => porTabla.has(t.name));
  const sueltas = datos.tables.filter((t) => !porTabla.has(t.name));

  if (datos.relations.length === 0) {
    return <Vacio icono="link" titular="Sin *relaciones*.">Ninguna tabla de «{schema}» tiene claves ajenas.</Vacio>;
  }

  return (
    <div style={{ display: "grid", gap: "0.8rem" }}>
      <p className="texto-apagado">{datos.relations.length} {datos.relations.length === 1 ? "relación" : "relaciones"} entre {datos.tables.length} tablas.</p>
      <div className="rejilla">
        {conRelaciones.map((t, i) => (
          <article key={t.name} className="tarjeta cristal subir" style={{ "--i": i + 1 } as React.CSSProperties}>
            <div className="tarjeta-cab">
              <div className="tarjeta-titulo">
                <button className="enlace-tabla tarjeta-nombre" onClick={() => alIrA(t.schema, t.name)}>{t.name}</button>
                <span className="tarjeta-sub">{t.columns.length} columnas</span>
              </div>
            </div>
            <ul className="lista-simple">
              {porTabla.get(t.name)!.map((r) => (
                <li key={r.constraintName}>
                  <code>{r.columns.join(", ")}</code>
                  <span aria-hidden>→</span>
                  <button className="enlace-tabla" onClick={() => alIrA(r.refSchema, r.refTable)}>
                    {r.refTable}<span className="texto-apagado">.{r.refColumns.join(", ")}</span>
                  </button>
                </li>
              ))}
            </ul>
          </article>
        ))}
      </div>
      {sueltas.length > 0 && (
        <p className="texto-apagado">
          Sin claves ajenas propias: {sueltas.map((t) => t.name).join(", ")}.
        </p>
      )}
    </div>
  );
}
