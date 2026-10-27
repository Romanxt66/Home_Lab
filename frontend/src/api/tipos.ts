/** Tipos que devuelve la API. Reflejan los DTO de Go uno a uno. */

export interface Usuario {
  id: string;
  email: string;
  name: string | null;
  role: string;
}

export interface Evento {
  id: string;
  title: string;
  description: string | null;
  start: string;
  end: string | null;
  allDay: boolean;
  location: string | null;
  color: string | null;
  remindMinutesBefore: number | null;
}

/** Lo que se envía al crear o editar un evento. */
export interface EventoInput {
  title: string;
  description: string | null;
  start: string;
  end: string | null;
  allDay: boolean;
  location: string | null;
  remindMinutesBefore: number | null;
}
