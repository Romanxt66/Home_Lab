import { useEffect, useState } from "react";

import { api } from "../../api/client";
import type { AccionTarea, CuentaGmail, Tarea, TareaInput } from "../../api/tipos";
import { Dialogo } from "../../ui/Dialogo";
import { PRESETS, describirCron, errorDeCron } from "./cron";
import { Selector } from "../../ui/Selector";

interface Props {
  /** Si viene, se edita; si no, se crea. */
  tarea: Tarea | null;
  guardando: boolean;
  error: string | null;
  alGuardar(input: TareaInput): void;
  alEliminar?(): void;
  alCerrar(): void;
}

export function DialogoTarea({ tarea, guardando, error, alGuardar, alEliminar, alCerrar }: Props) {
  const p = tarea?.params ?? {};

  const [nombre, setNombre] = useState(tarea?.name ?? "");
  const [cron, setCron] = useState(tarea?.cron ?? "0 9 * * *");
  const [accion, setAccion] = useState<AccionTarea>(tarea?.action ?? "email");
  const [activa, setActiva] = useState(tarea?.enabled ?? true);

  // correo
  const [destinatarios, setDestinatarios] = useState(p.recipients ?? "");
  const [asunto, setAsunto] = useState(p.subject ?? "");
  const [cuerpo, setCuerpo] = useState(p.body ?? "");
  const [cuenta, setCuenta] = useState(p.googleAccountId ?? "");

  // scraping
  const [url, setUrl] = useState(p.url ?? "");
  const [selector, setSelector] = useState(p.selector ?? "");
  const [atributo, setAtributo] = useState(p.attribute ?? "");

  // Las cuentas de Gmail conectadas, para elegir desde cuál se envía. Si no
  // se pueden leer no pasa nada: queda el correo del sistema.
  const [cuentas, setCuentas] = useState<CuentaGmail[]>([]);
  useEffect(() => {
    let vigente = true;
    api
      .get<CuentaGmail[]>("/email/accounts")
      .then((c) => {
        if (vigente) setCuentas(c);
      })
      .catch(() => {});
    return () => {
      vigente = false;
    };
  }, []);

  const fallo = errorDeCron(cron);
  const descripcion = fallo ? null : describirCron(cron);

  function enviar() {
    // Se parte de los parámetros que ya tenía y se pisan solo los que este
    // formulario maneja: así no se pierde nada que la tarea llevara de más
    // (por ejemplo, variables de una plantilla).
    const base = { ...(tarea?.params ?? {}) };

    const params =
      accion === "email"
        ? {
            ...base,
            recipients: destinatarios,
            subject: asunto,
            body: cuerpo,
            googleAccountId: cuenta || undefined,
          }
        : {
            ...base,
            url,
            selector,
            attribute: atributo || undefined,
          };

    alGuardar({ name: nombre, cron: cron.trim(), action: accion, params, enabled: activa });
  }

  return (
    <Dialogo
      etiqueta={tarea ? "editar tarea" : "nueva tarea"}
      titular={tarea ? "Ajusta *esta tarea*." : "Automatiza *algo nuevo*."}
      error={error}
      enviando={guardando}
      alEnviar={enviar}
      alCerrar={alCerrar}
      accionIzquierda={
        alEliminar && (
          <button type="button" className="btn btn-fantasma btn-peligro-texto" onClick={alEliminar}>
            Eliminar
          </button>
        )
      }
    >
      <div className="campo">
        <label htmlFor="t-nombre">Nombre</label>
        <input
          id="t-nombre"
          type="text"
          value={nombre}
          onChange={(e) => setNombre(e.target.value)}
          placeholder="Resumen diario, vigilar precios…"
          required
          autoFocus
        />
      </div>

      <div className="campo">
        <label htmlFor="t-cron">Cuándo</label>
        <input
          id="t-cron"
          type="text"
          className="entrada-mono"
          value={cron}
          onChange={(e) => setCron(e.target.value)}
          placeholder="min hora día mes día-semana"
          spellCheck={false}
          autoComplete="off"
          aria-invalid={fallo !== null}
          aria-describedby="t-cron-ayuda"
          required
        />
        <div className="presets" role="group" aria-label="Frecuencias habituales">
          {PRESETS.map((pr) => (
            <button
              key={pr.cron}
              type="button"
              className={`ficha ficha-boton${cron.trim() === pr.cron ? " ficha-acento" : ""}`}
              onClick={() => setCron(pr.cron)}
            >
              {pr.etiqueta}
            </button>
          ))}
        </div>
        <span id="t-cron-ayuda" className={`campo-ayuda${fallo ? " campo-error" : ""}`} aria-live="polite">
          {fallo ?? `Se ejecutará ${descripcion}. Hora del servidor.`}
        </span>
      </div>

      <div className="campo">
        <label htmlFor="t-accion">Qué hace</label>
        <Selector id="t-accion" value={accion} onChange={(e) => setAccion(e.target.value as AccionTarea)}>
          <option value="email">Enviar un correo</option>
          <option value="scrape">Leer una página web</option>
        </Selector>
      </div>

      {accion === "email" ? (
        <>
          <div className="campo">
            <label htmlFor="t-dest">Para</label>
            <textarea
              id="t-dest"
              value={destinatarios}
              onChange={(e) => setDestinatarios(e.target.value)}
              rows={2}
              placeholder="ana@ejemplo.com, luis@ejemplo.com"
              required
            />
            <span className="campo-ayuda">Separa las direcciones con comas, puntos y comas o saltos de línea.</span>
          </div>
          <div className="campo">
            <label htmlFor="t-asunto">Asunto</label>
            <input id="t-asunto" type="text" value={asunto} onChange={(e) => setAsunto(e.target.value)} required />
          </div>
          <div className="campo">
            <label htmlFor="t-cuerpo">Mensaje</label>
            <textarea id="t-cuerpo" value={cuerpo} onChange={(e) => setCuerpo(e.target.value)} rows={4} required />
          </div>
          <div className="campo">
            <label htmlFor="t-cuenta">Enviar desde</label>
            <Selector id="t-cuenta" value={cuenta} onChange={(e) => setCuenta(e.target.value)}>
              <option value="">El correo del sistema</option>
              {cuentas.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.email}
                </option>
              ))}
            </Selector>
          </div>
        </>
      ) : (
        <>
          <div className="campo">
            <label htmlFor="t-url">Dirección</label>
            <input
              id="t-url"
              type="url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://ejemplo.com/precios"
              required
            />
          </div>
          <div className="fila">
            <div className="campo">
              <label htmlFor="t-selector">Selector CSS</label>
              <input
                id="t-selector"
                type="text"
                className="entrada-mono"
                value={selector}
                onChange={(e) => setSelector(e.target.value)}
                placeholder="h1, .precio"
                spellCheck={false}
              />
            </div>
            <div className="campo">
              <label htmlFor="t-atributo">Atributo</label>
              <input
                id="t-atributo"
                type="text"
                className="entrada-mono"
                value={atributo}
                onChange={(e) => setAtributo(e.target.value)}
                placeholder="href (opcional)"
                spellCheck={false}
              />
            </div>
          </div>
          <span className="campo-ayuda" style={{ display: "block", marginBottom: "0.9rem" }}>
            Sin selector se guarda solo el tamaño de la página; con él, lo que encuentre.
          </span>
        </>
      )}

      <div className="campo">
        <label className="casilla">
          <input type="checkbox" checked={activa} onChange={(e) => setActiva(e.target.checked)} />
          Ejecutar automáticamente
        </label>
      </div>
    </Dialogo>
  );
}
