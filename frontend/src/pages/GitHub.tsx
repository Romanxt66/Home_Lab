import { useEffect, useState } from "react";
import type { FormEvent } from "react";

import { api, ApiError } from "../api/client";
import type { ConfigGitHub, RepoGitHub, ResumenGitHub } from "../api/tipos";
import { Alerta } from "../ui/Alerta";
import { Cabecera } from "../ui/Cabecera";
import { Confirmar } from "../ui/Confirmar";
import { Dialogo } from "../ui/Dialogo";
import { Icono } from "../ui/Iconos";
import { Segmentado } from "../ui/Segmentado";
import { Esqueletos, Vacio } from "../ui/Vacio";
import { useDatos } from "../ui/useDatos";
import { hace } from "../ui/tiempo";

type Orden = "recent" | "stars" | "name";

/** GitHub: tus repositorios, con búsqueda, orden y filtros. */
export function GitHub() {
  const config = useDatos<ConfigGitHub | null>("/github/config");

  const [busqueda, setBusqueda] = useState("");
  const [consulta, setConsulta] = useState("");
  const [orden, setOrden] = useState<Orden>("recent");
  const [forks, setForks] = useState(true);
  const [archivados, setArchivados] = useState(true);

  // La búsqueda espera a que se deje de teclear: sin esto, cada letra sería
  // una petición, y GitHub tiene un límite de peticiones por hora.
  useEffect(() => {
    const t = setTimeout(() => setConsulta(busqueda), 300);
    return () => clearTimeout(t);
  }, [busqueda]);

  // No se pide el resumen hasta saber que hay una cuenta configurada.
  const hayCuenta = config.datos != null;
  const ruta = hayCuenta
    ? `/github/overview?${new URLSearchParams({ q: consulta, sort: orden, forks: String(forks), archived: String(archivados) })}`
    : null;
  const resumen = useDatos<ResumenGitHub>(ruta);

  const [dialogo, setDialogo] = useState(false);
  const [guardando, setGuardando] = useState(false);
  const [errorDialogo, setErrorDialogo] = useState<string | null>(null);
  const [desconectando, setDesconectando] = useState(false);
  const [trabajando, setTrabajando] = useState(false);
  const [aviso, setAviso] = useState<string | null>(null);

  async function guardar(usuario: string, token: string) {
    setGuardando(true);
    setErrorDialogo(null);
    try {
      await api.post("/github/config", { username: usuario, token });
      setDialogo(false);
      config.recargar();
      resumen.recargar();
    } catch (err) {
      setErrorDialogo(err instanceof ApiError ? err.message : "No se pudo guardar.");
    } finally {
      setGuardando(false);
    }
  }

  async function desconectar() {
    if (!config.datos) return;
    setTrabajando(true);
    try {
      await api.del(`/github/config/${config.datos.id}`);
      config.recargar();
    } catch (err) {
      setAviso(err instanceof Error ? err.message : "No se pudo desconectar.");
    } finally {
      setTrabajando(false);
      setDesconectando(false);
    }
  }

  const perfil = resumen.datos?.profile;
  const repos = resumen.datos?.repos ?? [];

  return (
    <div className="pagina">
      <Cabecera
        etiqueta="github"
        titular="Todo tu código, *a mano*."
        entradilla="Tus repositorios con búsqueda, orden y filtros, sin salir del panel."
        acciones={
          hayCuenta && (
            <button className="btn" onClick={() => { setErrorDialogo(null); setDialogo(true); }}>
              <Icono nombre="user" tamano={15} />
              Cambiar cuenta
            </button>
          )
        }
      />

      {(config.error ?? resumen.error) && <Alerta tipo="error">{config.error ?? resumen.error}</Alerta>}
      {aviso && <Alerta tipo="error" alCerrar={() => setAviso(null)}>{aviso}</Alerta>}

      {config.cargando && !config.datos && !config.error ? (
        <Esqueletos cantidad={2} />
      ) : !hayCuenta ? (
        <Vacio
          icono="git"
          titular="Conecta *tu cuenta*."
          accion={
            <button className="btn btn-principal" onClick={() => { setErrorDialogo(null); setDialogo(true); }}>
              Conectar GitHub
            </button>
          }
        >
          Pon tu usuario para ver tus repositorios públicos. Con un token, también los privados.
        </Vacio>
      ) : (
        <>
          {perfil && (
            <section className="cristal panel perfil subir" style={{ "--i": 1 } as React.CSSProperties} aria-label="Perfil">
              <img className="perfil-foto" src={perfil.avatarUrl} alt="" width={64} height={64} referrerPolicy="no-referrer" />
              <div className="perfil-datos">
                <div className="perfil-nombre">
                  <h2>{perfil.name ?? perfil.login}</h2>
                  <a className="tarjeta-sub" href={perfil.htmlUrl} target="_blank" rel="noopener noreferrer">
                    @{perfil.login}
                  </a>
                </div>
                {perfil.bio && <p className="texto-apagado">{perfil.bio}</p>}
                <div className="fichas">
                  <span className="ficha">{perfil.publicRepos} repos públicos</span>
                  <span className="ficha">{perfil.followers} seguidores</span>
                  <span className="ficha">{perfil.following} siguiendo</span>
                  {perfil.location && <span className="ficha">{perfil.location}</span>}
                  {perfil.company && <span className="ficha">{perfil.company}</span>}
                  <span className="ficha">{config.datos?.hasToken ? "con token" : "sin token: solo públicos"}</span>
                </div>
              </div>
              <button className="btn btn-fantasma btn-peligro-texto btn-pequeno" onClick={() => setDesconectando(true)}>
                Desconectar
              </button>
            </section>
          )}

          <div className="barra-herramientas subir" style={{ "--i": 2 } as React.CSSProperties}>
            <div className="busqueda">
              <Icono nombre="search" tamano={16} />
              <input
                type="text"
                value={busqueda}
                onChange={(e) => setBusqueda(e.target.value)}
                placeholder="Buscar por nombre, descripción o tema"
                aria-label="Buscar repositorios"
              />
            </div>
            <Segmentado<Orden>
              etiqueta="Ordenar por"
              valor={orden}
              alCambiar={setOrden}
              opciones={[
                { valor: "recent", etiqueta: "Recientes" },
                { valor: "stars", etiqueta: "Estrellas" },
                { valor: "name", etiqueta: "Nombre" },
              ]}
            />
            <label className="casilla">
              <input type="checkbox" checked={forks} onChange={(e) => setForks(e.target.checked)} />
              Forks
            </label>
            <label className="casilla">
              <input type="checkbox" checked={archivados} onChange={(e) => setArchivados(e.target.checked)} />
              Archivados
            </label>
          </div>

          {resumen.cargando && !resumen.datos ? (
            <Esqueletos />
          ) : repos.length === 0 ? (
            <div className="cristal vacio subir">
              <p>{consulta ? `Ningún repositorio coincide con «${consulta}».` : "No hay repositorios que enseñar."}</p>
            </div>
          ) : (
            <>
              <p className="texto-apagado subir" aria-live="polite">
                {repos.length} {repos.length === 1 ? "repositorio" : "repositorios"}
              </p>
              <div className="rejilla" style={{ opacity: resumen.cargando ? 0.6 : 1, transition: "opacity 200ms" }}>
                {repos.map((r, i) => (
                  <TarjetaRepo key={r.id} repo={r} indice={Math.min(i, 8) + 3} />
                ))}
              </div>
            </>
          )}
        </>
      )}

      {dialogo && (
        <DialogoCuenta
          actual={config.datos}
          guardando={guardando}
          error={errorDialogo}
          alGuardar={(u, t) => void guardar(u, t)}
          alCerrar={() => setDialogo(false)}
        />
      )}

      {desconectando && (
        <Confirmar
          titulo="¿Desconectar GitHub?"
          texto="Se olvidará tu usuario y tu token. También dejará de funcionar el generador de despliegues con repositorios privados."
          etiquetaConfirmar="Desconectar"
          peligro
          trabajando={trabajando}
          alConfirmar={() => void desconectar()}
          alCancelar={() => setDesconectando(false)}
        />
      )}
    </div>
  );
}

function TarjetaRepo({ repo: r, indice }: { repo: RepoGitHub; indice: number }) {
  return (
    <article className="tarjeta cristal subir" style={{ "--i": indice } as React.CSSProperties}>
      <div className="tarjeta-cab">
        <div className="tarjeta-titulo">
          <a className="tarjeta-nombre enlace" href={r.url} target="_blank" rel="noopener noreferrer" title={r.fullName}>
            {r.name}
          </a>
          <span className="tarjeta-sub">{r.defaultBranch}</span>
        </div>
        {r.isPrivate && <span className="ficha ficha-aviso">privado</span>}
        {r.isFork && <span className="ficha">fork</span>}
        {r.isArchived && <span className="ficha">archivado</span>}
      </div>

      {r.description && <p className="descripcion">{r.description}</p>}

      {r.topics.length > 0 && (
        <div className="fichas">
          {r.topics.slice(0, 4).map((t) => (
            <span key={t} className="ficha ficha-acento">{t}</span>
          ))}
          {r.topics.length > 4 && <span className="ficha">+{r.topics.length - 4}</span>}
        </div>
      )}

      <div className="metadatos">
        {r.language && <span>{r.language}</span>}
        <span title="Estrellas"><Icono nombre="star" tamano={12} /> {r.stars}</span>
        <span title="Forks">{r.forks} forks</span>
        {r.openIssues > 0 && <span title="Incidencias abiertas">{r.openIssues} abiertas</span>}
        <span>{hace(r.pushedAt)}</span>
      </div>
    </article>
  );
}

function DialogoCuenta({ actual, guardando, error, alGuardar, alCerrar }: {
  actual: ConfigGitHub | null;
  guardando: boolean;
  error: string | null;
  alGuardar(usuario: string, token: string): void;
  alCerrar(): void;
}) {
  const [usuario, setUsuario] = useState(actual?.username ?? "");
  const [token, setToken] = useState("");

  function enviar(e?: FormEvent) {
    e?.preventDefault();
    alGuardar(usuario, token);
  }

  return (
    <Dialogo
      etiqueta="github"
      titular={actual ? "Cambia *de cuenta*." : "Conecta *tu cuenta*."}
      error={error}
      enviando={guardando}
      alEnviar={enviar}
      alCerrar={alCerrar}
    >
      <div className="campo">
        <label htmlFor="gh-usuario">Usuario</label>
        <input id="gh-usuario" type="text" value={usuario} onChange={(e) => setUsuario(e.target.value)} placeholder="tu-usuario" required autoFocus />
      </div>
      <div className="campo">
        <label htmlFor="gh-token">Token (opcional)</label>
        <input
          id="gh-token"
          type="password"
          className="entrada-mono"
          value={token}
          onChange={(e) => setToken(e.target.value)}
          placeholder={actual?.hasToken ? "•••••• guardado: vacío para conservarlo" : "ghp_… o github_pat_…"}
          autoComplete="off"
        />
        <span className="campo-ayuda">
          Sin token solo se ven los repositorios públicos y el límite de peticiones es mucho más bajo.
        </span>
      </div>
    </Dialogo>
  );
}
