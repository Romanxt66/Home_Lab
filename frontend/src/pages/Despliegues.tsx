import { useState } from "react";
import type { FormEvent } from "react";

import { api, ApiError } from "../api/client";
import type { AnalisisDespliegue } from "../api/tipos";
import { Alerta } from "../ui/Alerta";
import { Cabecera } from "../ui/Cabecera";
import { Codigo } from "../ui/Codigo";
import { Confirmar } from "../ui/Confirmar";
import { Icono } from "../ui/Iconos";
import { Segmentado } from "../ui/Segmentado";

const BASES = [
  { valor: "postgresql", etiqueta: "PostgreSQL" },
  { valor: "mysql", etiqueta: "MySQL" },
  { valor: "mongodb", etiqueta: "MongoDB" },
  { valor: "redis", etiqueta: "Redis" },
] as const;
type TipoBase = (typeof BASES)[number]["valor"];

interface Archivo {
  nombre: string;
  contenido: string;
}

/** Generador de despliegues: mira un repositorio y propone su Dockerfile. */
export function Despliegues() {
  const [repoUrl, setRepoUrl] = useState("");
  const [rama, setRama] = useState("");
  const [subdir, setSubdir] = useState("");
  const [analizando, setAnalizando] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [analisis, setAnalisis] = useState<AnalisisDespliegue | null>(null);

  const [porSubir, setPorSubir] = useState<Archivo | null>(null);
  const [subiendo, setSubiendo] = useState(false);
  const [exito, setExito] = useState<string | null>(null);

  const [tipoBase, setTipoBase] = useState<TipoBase>("postgresql");
  const [plantilla, setPlantilla] = useState<string | null>(null);
  const [cargandoBase, setCargandoBase] = useState(false);

  async function analizar(e: FormEvent) {
    e.preventDefault();
    setAnalizando(true);
    setError(null);
    setExito(null);
    setPlantilla(null);
    try {
      setAnalisis(await api.post<AnalisisDespliegue>("/deploygen/analyze", { repoUrl, branch: rama, subdir }));
    } catch (err) {
      setAnalisis(null);
      setError(err instanceof ApiError ? err.message : "No se pudo analizar el repositorio.");
    } finally {
      setAnalizando(false);
    }
  }

  async function subir() {
    if (!analisis || !porSubir) return;
    setSubiendo(true);
    setError(null);
    try {
      const r = await api.post<{ mensaje: string }>("/deploygen/commit", {
        repoUrl,
        branch: analisis.branch,
        baseDir: analisis.baseDir,
        fileName: porSubir.nombre,
        content: porSubir.contenido,
      });
      setExito(r.mensaje);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudo subir el archivo.");
    } finally {
      setSubiendo(false);
      setPorSubir(null);
    }
  }

  async function verPlantilla(tipo: TipoBase) {
    setTipoBase(tipo);
    setCargandoBase(true);
    try {
      const r = await api.get<{ compose: string }>(`/deploygen/database-template?type=${tipo}`);
      setPlantilla(r.compose);
    } catch (err) {
      setPlantilla(null);
      setError(err instanceof ApiError ? err.message : "No se pudo cargar la plantilla.");
    } finally {
      setCargandoBase(false);
    }
  }

  const d = analisis?.detection;
  const botonSubir = (nombre: string, contenido: string) => (
    <button type="button" className="btn btn-pequeno" onClick={() => setPorSubir({ nombre, contenido })}>
      <Icono nombre="rocket" tamano={13} />
      Subir al repositorio
    </button>
  );

  return (
    <div className="pagina">
      <Cabecera
        etiqueta="generador de despliegues"
        titular="Del repositorio al *contenedor*."
        entradilla="Analiza un repositorio de GitHub y genera el Dockerfile que necesita para desplegarse."
      />

      <form className="cristal panel subir" style={{ "--i": 1 } as React.CSSProperties} onSubmit={analizar}>
        <div className="campo">
          <label htmlFor="dg-url">Repositorio</label>
          <input id="dg-url" type="url" value={repoUrl} onChange={(e) => setRepoUrl(e.target.value)} placeholder="https://github.com/usuario/proyecto" required />
        </div>
        <div className="fila">
          <div className="campo">
            <label htmlFor="dg-rama">Rama</label>
            <input id="dg-rama" type="text" className="entrada-mono" value={rama} onChange={(e) => setRama(e.target.value)} placeholder="la principal" spellCheck={false} />
          </div>
          <div className="campo">
            <label htmlFor="dg-sub">Subcarpeta</label>
            <input id="dg-sub" type="text" className="entrada-mono" value={subdir} onChange={(e) => setSubdir(e.target.value)} placeholder="raíz del repositorio" spellCheck={false} />
          </div>
        </div>
        <div className="acciones-fila">
          <button type="submit" className="btn btn-principal" disabled={analizando}>
            <Icono nombre={analizando ? "refresh" : "search"} tamano={15} />
            {analizando ? "Analizando…" : "Analizar"}
          </button>
        </div>
      </form>

      {error && <Alerta tipo="error" alCerrar={() => setError(null)}>{error}</Alerta>}
      {exito && <Alerta alCerrar={() => setExito(null)}>{exito}</Alerta>}

      {analisis && d && (
        <section className="subir" aria-label="Resultado del análisis" style={{ display: "grid", gap: "0.9rem" }}>
          <div className="cristal panel">
            <div className="panel-cab">
              <h2>{analisis.owner}/{analisis.repo}</h2>
              <span className="ficha">{analisis.branch}{analisis.baseDir ? ` · ${analisis.baseDir}` : ""}</span>
            </div>
            <div className="fichas">
              <span className="ficha ficha-estado" style={{ "--tono": "var(--acento)" } as React.CSSProperties}>{d.framework || d.runtime}</span>
              <span className="ficha">{d.runtime}{d.runtimeVersion ? ` ${d.runtimeVersion}` : ""}</span>
              {d.packageManager && <span className="ficha">{d.packageManager}</span>}
              <span className="ficha">puerto {d.port}</span>
              {d.isStatic && <span className="ficha">estático{d.publishDir ? ` · ${d.publishDir}` : ""}</span>}
              {d.needsDatabase && <span className="ficha">necesita {d.databaseType || "base de datos"}</span>}
              <span className="ficha">build pack: {d.recommendedBuildPack}</span>
            </div>
            {(d.hasDockerfile || d.hasCompose) && (
              <p className="texto-apagado">
                El repositorio ya tiene {d.hasDockerfile ? "un Dockerfile" : ""}
                {d.hasDockerfile && d.hasCompose ? " y " : ""}
                {d.hasCompose ? "un docker-compose" : ""}. Subir los generados los sobrescribiría.
              </p>
            )}
            {d.notes.length > 0 && (
              <ul className="notas">
                {d.notes.map((n, i) => <li key={i}>{n}</li>)}
              </ul>
            )}
            <dl className="comandos">
              {([["instalar", d.installCommand], ["compilar", d.buildCommand], ["arrancar", d.startCommand]] as const)
                .filter(([, v]) => v)
                .map(([k, v]) => (
                  <div key={k}><dt className="etiqueta">{k}</dt><dd>{v}</dd></div>
                ))}
            </dl>
          </div>

          <Codigo titulo="Dockerfile" texto={analisis.dockerfile} extra={botonSubir("Dockerfile", analisis.dockerfile)} />
          {analisis.compose && (
            <Codigo titulo="docker-compose.yml" texto={analisis.compose} extra={botonSubir("docker-compose.yml", analisis.compose)} />
          )}
        </section>
      )}

      <section className="cristal panel subir" style={{ "--i": 2 } as React.CSSProperties} aria-label="Plantillas de bases de datos">
        <div className="panel-cab">
          <h2>Una base de datos suelta</h2>
          <Segmentado<TipoBase>
            etiqueta="Tipo de base"
            valor={tipoBase}
            opciones={BASES.map((b) => ({ valor: b.valor, etiqueta: b.etiqueta }))}
            alCambiar={(t) => void verPlantilla(t)}
          />
        </div>
        {plantilla === null ? (
          <div className="acciones-fila">
            <button type="button" className="btn" disabled={cargandoBase} onClick={() => void verPlantilla(tipoBase)}>
              <Icono nombre="database" tamano={15} />
              {cargandoBase ? "Cargando…" : "Ver plantilla de compose"}
            </button>
          </div>
        ) : (
          <Codigo titulo={`compose · ${tipoBase}`} texto={plantilla} />
        )}
      </section>

      {porSubir && analisis && (
        <Confirmar
          titulo={`¿Subir ${porSubir.nombre} a ${analisis.owner}/${analisis.repo}?`}
          texto={`Se hará un commit directo en la rama «${analisis.branch}»${d?.hasDockerfile || d?.hasCompose ? " y se sobrescribirá el archivo si ya existe" : ""}.`}
          etiquetaConfirmar="Subir"
          trabajando={subiendo}
          alConfirmar={() => void subir()}
          alCancelar={() => setPorSubir(null)}
        />
      )}
    </div>
  );
}
