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

export type RolUsuario = "user" | "superadmin";
export type EstadoUsuario = "pending" | "approved" | "rejected";

/** Una cuenta tal como la ve la administración. Nunca lleva la contraseña. */
export interface UsuarioAdmin {
  id: string;
  email: string;
  name: string | null;
  picture: string | null;
  role: RolUsuario;
  status: EstadoUsuario;
  /** Las cuentas que solo entran con Google no tienen contraseña. */
  authMethod: "google" | "password";
  createdAt: string;
  /** Es la cuenta de quien mira: no se puede rechazar ni cambiar de rol. */
  isSelf: boolean;
}

export type AccionTarea = "email" | "scrape";

/** Parámetros de una tarea. Cada acción espera los suyos; el resto se conserva. */
export interface ParametrosTarea {
  // email
  recipients?: string;
  subject?: string;
  body?: string;
  googleAccountId?: string;
  // scrape
  url?: string;
  selector?: string;
  attribute?: string;
  [otro: string]: unknown;
}

export interface Tarea {
  id: string;
  name: string;
  cron: string;
  action: AccionTarea;
  params: ParametrosTarea;
  enabled: boolean;
  createdAt: string;
}

export interface TareaInput {
  name: string;
  cron: string;
  action: AccionTarea;
  params: ParametrosTarea;
  enabled: boolean;
}

export interface EjecucionTarea {
  id: string;
  jobId: string;
  jobName: string;
  status: "success" | "error";
  output: string | null;
  startedAt: string;
}

export interface CuentaGmail {
  id: string;
  email: string;
  name: string | null;
  picture: string | null;
}
