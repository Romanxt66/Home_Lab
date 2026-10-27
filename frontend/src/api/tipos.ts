/** Tipos que devuelve la API. Reflejan los DTO de Go uno a uno. */

export type MonitorStatus = "up" | "down" | "unknown";
export type HttpMethod = "GET" | "HEAD";

export interface Monitor {
  id: string;
  name: string;
  url: string;
  method: HttpMethod;
  /** 0 = vale cualquier 2xx o 3xx. */
  expectedStatus: number;
  intervalSeconds: number;
  timeoutMs: number;
  active: boolean;
  notifyOnFailure: boolean;
  lastStatus: MonitorStatus;
  lastCheckedAt: string | null;
  lastResponseMs: number | null;
  lastStatusCode: number | null;
  lastError: string | null;
  consecutiveFailures: number;
}

export interface Check {
  ok: boolean;
  statusCode: number | null;
  responseMs: number | null;
  error: string | null;
  checkedAt: string;
}

export interface MonitorDetail {
  monitor: Monitor;
  checks: Check[];
  uptime24h: number;
}

/** Los campos que se envían al crear o editar. */
export interface MonitorInput {
  name: string;
  url: string;
  method: HttpMethod;
  expectedStatus: number;
  intervalSeconds: number;
  timeoutMs: number;
  active: boolean;
  notifyOnFailure: boolean;
}

export interface Usuario {
  id: string;
  email: string;
  name: string | null;
  role: string;
}

export const ETIQUETAS_ESTADO: Record<MonitorStatus, string> = {
  up: "En línea",
  down: "Caído",
  unknown: "Sin datos",
};
