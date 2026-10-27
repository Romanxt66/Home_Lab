import { useState } from "react";
import type { FormEvent } from "react";

import { api, ApiError } from "../api/client";
import type { ConfigTelegram } from "../api/tipos";
import { Alerta } from "../ui/Alerta";
import { Cabecera } from "../ui/Cabecera";
import { Confirmar } from "../ui/Confirmar";
import { Icono } from "../ui/Iconos";
import { Esqueletos } from "../ui/Vacio";
import { useDatos } from "../ui/useDatos";

/** Telegram: a dónde llegan los avisos del sistema. */
export function Telegram() {
  const { datos: config, error, cargando, recargar } = useDatos<ConfigTelegram | null>("/notifications/config");

  const [chat, setChat] = useState<string | null>(null);
  const [token, setToken] = useState("");
  const [guardando, setGuardando] = useState(false);
  const [probando, setProbando] = useState(false);
  const [aviso, setAviso] = useState<{ tipo: "info" | "error"; texto: string } | null>(null);
  const [desconectando, setDesconectando] = useState(false);
  const [trabajando, setTrabajando] = useState(false);

  // Hasta que se toca el campo se enseña lo que hay guardado.
  const valorChat = chat ?? config?.recipient ?? "";
  const hayConfig = config != null;

  async function guardar(e: FormEvent) {
    e.preventDefault();
    setGuardando(true);
    setAviso(null);
    try {
      await api.post<ConfigTelegram>("/notifications/config", {
        id: config?.id ?? "",
        recipient: valorChat,
        // Vacío significa «conserva el que ya hay»: el token nunca se vuelve a mostrar.
        credential: token,
      });
      setToken("");
      setChat(null);
      setAviso({ tipo: "info", texto: "Guardado. Manda una prueba para comprobar que llega." });
      recargar();
    } catch (err) {
      setAviso({ tipo: "error", texto: err instanceof ApiError ? err.message : "No se pudo guardar." });
    } finally {
      setGuardando(false);
    }
  }

  async function probar() {
    setProbando(true);
    setAviso(null);
    try {
      await api.post("/notifications/test");
      setAviso({ tipo: "info", texto: "Mensaje enviado. Mira tu Telegram." });
    } catch (err) {
      setAviso({ tipo: "error", texto: err instanceof ApiError ? err.message : "No se pudo enviar." });
    } finally {
      setProbando(false);
    }
  }

  async function desconectar() {
    if (!config) return;
    setTrabajando(true);
    try {
      await api.del(`/notifications/config/${config.id}`);
      setDesconectando(false);
      setAviso({ tipo: "info", texto: "Telegram desconectado. Ya no recibirás avisos." });
      recargar();
    } catch (err) {
      setAviso({ tipo: "error", texto: err instanceof Error ? err.message : "No se pudo desconectar." });
      setDesconectando(false);
    } finally {
      setTrabajando(false);
    }
  }

  return (
    <div className="pagina">
      <Cabecera
        etiqueta="telegram"
        titular="Avisos *directos al móvil*."
        entradilla="Los recordatorios del calendario y las reglas de automatización te llegan por aquí."
      />

      {error && <Alerta tipo="error">{error}</Alerta>}
      {aviso && <Alerta tipo={aviso.tipo} alCerrar={() => setAviso(null)}>{aviso.texto}</Alerta>}

      {cargando && !config ? (
        <Esqueletos cantidad={2} />
      ) : (
        <div className="dos-columnas subir" style={{ "--i": 1 } as React.CSSProperties}>
          <form className="cristal panel" onSubmit={guardar}>
            <div className="panel-cab">
              <h2>Conexión</h2>
              <span className={`ficha ficha-estado ${hayConfig ? "tono-acento" : ""}`} style={{ "--tono": hayConfig ? "var(--acento)" : "var(--texto-4)" } as React.CSSProperties}>
                <span className="punto" />
                {hayConfig ? "Conectado" : "Sin conectar"}
              </span>
            </div>

            <div className="campo">
              <label htmlFor="tg-chat">Chat ID</label>
              <input
                id="tg-chat"
                type="text"
                className="entrada-mono"
                value={valorChat}
                onChange={(e) => setChat(e.target.value)}
                placeholder="123456789"
                inputMode="numeric"
                autoComplete="off"
                required
              />
            </div>

            <div className="campo">
              <label htmlFor="tg-token">Token del bot</label>
              <input
                id="tg-token"
                type="password"
                className="entrada-mono"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                placeholder={config?.hasCredential ? "•••••• guardado: déjalo vacío para conservarlo" : "123456:ABC-DEF…"}
                autoComplete="off"
                required={!config?.hasCredential}
              />
              <span className="campo-ayuda">El token se guarda en el servidor y no vuelve a mostrarse.</span>
            </div>

            <div className="acciones-fila">
              <button type="submit" className="btn btn-principal" disabled={guardando}>
                {guardando ? "Guardando…" : "Guardar"}
              </button>
              <button type="button" className="btn" onClick={() => void probar()} disabled={!hayConfig || probando}>
                <Icono nombre="send" tamano={14} />
                {probando ? "Enviando…" : "Enviar prueba"}
              </button>
              {hayConfig && (
                <button type="button" className="btn btn-fantasma btn-peligro-texto" onClick={() => setDesconectando(true)}>
                  Desconectar
                </button>
              )}
            </div>
          </form>

          <aside className="cristal panel" aria-label="Cómo conseguir los datos">
            <h2>Cómo conseguir los datos</h2>
            <ol className="pasos">
              <li>
                En Telegram habla con <b>@BotFather</b> y escribe <code>/newbot</code>. Te dará el <b>token</b>.
              </li>
              <li>
                Abre una conversación con tu bot nuevo y mándale un mensaje cualquiera: sin eso no puede escribirte.
              </li>
              <li>
                Habla con <b>@userinfobot</b>: te responde con tu <b>Chat ID</b>, un número.
              </li>
              <li>Pega los dos aquí, guarda y manda la prueba.</li>
            </ol>
          </aside>
        </div>
      )}

      {desconectando && (
        <Confirmar
          titulo="¿Desconectar Telegram?"
          texto="Dejarás de recibir recordatorios y avisos. Podrás volver a conectarlo cuando quieras."
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
