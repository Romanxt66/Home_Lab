import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { useNavigate } from "react-router-dom";

import { api, ApiError } from "../api/client";
import type { MensajeAsistente, RespuestaAsistente } from "../api/tipos";
import { Icono } from "./Iconos";

interface Burbuja extends MensajeAsistente {
  /** Herramientas que usó para responder: se enseñan para que no sea una caja negra. */
  herramientas?: string[];
  error?: boolean;
}

const SUGERENCIAS = ["¿Qué tengo en el calendario esta semana?", "¿Cómo están mis aplicaciones en Coolify?", "Lista mis repositorios de GitHub"];

/** El nombre de una herramienta, legible: «list_events» → «list events». */
const legible = (h: string) => h.replaceAll("_", " ");

/** Asistente flotante: una conversación con acceso a las herramientas del panel. */
export function Asistente() {
  const navegar = useNavigate();
  const [abierto, setAbierto] = useState(false);
  const [mensajes, setMensajes] = useState<Burbuja[]>([]);
  const [texto, setTexto] = useState("");
  const [pensando, setPensando] = useState(false);
  const fin = useRef<HTMLDivElement>(null);
  const entrada = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    fin.current?.scrollIntoView({ block: "end" });
  }, [mensajes, pensando, abierto]);

  useEffect(() => {
    if (!abierto) return;
    entrada.current?.focus();
    const cerrar = (e: KeyboardEvent) => e.key === "Escape" && setAbierto(false);
    window.addEventListener("keydown", cerrar);
    return () => window.removeEventListener("keydown", cerrar);
  }, [abierto]);

  async function enviar(mensaje: string) {
    const limpio = mensaje.trim();
    if (!limpio || pensando) return;

    // El servidor no guarda la conversación: se le manda entera cada vez,
    // sin los errores ni la pregunta que los causó: no se llegó a hablar de eso,
    // y dos mensajes seguidos del mismo rol descolocan al modelo.
    const historial = mensajes
      .filter((m, i) => !m.error && !mensajes[i + 1]?.error)
      .map(({ role, content }) => ({ role, content }));
    setMensajes((m) => [...m, { role: "user", content: limpio }]);
    setTexto("");
    setPensando(true);

    try {
      const r = await api.post<RespuestaAsistente>("/assistant/chat", { history: historial, message: limpio });
      setMensajes((m) => [...m, { role: "assistant", content: r.reply, herramientas: r.toolsUsed }]);
      if (r.navigateTo) navegar(`/${r.navigateTo}`);
    } catch (e) {
      const texto = e instanceof ApiError ? e.message : "No se pudo contactar con el asistente.";
      setMensajes((m) => [...m, { role: "assistant", content: texto, error: true }]);
    } finally {
      setPensando(false);
    }
  }

  function alEnviar(e: FormEvent) {
    e.preventDefault();
    void enviar(texto);
  }

  return (
    <>
      <button
        className={`asistente-boton${abierto ? " abierto" : ""}`}
        onClick={() => setAbierto((a) => !a)}
        aria-label={abierto ? "Cerrar el asistente" : "Abrir el asistente"}
        aria-expanded={abierto}
        aria-controls="asistente-panel"
      >
        <Icono nombre={abierto ? "close" : "chat"} tamano={22} />
      </button>

      {abierto && (
        <section id="asistente-panel" className="asistente-panel cristal" role="dialog" aria-label="Asistente">
          <header className="asistente-cab">
            <div>
              <span className="etiqueta">asistente</span>
              <h2>¿En qué <em>te ayudo</em>?</h2>
            </div>
            {mensajes.length > 0 && (
              <button className="btn btn-pequeno btn-fantasma" onClick={() => setMensajes([])} disabled={pensando}>
                Nueva
              </button>
            )}
          </header>

          <div className="asistente-hilo" role="log" aria-live="polite">
            {mensajes.length === 0 && (
              <div className="asistente-vacio">
                <p className="texto-apagado">Pregúntame por el calendario, tus servidores, tus bases o tus repositorios. Si te lo pido, también puedo llevarte a la pantalla que necesites.</p>
                <div className="fichas">
                  {SUGERENCIAS.map((s) => (
                    <button key={s} className="ficha ficha-boton" onClick={() => void enviar(s)}>{s}</button>
                  ))}
                </div>
              </div>
            )}

            {mensajes.map((m, i) => (
              <div key={i} className={`burbuja ${m.role === "user" ? "burbuja-yo" : "burbuja-ia"}${m.error ? " burbuja-error" : ""}`}>
                <p>{m.content}</p>
                {m.herramientas && m.herramientas.length > 0 && (
                  <span className="burbuja-herramientas">
                    <Icono nombre="bolt" tamano={11} />
                    {[...new Set(m.herramientas)].map(legible).join(" · ")}
                  </span>
                )}
              </div>
            ))}

            {pensando && (
              <div className="burbuja burbuja-ia" role="status" aria-label="Pensando">
                <span className="puntos"><i /><i /><i /></span>
              </div>
            )}
            <div ref={fin} />
          </div>

          <form className="asistente-pie" onSubmit={alEnviar}>
            <textarea
              ref={entrada}
              rows={1}
              value={texto}
              onChange={(e) => setTexto(e.target.value)}
              onKeyDown={(e) => {
                // Intro envía; Mayús + Intro baja de línea.
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  void enviar(texto);
                }
              }}
              placeholder="Escribe un mensaje…"
              aria-label="Mensaje"
            />
            <button type="submit" className="btn btn-principal btn-icono" disabled={pensando || !texto.trim()} aria-label="Enviar">
              <Icono nombre="send" tamano={16} />
            </button>
          </form>
        </section>
      )}
    </>
  );
}
