import { useEffect, useMemo, useRef, useState } from "react";

import type { DiagramaDb } from "../../api/tipos";
import { Alerta } from "../../ui/Alerta";
import { Icono } from "../../ui/Iconos";
import { Esqueletos, Vacio } from "../../ui/Vacio";
import { useDatos } from "../../ui/useDatos";
import { ALTO_CABECERA, ALTO_FILA, disponer, trazo } from "./diagrama";

const PASO = 0.15;
const ZOOM_MIN = 0.35;
const ZOOM_MAX = 2;
const ZOOM_INICIAL = 0.8;

interface Props {
  conexion: string;
  schema: string;
  /** Pulsar una tabla la abre. */
  alAbrir(tabla: string): void;
}

/** Diagrama entidad-relación de un esquema, dibujado en SVG, con zoom y pantalla completa. */
export function Diagrama({ conexion, schema, alAbrir }: Props) {
  const { datos, error, cargando } = useDatos<DiagramaDb>(`/db-admin/connections/${conexion}/diagram?schema=${encodeURIComponent(schema)}`);
  const dibujo = useMemo(() => (datos ? disponer(datos) : null), [datos]);

  const [zoom, setZoom] = useState(ZOOM_INICIAL);
  const [resaltada, setResaltada] = useState<string | null>(null);
  const [completa, setCompleta] = useState(false);
  const marco = useRef<HTMLDivElement>(null);
  const zoomPrevio = useRef(ZOOM_INICIAL);

  // La pantalla completa nativa también se abandona con Esc o con la interfaz del navegador.
  useEffect(() => {
    const alCambiar = () => {
      if (!document.fullscreenElement) {
        setCompleta((c) => {
          if (c) setZoom(zoomPrevio.current);
          return false;
        });
      }
    };
    document.addEventListener("fullscreenchange", alCambiar);
    return () => document.removeEventListener("fullscreenchange", alCambiar);
  }, []);

  // Sin la API nativa (iPhone) se usa una capa fija: Esc la cierra.
  useEffect(() => {
    if (!completa) return;
    const alPulsar = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !document.fullscreenElement) {
        setCompleta(false);
        setZoom(zoomPrevio.current);
      }
    };
    window.addEventListener("keydown", alPulsar);
    return () => window.removeEventListener("keydown", alPulsar);
  }, [completa]);

  if (error) return <Alerta tipo="error">{error}</Alerta>;
  if (cargando && !dibujo) return <Esqueletos cantidad={2} />;
  if (!dibujo) return null;
  if (dibujo.nodes.length === 0) {
    return <Vacio icono="database" titular="Nada que *dibujar*.">Este esquema no tiene tablas.</Vacio>;
  }

  const relacionadas = new Set<string>();
  if (resaltada) {
    relacionadas.add(resaltada);
    for (const e of dibujo.edges) {
      if (e.from === resaltada) relacionadas.add(e.to);
      if (e.to === resaltada) relacionadas.add(e.from);
    }
  }
  const apagada = (t: string) => resaltada !== null && !relacionadas.has(t);

  function alternarCompleta() {
    if (completa) {
      if (document.fullscreenElement) void document.exitFullscreen();
      else {
        setCompleta(false);
        setZoom(zoomPrevio.current);
      }
      return;
    }
    const el = marco.current;
    if (!el || !dibujo) return;

    zoomPrevio.current = zoom;
    setCompleta(true);

    const nativa = typeof el.requestFullscreen === "function";
    const ancho = nativa ? window.screen.width : window.innerWidth;
    const alto = nativa ? window.screen.height : window.innerHeight;
    // Que quepa entero, dejando sitio a la barra y a la leyenda.
    const ajuste = Math.min((ancho - 48) / dibujo.width, (alto - 120) / dibujo.height);
    setZoom(Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, ajuste)));

    if (nativa) {
      el.requestFullscreen().catch(() => {
        // El navegador lo denegó: la capa fija ya está puesta, no hay nada más que hacer.
      });
    }
  }

  return (
    <div ref={marco} className={`diagrama${completa ? " diagrama-completa" : ""}`}>
      <div className="barra-herramientas">
        <p className="texto-apagado diagrama-ayuda">
          {dibujo.nodes.length} tablas · {dibujo.edges.length} relaciones. Pasa el cursor por una tabla para resaltar sus vínculos; púlsala para abrirla.
        </p>
        <div className="diagrama-zoom">
          <button className="btn btn-pequeno" onClick={() => setZoom((z) => Math.max(ZOOM_MIN, z - PASO))} aria-label="Alejar" title="Alejar">−</button>
          <button className="btn btn-pequeno" onClick={() => setZoom(ZOOM_INICIAL)} aria-label="Restablecer el zoom" title="Restablecer el zoom">
            <Icono nombre="refresh" tamano={13} />
          </button>
          <button className="btn btn-pequeno" onClick={() => setZoom((z) => Math.min(ZOOM_MAX, z + PASO))} aria-label="Acercar" title="Acercar">+</button>
          <button className="btn btn-pequeno" onClick={alternarCompleta} aria-pressed={completa}>
            {completa ? "Salir" : "Pantalla completa"}
          </button>
        </div>
      </div>

      <div className="cristal diagrama-lienzo">
        <svg
          width={dibujo.width * zoom}
          height={dibujo.height * zoom}
          viewBox={`0 0 ${dibujo.width} ${dibujo.height}`}
          role="img"
          aria-label={`Diagrama del esquema ${schema}`}
        >
          <defs>
            <marker id="erd-flecha" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
              <path d="M 0 0 L 10 5 L 0 10 z" className="erd-flecha" />
            </marker>
          </defs>

          {/* Primero las relaciones, para que las cajas queden encima de las líneas. */}
          <g>
            {dibujo.edges.map((e) => {
              const activa = resaltada === null || e.from === resaltada || e.to === resaltada;
              return (
                <path
                  key={e.id}
                  d={trazo(e)}
                  fill="none"
                  markerEnd="url(#erd-flecha)"
                  className={`erd-linea${activa ? "" : " erd-apagada"}${activa && resaltada ? " erd-activa" : ""}`}
                >
                  <title>{`${e.from}.${e.label}`}</title>
                </path>
              );
            })}
          </g>

          {dibujo.nodes.map((n) => (
            <g
              key={n.table}
              transform={`translate(${n.x}, ${n.y})`}
              className={`erd-nodo${apagada(n.table) ? " erd-apagada-nodo" : ""}`}
              onMouseEnter={() => setResaltada(n.table)}
              onMouseLeave={() => setResaltada(null)}
              onClick={() => alAbrir(n.table)}
              role="button"
              tabIndex={0}
              aria-label={`Abrir la tabla ${n.table}`}
              onKeyDown={(e) => (e.key === "Enter" || e.key === " ") && alAbrir(n.table)}
              onFocus={() => setResaltada(n.table)}
              onBlur={() => setResaltada(null)}
            >
              <rect width={n.width} height={n.height} rx={8} className={`erd-caja${resaltada === n.table ? " erd-caja-activa" : ""}`} />
              <rect width={n.width} height={ALTO_CABECERA} rx={8} className="erd-cabecera" />
              {/* Cuadra las esquinas de abajo de la cabecera. */}
              <rect y={ALTO_CABECERA - 8} width={n.width} height={8} className="erd-cabecera" />
              <line y1={ALTO_CABECERA} x2={n.width} y2={ALTO_CABECERA} className="erd-separador" />
              <text x={10} y={ALTO_CABECERA / 2 + 4} className="erd-titulo">{n.table}</text>

              {n.columns.map((c) => (
                <g key={c.name}>
                  {c.isPrimaryKey ? (
                    <circle cx={14} cy={c.offsetY - 3} r={3} className="erd-pk" />
                  ) : c.isForeignKey ? (
                    <circle cx={14} cy={c.offsetY - 3} r={3} className="erd-fk" />
                  ) : null}
                  <text x={24} y={c.offsetY} className="erd-columna">{c.name}</text>
                  <text x={n.width - 10} y={c.offsetY} textAnchor="end" className="erd-tipo">{c.dataType}</text>
                </g>
              ))}

              {n.hiddenCount > 0 && (
                <text x={24} y={ALTO_CABECERA + n.columns.length * ALTO_FILA + ALTO_FILA / 2} className="erd-tipo erd-mas">
                  +{n.hiddenCount} columnas más
                </text>
              )}
            </g>
          ))}
        </svg>
      </div>

      <div className="diagrama-leyenda">
        <span><i className="erd-punto erd-punto-pk" /> Clave primaria</span>
        <span><i className="erd-punto erd-punto-fk" /> Clave foránea</span>
      </div>
    </div>
  );
}
