import { useSesion } from "./api/sesion";
import type { Seccion } from "./ui/Navegacion";

export interface SeccionDelPanel extends Seccion {
  /** Qué se hace ahí, en una línea: es lo que lee quien aún no la conoce. */
  detalle: string;
  soloAdmin?: boolean;
}

/** Las secciones del panel. Añadir un módulo es añadir una línea aquí. */
export const SECCIONES: SeccionDelPanel[] = [
  { ruta: "/", etiqueta: "Inicio", corta: "inicio", icono: "home", detalle: "Tu panel de herramientas." },
  { ruta: "/calendar", etiqueta: "Calendario", corta: "agenda", icono: "calendar", detalle: "Eventos y recordatorios que te avisan por Telegram." },
  { ruta: "/automations", etiqueta: "Automatizaciones", corta: "reglas", icono: "bolt", detalle: "Reglas «cuando pasa esto, haz aquello»." },
  { ruta: "/coolify", etiqueta: "Coolify", corta: "servidor", icono: "server", detalle: "Aplicaciones, bases y despliegues de tu servidor." },
  { ruta: "/db-admin", etiqueta: "Bases de datos", corta: "datos", icono: "database", detalle: "Explora tablas, edita filas y lanza consultas." },
  { ruta: "/email-automation", etiqueta: "Correo", corta: "correo", icono: "mail", detalle: "Envía correos con plantillas desde tus cuentas de Gmail." },
  { ruta: "/github", etiqueta: "GitHub", corta: "github", icono: "git", detalle: "Tus repositorios con búsqueda, orden y filtros." },
  { ruta: "/telegram", etiqueta: "Telegram", corta: "avisos", icono: "telegram", detalle: "A dónde llegan los avisos del sistema." },
  { ruta: "/scraper", etiqueta: "Scraper", corta: "scraper", icono: "code", detalle: "Extrae de una página justo lo que señale un selector." },
  { ruta: "/scheduler", etiqueta: "Tareas programadas", corta: "tareas", icono: "clock", detalle: "Trabajos que se ejecutan solos con su horario." },
  { ruta: "/users", etiqueta: "Usuarios", corta: "gente", icono: "shield", detalle: "Quién entra al panel y con qué permisos.", soloAdmin: true },
];

/** Las secciones que ve quien ha entrado: las de administración, solo el administrador. */
export function useSecciones() {
  const { usuario } = useSesion();
  return SECCIONES.filter((s) => !s.soloAdmin || usuario?.role === "superadmin");
}
