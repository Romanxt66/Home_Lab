import { useState } from "react";

import type { Evento, EventoInput } from "../../api/tipos";
import { Dialogo } from "../../ui/Dialogo";
import { RECORDATORIOS, hhmm, ymd } from "./fechas";

interface Props {
  /** Si viene, se edita; si no, se crea. */
  evento: Evento | null;
  /** Día con el que arranca un evento nuevo (el que se pulsó en la rejilla). */
  diaInicial: Date;
  guardando: boolean;
  error: string | null;
  alGuardar(input: EventoInput): void;
  alEliminar?(): void;
  alCerrar(): void;
}

export function DialogoEvento({ evento, diaInicial, guardando, error, alGuardar, alEliminar, alCerrar }: Props) {
  // El estado se inicializa una sola vez: el diálogo se monta de nuevo cada
  // vez que se abre, así que no hace falta sincronizarlo con efectos.
  const inicio = evento ? new Date(evento.start) : diaInicial;
  const fin = evento?.end ? new Date(evento.end) : null;

  const [titulo, setTitulo] = useState(evento?.title ?? "");
  const [dia, setDia] = useState(ymd(inicio));
  const [todoElDia, setTodoElDia] = useState(evento?.allDay ?? false);
  const [horaInicio, setHoraInicio] = useState(evento && !evento.allDay ? hhmm(inicio) : "09:00");
  const [horaFin, setHoraFin] = useState(fin ? hhmm(fin) : "10:00");
  const [lugar, setLugar] = useState(evento?.location ?? "");
  const [descripcion, setDescripcion] = useState(evento?.description ?? "");
  const [recordar, setRecordar] = useState(
    evento?.remindMinutesBefore != null ? String(evento.remindMinutesBefore) : "none",
  );

  function enviar() {
    // Se construye con la zona del navegador y se manda en ISO: así «las
    // 9:00» son las 9:00 de quien lo escribe, vaya donde vaya el servidor.
    const comienza = new Date(todoElDia ? `${dia}T00:00` : `${dia}T${horaInicio}`);
    const termina = todoElDia || !horaFin ? null : new Date(`${dia}T${horaFin}`);

    alGuardar({
      title: titulo,
      description: descripcion.trim() || null,
      location: lugar.trim() || null,
      start: comienza.toISOString(),
      end: termina ? termina.toISOString() : null,
      allDay: todoElDia,
      remindMinutesBefore: recordar === "none" ? null : Number(recordar),
    });
  }

  return (
    <Dialogo
      etiqueta={evento ? "editar evento" : "nuevo evento"}
      titular={evento ? "Ajusta *este evento*." : "Apunta *algo nuevo*."}
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
        <label htmlFor="ev-titulo">Título</label>
        <input
          id="ev-titulo"
          type="text"
          value={titulo}
          onChange={(e) => setTitulo(e.target.value)}
          placeholder="Reunión, cumpleaños, entrega…"
          required
          autoFocus
        />
      </div>

      <div className="fila">
        <div className="campo">
          <label htmlFor="ev-dia">Día</label>
          <input id="ev-dia" type="date" value={dia} onChange={(e) => setDia(e.target.value)} required />
        </div>
        <div className="campo campo-casilla">
          <label className="casilla">
            <input type="checkbox" checked={todoElDia} onChange={(e) => setTodoElDia(e.target.checked)} />
            Todo el día
          </label>
        </div>
      </div>

      {!todoElDia && (
        <div className="fila">
          <div className="campo">
            <label htmlFor="ev-ini">Empieza</label>
            <input id="ev-ini" type="time" value={horaInicio} onChange={(e) => setHoraInicio(e.target.value)} required />
          </div>
          <div className="campo">
            <label htmlFor="ev-fin">Termina</label>
            <input id="ev-fin" type="time" value={horaFin} onChange={(e) => setHoraFin(e.target.value)} />
          </div>
        </div>
      )}

      <div className="campo">
        <label htmlFor="ev-lugar">Lugar</label>
        <input id="ev-lugar" type="text" value={lugar} onChange={(e) => setLugar(e.target.value)} placeholder="Opcional" />
      </div>

      <div className="campo">
        <label htmlFor="ev-desc">Descripción</label>
        <textarea
          id="ev-desc"
          value={descripcion}
          onChange={(e) => setDescripcion(e.target.value)}
          rows={3}
          placeholder="Opcional"
        />
      </div>

      <div className="campo">
        <label htmlFor="ev-rec">Recordatorio</label>
        <select id="ev-rec" value={recordar} onChange={(e) => setRecordar(e.target.value)}>
          {RECORDATORIOS.map((r) => (
            <option key={r.valor} value={r.valor}>
              {r.etiqueta}
            </option>
          ))}
        </select>
        {recordar !== "none" && (
          <span className="campo-ayuda">El aviso te llegará por Telegram, si lo tienes configurado.</span>
        )}
      </div>
    </Dialogo>
  );
}
