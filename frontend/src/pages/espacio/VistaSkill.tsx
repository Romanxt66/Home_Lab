import { useEffect, useState } from "react";
import { createPortal } from "react-dom";

import { api, urlApi } from "../../api/client";
import type { DetalleEspacio } from "../../api/tipos";
import { Button, Glass } from "../../design";
import { Alerta } from "../../ui/Alerta";
import { Icono } from "../../ui/Iconos";
import { Esqueletos } from "../../ui/Vacio";
import { useDatos } from "../../ui/useDatos";
import { hace } from "../../ui/tiempo";
import { tamano } from "./formato";

interface Props {
  id: string;
  alCerrar(): void;
}

/** Por dentro de una skill: sus archivos, el texto de cada uno y su historial. */
export function VistaSkill({ id, alCerrar }: Props) {
  const { datos, error, cargando } = useDatos<DetalleEspacio>(`/workspace/items/${id}`);
  const [elegido, setElegido] = useState<string | null>(null);
  const [version, setVersion] = useState(0);
  const [texto, setTexto] = useState<{ ruta: string; version: number; contenido: string } | null>(null);
  const [errorArchivo, setErrorArchivo] = useState<string | null>(null);

  // Al abrir, se enseña SKILL.md: es lo que explica qué hace la skill.
  const ruta = elegido ?? (datos?.files.some((f) => f.path === "SKILL.md") ? "SKILL.md" : (datos?.files[0]?.path ?? null));
  const info = datos?.files.find((f) => f.path === ruta);

  useEffect(() => {
    const cerrar = (e: KeyboardEvent) => e.key === "Escape" && alCerrar();
    window.addEventListener("keydown", cerrar);
    return () => window.removeEventListener("keydown", cerrar);
  }, [alCerrar]);

  useEffect(() => {
    if (!ruta || !info?.text) return;
    let vigente = true;
    api
      .get<{ content: string }>(`/workspace/items/${id}/file?${new URLSearchParams({ path: ruta, version: String(version) })}`)
      .then((r) => {
        if (!vigente) return;
        setTexto({ ruta, version, contenido: r.content });
        setErrorArchivo(null);
      })
      .catch((e) => vigente && setErrorArchivo(e instanceof Error ? e.message : "No se pudo abrir el archivo."));
    return () => {
      vigente = false;
    };
  }, [id, ruta, version, info?.text]);

  const item = datos?.item;
  const verTexto = texto && texto.ruta === ruta && texto.version === version ? texto.contenido : null;

  return createPortal(
    <div className="cr cr-overlay" onMouseDown={(e) => e.target === e.currentTarget && alCerrar()}>
      <Glass strong className="cr-dialog vista-skill" role="dialog" aria-modal="true" aria-label={item?.name ?? "Skill"}>
        <div className="dialogo-cab">
          <div className="dialogo-cab-texto">
            <span className="cr-page-tag">{item?.kind === "doc" ? "documento" : "skill"}</span>
            <h2 className="dialogo-titular">{item?.name ?? "…"}</h2>
            {item && <p className="label cr-muted">{item.description}</p>}
          </div>
          <Button variant="ghost" className="cr-btn-icon" onClick={alCerrar} aria-label="Cerrar" title="Cerrar">
            <Icono nombre="close" tamano={18} />
          </Button>
        </div>

        <div className="dialogo-cuerpo">
          {error && <Alerta tipo="error">{error}</Alerta>}
          {cargando && !datos ? (
            <Esqueletos cantidad={2} />
          ) : datos ? (
            <>
              <div className="fichas">
                <span className="ficha">v{item!.version}</span>
                <span className="ficha">{datos.files.length} archivos</span>
                <span className="ficha">{tamano(item!.bytes)}</span>
                {!item!.mine && <span className="ficha">de {item!.ownerName}</span>}
              </div>

              <div className="vista-skill-cuerpo">
                <ul className="vista-skill-archivos" aria-label="Archivos">
                  {datos.files.map((f) => (
                    <li key={f.path}>
                      <button className={f.path === ruta ? "activa" : ""} onClick={() => setElegido(f.path)} title={f.path}>
                        <Icono nombre="file" tamano={13} />
                        <span>{f.path}</span>
                        <span className="lista-tablas-n">{tamano(f.bytes)}</span>
                      </button>
                    </li>
                  ))}
                </ul>
                <div className="vista-skill-texto" tabIndex={0} role="region" aria-label={`Contenido de ${ruta ?? ""}`}>
                  {errorArchivo ? (
                    <Alerta tipo="error">{errorArchivo}</Alerta>
                  ) : info && !info.text ? (
                    <p className="texto-apagado">Este archivo no es de texto ({tamano(info.bytes)}). Descarga la skill para usarlo.</p>
                  ) : verTexto === null ? (
                    <p className="texto-apagado">Cargando…</p>
                  ) : (
                    <pre>{verTexto}</pre>
                  )}
                </div>
              </div>

              <h3 className="heading" style={{ marginTop: "var(--space-4)" }}>Historial</h3>
              <ul className="historial-versiones">
                {datos.versions.map((v) => (
                  <li key={v.number}>
                    <span className="ficha">v{v.number}</span>
                    <span className="historial-texto">
                      <span className="label">{v.message || "Sin mensaje"}</span>
                      <span className="caption cr-muted">{v.authorName} · {hace(v.createdAt)} · {v.files} archivos</span>
                    </span>
                    <button className="cr-btn cr-btn-ghost cr-btn-sm" onClick={() => { setVersion(v.number === item!.version ? 0 : v.number); setElegido(null); }}>
                      {(version === v.number || (version === 0 && v.number === item!.version)) ? "Viendo" : "Ver"}
                    </button>
                    <a className="cr-btn cr-btn-glass cr-btn-sm" href={urlApi(`/workspace/items/${id}/download?version=${v.number}`)} download>
                      <Icono nombre="download" tamano={13} />
                      Bajar
                    </a>
                  </li>
                ))}
              </ul>
            </>
          ) : null}
        </div>
      </Glass>
    </div>,
    document.body,
  );
}
