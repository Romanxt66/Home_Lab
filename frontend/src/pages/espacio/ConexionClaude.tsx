import { useState } from "react";
import { createPortal } from "react-dom";

import { api, ApiError, origenApi } from "../../api/client";
import type { ConexionOAuth, TokenApi } from "../../api/tipos";
import { Button, Glass } from "../../design";
import { Alerta } from "../../ui/Alerta";
import { Codigo } from "../../ui/Codigo";
import { Confirmar } from "../../ui/Confirmar";
import { Dialogo } from "../../ui/Dialogo";
import { Icono } from "../../ui/Iconos";
import { Esqueletos } from "../../ui/Vacio";
import { useDatos } from "../../ui/useDatos";
import { hace } from "../../ui/tiempo";

const PERMISOS = {
  read: { titulo: "Solo leer", detalle: "Claude puede listar y leer tus skills y documentos. Es lo más seguro." },
  write: { titulo: "Leer y guardar documentos", detalle: "Además puede crear y actualizar documentos .md (cada cambio queda en el historial)." },
} as const;

/** Enlazar con Claude: tokens personales para el servidor MCP del espacio de trabajo. */
export function ConexionClaude() {
  const tokens = useDatos<TokenApi[]>("/tokens");
  const conexiones = useDatos<ConexionOAuth[]>("/oauth/grants");
  const [porDesconectar, setPorDesconectar] = useState<ConexionOAuth | null>(null);
  const [creando, setCreando] = useState(false);
  const [recien, setRecien] = useState<{ token: string; item: TokenApi } | null>(null);
  const [porRevocar, setPorRevocar] = useState<TokenApi | null>(null);
  const [aviso, setAviso] = useState<string | null>(null);

  const activos = (tokens.datos ?? []).filter((t) => !t.revokedAt);
  const revocados = (tokens.datos ?? []).filter((t) => t.revokedAt);
  const url = `${origenApi()}/mcp`;

  async function desconectar() {
    if (!porDesconectar) return;
    try {
      await api.del(`/oauth/grants/${porDesconectar.id}`);
      conexiones.recargar();
    } catch (e) {
      setAviso(e instanceof Error ? e.message : "No se pudo desconectar.");
    } finally {
      setPorDesconectar(null);
    }
  }

  async function revocar() {
    if (!porRevocar) return;
    try {
      await api.del(`/tokens/${porRevocar.id}`);
      tokens.recargar();
    } catch (e) {
      setAviso(e instanceof Error ? e.message : "No se pudo revocar.");
    } finally {
      setPorRevocar(null);
    }
  }

  return (
    <div style={{ display: "grid", gap: "var(--space-4)" }}>
      <section className="cr-glass cr-pad panel" aria-label="Cómo conectar">
        <h2 className="heading">Conecta Claude a tu espacio de trabajo</h2>
        <p className="body cr-muted">
          Home Lab habla el protocolo MCP: Claude puede <strong>listar y leer tus skills y documentos</strong> y, si lo permites,{" "}
          <strong>guardar documentos</strong>, sin que tengas que copiar nada a mano. Hay dos formas de conectarlo.
        </p>
        <h3 className="label">Desde claude.ai (sin tokens)</h3>
        <ol className="pasos-conexion">
          <li>En claude.ai, abre <em>Ajustes → Conectores → Añadir conector personalizado</em>.</li>
          <li>Pega esta dirección: <code>{url}</code></li>
          <li>Te pedirá entrar en Home Lab y elegir qué permiso concedes. Luego aparece aquí abajo, en «Aplicaciones conectadas».</li>
        </ol>
        <h3 className="label">Desde Claude Code (con un token)</h3>
        <ol className="pasos-conexion">
          <li>Crea un token aquí abajo. Se enseña <strong>una sola vez</strong>.</li>
          <li>Ejecuta en tu terminal el comando que te damos, con ese token.</li>
          <li>Pide cosas como «lee mi CLAUDE.md de homelab» o «guarda estas reglas como documento».</li>
        </ol>
      </section>

      <div className="barra-herramientas">
        <h2 className="heading">Aplicaciones conectadas</h2>
      </div>
      {conexiones.error && <Alerta tipo="error">{conexiones.error}</Alerta>}
      {conexiones.cargando && !conexiones.datos ? (
        <Esqueletos cantidad={1} />
      ) : (conexiones.datos ?? []).length === 0 ? (
        <p className="texto-apagado">Ninguna aplicación se ha conectado por OAuth todavía.</p>
      ) : (
        <ul className="cr-glass lista-tokens" aria-label="Aplicaciones conectadas">
          {(conexiones.datos ?? []).map((c) => (
            <li key={c.id}>
              <span className="tarjeta-avatar" style={{ width: 36, height: 36 }}><Icono nombre="link" tamano={16} /></span>
              <span className="historial-texto">
                <span className="label">{c.clientName}</span>
                <span className="caption cr-muted">
                  {c.scope === "write" ? "leer y guardar" : "solo leer"} · conectada {hace(c.createdAt)} ·{" "}
                  {c.lastUsedAt ? `usada ${hace(c.lastUsedAt)}` : "sin usar todavía"}
                </span>
              </span>
              <button className="cr-btn cr-btn-ghost cr-btn-danger cr-btn-sm" onClick={() => setPorDesconectar(c)}>
                Desconectar
              </button>
            </li>
          ))}
        </ul>
      )}

      <div className="barra-herramientas">
        <h2 className="heading">Tus tokens personales</h2>
        <span style={{ flex: 1 }} />
        <button className="cr-btn cr-btn-primary cr-btn-sm" onClick={() => setCreando(true)}>
          <Icono nombre="plus" tamano={14} />
          Nuevo token
        </button>
      </div>

      {(tokens.error || aviso) && <Alerta tipo="error" alCerrar={aviso ? () => setAviso(null) : undefined}>{aviso ?? tokens.error}</Alerta>}

      {tokens.cargando && !tokens.datos ? (
        <Esqueletos cantidad={2} />
      ) : activos.length === 0 && revocados.length === 0 ? (
        <p className="texto-apagado">Todavía no has creado ningún token.</p>
      ) : (
        <ul className="cr-glass lista-tokens" aria-label="Tokens">
          {[...activos, ...revocados].map((t) => (
            <li key={t.id} className={t.revokedAt ? "revocado" : ""}>
              <span className="tarjeta-avatar" style={{ width: 36, height: 36 }}><Icono nombre="link" tamano={16} /></span>
              <span className="historial-texto">
                <span className="label">{t.name}</span>
                <span className="caption cr-muted">
                  <code>{t.prefix}…</code> · {t.scope === "write" ? "leer y guardar" : "solo leer"} ·{" "}
                  {t.revokedAt ? `revocado ${hace(t.revokedAt)}` : t.lastUsedAt ? `usado ${hace(t.lastUsedAt)}` : "sin usar todavía"}
                </span>
              </span>
              {t.revokedAt ? (
                <span className="ficha">revocado</span>
              ) : (
                <button className="cr-btn cr-btn-ghost cr-btn-danger cr-btn-sm" onClick={() => setPorRevocar(t)}>
                  Revocar
                </button>
              )}
            </li>
          ))}
        </ul>
      )}

      {creando && (
        <DialogoNuevoToken
          alCerrar={() => setCreando(false)}
          alCrear={(r) => {
            setCreando(false);
            setRecien(r);
            tokens.recargar();
          }}
        />
      )}
      {recien && <TokenCreado url={url} token={recien.token} item={recien.item} alCerrar={() => setRecien(null)} />}
      {porDesconectar && (
        <Confirmar
          titulo={`¿Desconectar «${porDesconectar.clientName}»?`}
          texto="Dejará de poder entrar al momento. Si quiere volver, tendrás que autorizarla otra vez."
          etiquetaConfirmar="Desconectar"
          peligro
          alConfirmar={() => void desconectar()}
          alCancelar={() => setPorDesconectar(null)}
        />
      )}
      {porRevocar && (
        <Confirmar
          titulo={`¿Revocar «${porRevocar.name}»?`}
          texto="Quien lo use dejará de poder entrar al momento. No se puede deshacer, pero puedes crear otro."
          etiquetaConfirmar="Revocar"
          peligro
          alConfirmar={() => void revocar()}
          alCancelar={() => setPorRevocar(null)}
        />
      )}
    </div>
  );
}

function DialogoNuevoToken({ alCerrar, alCrear }: { alCerrar(): void; alCrear(r: { token: string; item: TokenApi }): void }) {
  const [nombre, setNombre] = useState("");
  const [permiso, setPermiso] = useState<"read" | "write">("read");
  const [enviando, setEnviando] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function crear() {
    setEnviando(true);
    setError(null);
    try {
      alCrear(await api.post<{ token: string; item: TokenApi }>("/tokens", { name: nombre, scope: permiso }));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "No se pudo crear.");
    } finally {
      setEnviando(false);
    }
  }

  return (
    <Dialogo etiqueta="nuevo token" titular="Una llave *nueva*." etiquetaEnviar="Crear token" error={error} enviando={enviando} alEnviar={() => void crear()} alCerrar={alCerrar}>
      <div className="campo">
        <label htmlFor="tk-nombre">Nombre</label>
        <input id="tk-nombre" type="text" value={nombre} onChange={(e) => setNombre(e.target.value)} maxLength={60} placeholder="Mi portátil" required autoFocus />
        <span className="campo-ayuda">Para reconocerlo luego en la lista.</span>
      </div>
      <fieldset className="permisos" style={{ border: 0, padding: 0, margin: 0 }}>
        <legend className="label cr-muted" style={{ marginBottom: "var(--space-2)" }}>Qué puede hacer</legend>
        {(Object.keys(PERMISOS) as (keyof typeof PERMISOS)[]).map((k) => (
          <label key={k} className={`permiso${permiso === k ? " elegido" : ""}`}>
            <input type="radio" name="permiso" value={k} checked={permiso === k} onChange={() => setPermiso(k)} />
            <span>
              <span className="label">{PERMISOS[k].titulo}{k === "read" ? " (recomendado)" : ""}</span>
              <span className="caption cr-muted">{PERMISOS[k].detalle}</span>
            </span>
          </label>
        ))}
      </fieldset>
    </Dialogo>
  );
}

/** El único momento en que se ve el secreto: con los comandos listos para copiar. */
function TokenCreado({ url, token, item, alCerrar }: { url: string; token: string; item: TokenApi; alCerrar(): void }) {
  const comando = `claude mcp add --transport http homelab ${url} --header "Authorization: Bearer ${token}"`;
  const conVariable = [
    `export HOMELAB_TOKEN="${token}"`,
    "",
    "# .mcp.json (así el token no queda escrito en el archivo)",
    JSON.stringify({ mcpServers: { homelab: { type: "http", url, headers: { Authorization: "Bearer ${HOMELAB_TOKEN}" } } } }, null, 2),
  ].join("\n");

  return createPortal(
    <div className="cr cr-overlay">
      <Glass strong className="cr-dialog vista-skill" role="dialog" aria-modal="true" aria-labelledby="token-creado">
        <div className="dialogo-cab">
          <div className="dialogo-cab-texto">
            <span className="cr-page-tag">token creado</span>
            <h2 id="token-creado" className="dialogo-titular">Cópialo <em>ahora</em>.</h2>
          </div>
        </div>
        <div className="dialogo-cuerpo cuerpo-token">
          <Alerta tipo="error">
            Este token <strong>no se vuelve a mostrar</strong>. Quien lo tenga actúa como tú ({item.scope === "write" ? "puede leer y guardar documentos" : "solo puede leer"}). Si lo pierdes, revócalo y crea otro.
          </Alerta>
          <Codigo titulo="Opción 1 · una línea para Claude Code" texto={comando} />
          <Codigo titulo="Opción 2 · con variable de entorno" texto={conVariable} />
          <p className="label cr-muted">Luego, dentro de Claude Code, escribe <code>/mcp</code> para comprobar que «homelab» sale conectado.</p>
        </div>
        <div className="dialogo-pie">
          <span />
          <Button variant="primary" onClick={alCerrar}>Ya lo guardé</Button>
        </div>
      </Glass>
    </div>,
    document.body,
  );
}
