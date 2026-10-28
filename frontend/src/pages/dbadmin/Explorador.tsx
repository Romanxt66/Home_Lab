import { useState } from "react";

import type { ConexionDb, DetalleTablaDb, EsquemaDb, TablaDb } from "../../api/tipos";
import { Alerta } from "../../ui/Alerta";
import { Icono } from "../../ui/Iconos";
import { Segmentado } from "../../ui/Segmentado";
import { Esqueletos, Vacio } from "../../ui/Vacio";
import { useDatos } from "../../ui/useDatos";
import { ConsultaSql } from "./ConsultaSql";
import { Datos } from "./Datos";
import { Estructura } from "./Estructura";
import { Diagrama } from "./Diagrama";
import { Selector } from "../../ui/Selector";

type Vista = "datos" | "estructura" | "diagrama" | "sql";

/** Una conexión abierta: esquemas y tablas a la izquierda, lo que se haga con ellas a la derecha. */
export function Explorador({ conexion, alCerrar }: { conexion: ConexionDb; alCerrar(): void }) {
  const esquemas = useDatos<EsquemaDb[]>(`/db-admin/connections/${conexion.id}/schemas`);
  const [esquema, setEsquema] = useState<string | null>(null);
  const [tabla, setTabla] = useState<string | null>(null);
  const [vista, setVista] = useState<Vista>("datos");
  const [filtro, setFiltro] = useState("");

  const lista = esquemas.datos ?? [];
  // Por defecto «public» si existe y, si no, el primero que no sea de sistema.
  const esquemaActivo = esquema ?? lista.find((e) => e.name === "public")?.name ?? lista.find((e) => !e.isSystem)?.name ?? lista[0]?.name ?? null;

  const tablas = useDatos<TablaDb[]>(esquemaActivo ? `/db-admin/connections/${conexion.id}/tables?schema=${encodeURIComponent(esquemaActivo)}` : null);
  const detalle = useDatos<DetalleTablaDb>(
    esquemaActivo && tabla ? `/db-admin/connections/${conexion.id}/table?schema=${encodeURIComponent(esquemaActivo)}&table=${encodeURIComponent(tabla)}` : null,
  );

  const visibles = (tablas.datos ?? []).filter((t) => t.name.toLowerCase().includes(filtro.toLowerCase()));

  function irA(schema: string, nombre: string) {
    setEsquema(schema);
    setTabla(nombre);
    setVista("estructura");
  }

  return (
    <section className="explorador subir" aria-label={`Explorando ${conexion.name}`}>
      <aside className="cristal explorador-lateral">
        <div className="panel-cab">
          <button className="btn btn-pequeno btn-fantasma" onClick={alCerrar}>
            <Icono nombre="chevronL" tamano={14} />
            Conexiones
          </button>
          {conexion.readOnly && <span className="ficha">solo lectura</span>}
        </div>
        <div className="campo">
          <label htmlFor="ex-esq">Esquema</label>
          <Selector id="ex-esq" value={esquemaActivo ?? ""} onChange={(e) => { setEsquema(e.target.value); setTabla(null); }}>
            {lista.map((e) => <option key={e.name} value={e.name}>{e.name}{e.isSystem ? " (sistema)" : ""}</option>)}
          </Selector>
        </div>
        <div className="busqueda">
          <Icono nombre="search" tamano={15} />
          <input type="text" value={filtro} onChange={(e) => setFiltro(e.target.value)} placeholder="Filtrar tablas" aria-label="Filtrar tablas" />
        </div>
        {tablas.cargando && !tablas.datos ? (
          <Esqueletos cantidad={3} />
        ) : (
          <ul className="lista-tablas" aria-label="Tablas">
            {visibles.map((t) => (
              <li key={t.name}>
                <button
                  className={t.name === tabla ? "activa" : ""}
                  aria-current={t.name === tabla}
                  onClick={() => { setTabla(t.name); if (vista === "diagrama" || vista === "sql") setVista("datos"); }}
                >
                  <Icono nombre={t.kind === "view" ? "search" : "list"} tamano={13} />
                  <span>{t.name}</span>
                  <span className="lista-tablas-n">{t.rowsApprox >= 0 ? `~${t.rowsApprox}` : ""}</span>
                </button>
              </li>
            ))}
            {visibles.length === 0 && <li className="texto-apagado" style={{ padding: "0.4rem 0.6rem" }}>{tablas.datos?.length ? "Ninguna coincide." : "Sin tablas."}</li>}
          </ul>
        )}
      </aside>

      <div className="explorador-principal">
        {(esquemas.error ?? tablas.error) && <Alerta tipo="error">{esquemas.error ?? tablas.error}</Alerta>}

        <div className="barra-herramientas">
          <Segmentado<Vista>
            etiqueta="Vista"
            valor={vista}
            opciones={[
              { valor: "datos", etiqueta: "Datos" },
              { valor: "estructura", etiqueta: "Estructura" },
              { valor: "diagrama", etiqueta: "Diagrama" },
              { valor: "sql", etiqueta: "SQL" },
            ]}
            alCambiar={setVista}
          />
          {tabla && vista !== "sql" && vista !== "diagrama" && <span className="ficha">{esquemaActivo}.{tabla}</span>}
        </div>

        {vista === "sql" ? (
          <ConsultaSql key={conexion.id} conexion={conexion.id} sqlInicial={tabla ? `SELECT * FROM ${esquemaActivo}.${tabla} LIMIT 50;` : ""} />
        ) : vista === "diagrama" ? (
          esquemaActivo ? <Diagrama conexion={conexion.id} schema={esquemaActivo} alAbrir={(t) => { setTabla(t); setVista("datos"); }} /> : null
        ) : !tabla || !esquemaActivo ? (
          <Vacio icono="database" titular="Elige una *tabla*.">Pulsa una en la lista para ver sus datos o su estructura.</Vacio>
        ) : (
          <>
            {detalle.error && <Alerta tipo="error">{detalle.error}</Alerta>}
            {vista === "datos" ? (
              <Datos key={`${esquemaActivo}.${tabla}`} conexion={conexion.id} schema={esquemaActivo} tabla={tabla} detalle={detalle.datos} soloLectura={conexion.readOnly} />
            ) : detalle.datos ? (
              <Estructura detalle={detalle.datos} alIrA={irA} />
            ) : (
              <Esqueletos cantidad={2} />
            )}
          </>
        )}
      </div>
    </section>
  );
}
