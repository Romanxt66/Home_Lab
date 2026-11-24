import { useRef, useState } from "react";

import { api, ApiError } from "../../api/client";
import type { ElementoEspacio } from "../../api/tipos";
import { Dialogo } from "../../ui/Dialogo";
import { Icono } from "../../ui/Iconos";
import { tamano } from "./formato";

interface Props {
  alCerrar(): void;
  alSubir(e: ElementoEspacio): void;
}

/** Subir un .skill o un .zip. El servidor lo comprueba antes de guardarlo. */
export function DialogoSubir({ alCerrar, alSubir }: Props) {
  const entrada = useRef<HTMLInputElement>(null);
  const [archivo, setArchivo] = useState<File | null>(null);
  const [mensaje, setMensaje] = useState("");
  const [compartir, setCompartir] = useState(false);
  const [enviando, setEnviando] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [arrastrando, setArrastrando] = useState(false);

  async function enviar() {
    if (!archivo) {
      setError("Elige primero un archivo .skill o .zip.");
      return;
    }
    setEnviando(true);
    setError(null);
    try {
      const datos = new FormData();
      datos.append("file", archivo);
      datos.append("message", mensaje);
      datos.append("visibility", compartir ? "shared" : "private");
      alSubir(await api.subir<ElementoEspacio>("/workspace/skills", datos));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "No se pudo subir.");
    } finally {
      setEnviando(false);
    }
  }

  function elegir(f: File | undefined | null) {
    if (!f) return;
    setArchivo(f);
    setError(null);
  }

  return (
    <Dialogo
      etiqueta="nueva skill"
      titular="Sube una *skill*."
      etiquetaEnviar="Subir"
      error={error}
      enviando={enviando}
      alEnviar={() => void enviar()}
      alCerrar={alCerrar}
    >
      <div
        className={`soltar${arrastrando ? " soltar-activo" : ""}`}
        onDragOver={(e) => { e.preventDefault(); setArrastrando(true); }}
        onDragLeave={() => setArrastrando(false)}
        onDrop={(e) => { e.preventDefault(); setArrastrando(false); elegir(e.dataTransfer.files[0]); }}
      >
        <Icono nombre="upload" tamano={22} />
        {archivo ? (
          <p className="label">{archivo.name} <span className="cr-muted">· {tamano(archivo.size)}</span></p>
        ) : (
          <p className="label">Arrastra aquí tu .skill o .zip</p>
        )}
        <button type="button" className="cr-btn cr-btn-glass cr-btn-sm" onClick={() => entrada.current?.click()}>
          Elegir archivo
        </button>
        <input ref={entrada} type="file" accept=".skill,.zip,application/zip" hidden onChange={(e) => elegir(e.target.files?.[0])} />
      </div>
      <span className="campo-ayuda" style={{ display: "block", marginTop: "var(--space-2)" }}>
        Debe tener un SKILL.md con «name» y «description». Si ya tienes una skill con ese nombre, se guarda como una versión nueva.
      </span>

      <div className="campo" style={{ marginTop: "var(--space-4)" }}>
        <label htmlFor="sk-msg">Qué cambió (opcional)</label>
        <input id="sk-msg" type="text" value={mensaje} onChange={(e) => setMensaje(e.target.value)} maxLength={200} placeholder="Primera versión" />
      </div>
      <label className="casilla">
        <input type="checkbox" checked={compartir} onChange={(e) => setCompartir(e.target.checked)} />
        Compartirla con el resto de personas
      </label>
    </Dialogo>
  );
}
