import type { AccionRegla, TipoAccionRegla } from "../../api/tipos";
import type { NombreIcono } from "../../ui/Iconos";

export const TIPOS_DE_ACCION: { tipo: TipoAccionRegla; etiqueta: string; corta: string; icono: NombreIcono }[] = [
  { tipo: "telegram", etiqueta: "Enviar un mensaje de Telegram", corta: "Telegram", icono: "telegram" },
  { tipo: "calendar_event", etiqueta: "Crear un evento en el calendario", corta: "Calendario", icono: "calendar" },
  { tipo: "n8n_webhook", etiqueta: "Llamar a un webhook", corta: "Webhook", icono: "link" },
];

export function infoDeAccion(tipo: string) {
  return TIPOS_DE_ACCION.find((t) => t.tipo === tipo);
}

/** Una acción vacía del tipo pedido, con los valores de partida sensatos. */
export function accionNueva(tipo: TipoAccionRegla): AccionRegla {
  switch (tipo) {
    case "telegram":
      return { type: tipo, config: { message: "" } };
    case "calendar_event":
      return { type: tipo, config: { title: "", description: "", daysFromNow: "0" } };
    default:
      return { type: tipo, config: { webhookUrl: "" } };
  }
}

/** Una línea que resume la acción, para la tarjeta. */
export function resumenDeAccion(a: AccionRegla): string {
  switch (a.type) {
    case "telegram":
      return a.config.message ?? "";
    case "calendar_event":
      return a.config.title ?? "";
    default:
      return a.config.webhookUrl ?? "";
  }
}
