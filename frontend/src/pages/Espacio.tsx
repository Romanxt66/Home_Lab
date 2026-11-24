import { useState } from "react";

import { api, urlApi } from "../api/client";
import type { ElementoEspacio } from "../api/tipos";
import { useSesion } from "../api/sesion";
import { Alerta } from "../ui/Alerta";
import { Cabecera } from "../ui/Cabecera";
import { Confirmar } from "../ui/Confirmar";
import { Icono } from "../ui/Iconos";
import { Segmentado } from "../ui/Segmentado";
import { Esqueletos, Vacio } from "../ui/Vacio";
import { useDatos } from "../ui/useDatos";
import { hace } from "../ui/tiempo";
import { ConexionClaude } from "./espacio/ConexionClaude";
import { DialogoSubir } from "./espacio/DialogoSubir";
import { EditorDocs } from "./espacio/EditorDocs";
import { VistaSkill } from "./espacio/VistaSkill";
import { tamano } from "./espacio/formato";

type Pestana = "skills" | "docs" | "claude";
type Filtro = "all" | "mine" | "shared";

/** Espacio de trabajo: tus skills de Claude y tus documentos de instrucciones, con historial. */
export function Espacio() {
  const { usuario } = useSesion();
  const esAdmin = usuario?.role === "superadmin";
  const [pestana, setPestana] = useState<Pestana>("skills");

  const skills = useDatos<ElementoEspacio[]>("/workspace/items?kind=skill");
  const docs = useDatos<ElementoEspacio[]>("/workspace/items?kind=doc");

  return (
    <div className="pagina">
      <Cabecera
        etiqueta="espacio de trabajo"
        titular="Tus instrucciones, *a mano*."
        entradilla="Guarda tus skills y tus documentos .md para Claude, con historial, y bájalos cuando los necesites."
      />

      <div className="barra-herramientas">
        <Segmentado<Pestana>
          etiqueta="Apartado"
          valor={pestana}
          opciones={[
            { valor: "skills", etiqueta: `Skills${skills.datos ? ` · ${skills.datos.length}` : ""}` },
            { valor: "docs", etiqueta: `Documentos${docs.datos ? ` · ${docs.datos.length}` : ""}` },
            { valor: "claude", etiqueta: "Claude" },
          ]}
          alCambiar={setPestana}
        />
      </div>

      {pestana === "skills" ? (
        <Skills lista={skills.datos} cargando={skills.cargando} error={skills.error} esAdmin={esAdmin} recargar={skills.recargar} />
      ) : pestana === "docs" ? (
        <EditorDocs docs={docs.datos ?? []} cargando={docs.cargando} esAdmin={esAdmin} alCambiar={docs.recargar} />
      ) : (
        <ConexionClaude />
      )}
    </div>
  );
}

function Skills({ lista, cargando, error, esAdmin, recargar }: {
  lista: ElementoEspacio[] | null;
  cargando: boolean;
  error: string | null;
  esAdmin: boolean;
  recargar(): void;
}) {
  const [filtro, setFiltro] = useState<Filtro>("all");
  const [subiendo, setSubiendo] = useState(false);
  const [viendo, setViendo] = useState<string | null>(null);
  const [porBorrar, setPorBorrar] = useState<ElementoEspacio | null>(null);
  const [aviso, setAviso] = useState<{ tipo: "info" | "error"; texto: string } | null>(null);

  const visibles = (lista ?? []).filter((s) => filtro === "all" || (filtro === "mine" ? s.mine : !s.mine));

  async function cambiarVisibilidad(s: ElementoEspacio) {
    try {
      await api.put(`/workspace/items/${s.id}/visibility`, { visibility: s.visibility === "shared" ? "private" : "shared" });
      recargar();
    } catch (e) {
      setAviso({ tipo: "error", texto: e instanceof Error ? e.message : "No se pudo cambiar." });
    }
  }

  async function borrar() {
    if (!porBorrar) return;
    try {
      await api.del(`/workspace/items/${porBorrar.id}`);
      recargar();
    } catch (e) {
      setAviso({ tipo: "error", texto: e instanceof Error ? e.message : "No se pudo eliminar." });
    } finally {
      setPorBorrar(null);
    }
  }

  return (
    <>
      <div className="barra-herramientas">
        <Segmentado<Filtro>
          etiqueta="Qué skills ver"
          valor={filtro}
          opciones={[{ valor: "all", etiqueta: "Todas" }, { valor: "mine", etiqueta: "Mías" }, { valor: "shared", etiqueta: "De otras personas" }]}
          alCambiar={setFiltro}
        />
        <span style={{ flex: 1 }} />
        <a className="cr-btn cr-btn-glass cr-btn-sm" href={urlApi(`/workspace/download?scope=${filtro === "all" ? "all" : filtro === "mine" ? "mine" : "shared"}`)} download>
          <Icono nombre="download" tamano={14} />
          Bajar todo
        </a>
        <button className="cr-btn cr-btn-primary cr-btn-sm" onClick={() => setSubiendo(true)}>
          <Icono nombre="upload" tamano={14} />
          Subir skill
        </button>
      </div>

      {(error || aviso) && <Alerta tipo={aviso?.tipo ?? "error"} alCerrar={aviso ? () => setAviso(null) : undefined}>{aviso?.texto ?? error}</Alerta>}

      {cargando && !lista ? (
        <Esqueletos cantidad={3} />
      ) : visibles.length === 0 ? (
        <Vacio
          icono="folder"
          titular={filtro === "shared" ? "Nada *compartido*." : "Aún no hay *skills*."}
          accion={filtro !== "shared" && (
            <button className="cr-btn cr-btn-primary" onClick={() => setSubiendo(true)}>
              <Icono nombre="upload" tamano={16} />
              Subir la primera
            </button>
          )}
        >
          {filtro === "shared" ? "Cuando alguien comparta una skill aparecerá aquí." : "Sube un .skill o un .zip y quedará guardado con su historial."}
        </Vacio>
      ) : (
        <div className="rejilla">
          {visibles.map((s, i) => (
            <article key={s.id} className="tarjeta cr-glass subir" style={{ "--i": Math.min(i, 8) } as React.CSSProperties} aria-label={s.name}>
              <div className="tarjeta-cab">
                <span className="tarjeta-avatar"><Icono nombre="folder" tamano={18} /></span>
                <div className="tarjeta-titulo">
                  <span className="tarjeta-nombre" title={s.name}>{s.name}</span>
                  <span className="tarjeta-sub">{s.mine ? "tuya" : `de ${s.ownerName}`} · {hace(s.updatedAt)}</span>
                </div>
                {(s.mine || esAdmin) && (
                  <div className="tarjeta-iconos">
                    <button className="accion peligro" onClick={() => setPorBorrar(s)} aria-label={`Eliminar ${s.name}`} title="Eliminar"><Icono nombre="trash" tamano={16} /></button>
                  </div>
                )}
              </div>
              <p className="descripcion-skill" title={s.description}>{s.description}</p>
              <div className="fichas">
                <span className="ficha">v{s.version}</span>
                <span className="ficha">{s.files} archivos</span>
                <span className="ficha">{tamano(s.bytes)}</span>
                {s.visibility === "shared" && <span className="ficha ficha-acento">compartida</span>}
              </div>
              <div className="tarjeta-pie acciones-tarjeta">
                <button className="cr-btn cr-btn-glass cr-btn-sm" onClick={() => setViendo(s.id)}>
                  <Icono nombre="search" tamano={13} />
                  Ver
                </button>
                <a className="cr-btn cr-btn-glass cr-btn-sm" href={urlApi(`/workspace/items/${s.id}/download`)} download aria-label={`Bajar ${s.name}`}>
                  <Icono nombre="download" tamano={13} />
                  Bajar
                </a>
                {s.mine && (
                  <button className="cr-btn cr-btn-ghost cr-btn-sm" onClick={() => void cambiarVisibilidad(s)}>
                    {s.visibility === "shared" ? "Dejar de compartir" : "Compartir"}
                  </button>
                )}
              </div>
            </article>
          ))}
        </div>
      )}

      {subiendo && (
        <DialogoSubir
          alCerrar={() => setSubiendo(false)}
          alSubir={(e) => {
            setSubiendo(false);
            setAviso({ tipo: "info", texto: e.version > 1 ? `«${e.name}» ya existía: se guardó como la versión ${e.version}.` : `«${e.name}» subida.` });
            recargar();
          }}
        />
      )}
      {viendo && <VistaSkill id={viendo} alCerrar={() => setViendo(null)} />}
      {porBorrar && (
        <Confirmar
          titulo={`¿Eliminar «${porBorrar.name}»?`}
          texto={porBorrar.mine ? "Se borra la skill con todo su historial. No se puede deshacer." : `Es de ${porBorrar.ownerName}: la retiras de la biblioteca de todo el mundo.`}
          etiquetaConfirmar="Eliminar"
          peligro
          alConfirmar={() => void borrar()}
          alCancelar={() => setPorBorrar(null)}
        />
      )}
    </>
  );
}
