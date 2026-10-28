import { useState } from "react";

import type { AccionRegla, DisparadorRegla, Regla, ReglaInput, TipoAccionRegla } from "../../api/tipos";
import { Dialogo } from "../../ui/Dialogo";
import { Icono } from "../../ui/Iconos";
import { TIPOS_DE_ACCION, accionNueva } from "./acciones";
import { Selector } from "../../ui/Selector";

interface Props {
  regla: Regla | null;
  disparadores: DisparadorRegla[];
  guardando: boolean;
  error: string | null;
  alGuardar(input: ReglaInput): void;
  alEliminar?(): void;
  alCerrar(): void;
}

export function DialogoRegla({ regla, disparadores, guardando, error, alGuardar, alEliminar, alCerrar }: Props) {
  const [nombre, setNombre] = useState(regla?.name ?? "");
  const [disparador, setDisparador] = useState(regla?.trigger ?? disparadores[0]?.type ?? "");
  const [activa, setActiva] = useState(regla?.active ?? true);
  const [acciones, setAcciones] = useState<AccionRegla[]>(regla?.actions ?? [accionNueva("telegram")]);

  const info = disparadores.find((d) => d.type === disparador);
  // Una regla heredada puede tener un disparador que ya no existe: se deja
  // elegirlo (para no cambiárselo a escondidas) y el servidor explicará por qué no vale.
  const esHeredado = regla != null && !disparadores.some((d) => d.type === regla.trigger);

  const cambiar = (i: number, clave: string, valor: string) =>
    setAcciones((l) => l.map((a, j) => (j === i ? { ...a, config: { ...a.config, [clave]: valor } } : a)));

  const cambiarTipo = (i: number, tipo: TipoAccionRegla) =>
    setAcciones((l) => l.map((a, j) => (j === i ? accionNueva(tipo) : a)));

  const mover = (i: number, delta: number) =>
    setAcciones((l) => {
      const destino = i + delta;
      if (destino < 0 || destino >= l.length) return l;
      const copia = [...l];
      [copia[i], copia[destino]] = [copia[destino], copia[i]];
      return copia;
    });

  return (
    <Dialogo
      etiqueta={regla ? "editar regla" : "nueva regla"}
      titular={regla ? "Ajusta *esta regla*." : "Cuando pase *algo*…"}
      error={error}
      enviando={guardando}
      alEnviar={() => alGuardar({ name: nombre, trigger: disparador, active: activa, actions: acciones })}
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
        <label htmlFor="rg-nombre">Nombre</label>
        <input id="rg-nombre" type="text" value={nombre} onChange={(e) => setNombre(e.target.value)} placeholder="Avisarme de cada registro nuevo" required autoFocus />
      </div>

      <div className="campo">
        <label htmlFor="rg-disp">Cuando</label>
        <Selector id="rg-disp" value={disparador} onChange={(e) => setDisparador(e.target.value)}>
          {esHeredado && <option value={regla?.trigger}>{regla?.trigger} (ya no disponible)</option>}
          {disparadores.map((d) => (
            <option key={d.type} value={d.type}>
              {d.label}
            </option>
          ))}
        </Selector>
        {info && (
          <>
            <span className="campo-ayuda">{info.description}</span>
            <div className="fichas" aria-label="Variables disponibles">
              {info.variables.map((v) => (
                <span key={v} className="ficha">{`{{${v}}}`}</span>
              ))}
            </div>
          </>
        )}
      </div>

      <div className="acciones-lista">
        <span className="etiqueta">entonces, en este orden</span>

        {acciones.map((a, i) => (
          <fieldset key={i} className="accion-bloque">
            <legend className="ocultar-visualmente">Acción {i + 1}</legend>
            <div className="accion-bloque-cab">
              <span className="ficha">{i + 1}</span>
              <Selector
                aria-label={`Tipo de la acción ${i + 1}`}
                value={a.type}
                onChange={(e) => cambiarTipo(i, e.target.value as TipoAccionRegla)}
              >
                {TIPOS_DE_ACCION.map((t) => (
                  <option key={t.tipo} value={t.tipo}>
                    {t.etiqueta}
                  </option>
                ))}
              </Selector>
              <button type="button" className="accion" onClick={() => mover(i, -1)} disabled={i === 0} aria-label="Subir" title="Subir">
                <Icono nombre="chevronL" tamano={15} style={{ transform: "rotate(90deg)" }} />
              </button>
              <button type="button" className="accion" onClick={() => mover(i, 1)} disabled={i === acciones.length - 1} aria-label="Bajar" title="Bajar">
                <Icono nombre="chevronR" tamano={15} style={{ transform: "rotate(90deg)" }} />
              </button>
              <button
                type="button"
                className="accion peligro"
                onClick={() => setAcciones((l) => l.filter((_, j) => j !== i))}
                aria-label={`Quitar la acción ${i + 1}`}
                title="Quitar"
              >
                <Icono nombre="trash" tamano={15} />
              </button>
            </div>

            {a.type === "telegram" && (
              <div className="campo">
                <label htmlFor={`rg-m-${i}`}>Mensaje</label>
                <textarea id={`rg-m-${i}`} rows={3} value={a.config.message ?? ""} onChange={(e) => cambiar(i, "message", e.target.value)} placeholder="🆕 {{nombre}} se ha registrado" />
              </div>
            )}

            {a.type === "calendar_event" && (
              <>
                <div className="campo">
                  <label htmlFor={`rg-t-${i}`}>Título</label>
                  <input id={`rg-t-${i}`} type="text" value={a.config.title ?? ""} onChange={(e) => cambiar(i, "title", e.target.value)} placeholder="Revisar a {{nombre}}" />
                </div>
                <div className="fila">
                  <div className="campo">
                    <label htmlFor={`rg-d-${i}`}>Descripción</label>
                    <input id={`rg-d-${i}`} type="text" value={a.config.description ?? ""} onChange={(e) => cambiar(i, "description", e.target.value)} />
                  </div>
                  <div className="campo">
                    <label htmlFor={`rg-n-${i}`}>Días desde hoy</label>
                    <input id={`rg-n-${i}`} type="number" min={0} value={a.config.daysFromNow ?? "0"} onChange={(e) => cambiar(i, "daysFromNow", e.target.value)} />
                  </div>
                </div>
              </>
            )}

            {a.type === "n8n_webhook" && (
              <div className="campo">
                <label htmlFor={`rg-w-${i}`}>Dirección del webhook</label>
                <input id={`rg-w-${i}`} type="url" value={a.config.webhookUrl ?? ""} onChange={(e) => cambiar(i, "webhookUrl", e.target.value)} placeholder="https://n8n.midominio.com/webhook/…" />
              </div>
            )}
          </fieldset>
        ))}

        <button type="button" className="btn btn-pequeno" onClick={() => setAcciones((l) => [...l, accionNueva("telegram")])}>
          <Icono nombre="plus" tamano={14} />
          Añadir otra acción
        </button>
        <span className="campo-ayuda">Si una falla, las siguientes no se ejecutan.</span>
      </div>

      <div className="campo" style={{ marginTop: "0.9rem" }}>
        <label className="casilla">
          <input type="checkbox" checked={activa} onChange={(e) => setActiva(e.target.checked)} />
          Regla activa
        </label>
      </div>
    </Dialogo>
  );
}
