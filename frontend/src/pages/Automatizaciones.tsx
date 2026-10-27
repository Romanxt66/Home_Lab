import { useState } from "react";

import { api, ApiError } from "../api/client";
import type { DisparadorRegla, EjecucionRegla, Regla, ReglaInput } from "../api/tipos";
import { Alerta } from "../ui/Alerta";
import { Cabecera } from "../ui/Cabecera";
import { Confirmar } from "../ui/Confirmar";
import { Icono } from "../ui/Iconos";
import { Interruptor } from "../ui/Interruptor";
import { Esqueletos, Vacio } from "../ui/Vacio";
import { useDatos } from "../ui/useDatos";
import { hace } from "../ui/tiempo";
import { DialogoRegla } from "./automatizaciones/DialogoRegla";
import { infoDeAccion, resumenDeAccion } from "./automatizaciones/acciones";

/** Automatizaciones: reglas «cuando pasa esto, haz aquello». */
export function Automatizaciones() {
  const reglas = useDatos<Regla[]>("/automations/rules");
  const disparadores = useDatos<DisparadorRegla[]>("/automations/triggers");

  const [editando, setEditando] = useState<Regla | "nueva" | null>(null);
  const [guardando, setGuardando] = useState(false);
  const [errorDialogo, setErrorDialogo] = useState<string | null>(null);
  const [porBorrar, setPorBorrar] = useState<Regla | null>(null);
  const [borrando, setBorrando] = useState(false);
  const [cambiando, setCambiando] = useState<string | null>(null);
  const [aviso, setAviso] = useState<string | null>(null);

  const lista = reglas.datos ?? [];
  const nombreDelDisparador = (t: string) => disparadores.datos?.find((d) => d.type === t)?.label ?? t;

  async function guardar(input: ReglaInput) {
    setGuardando(true);
    setErrorDialogo(null);
    try {
      if (editando && editando !== "nueva") await api.put(`/automations/rules/${editando.id}`, input);
      else await api.post("/automations/rules", input);
      setEditando(null);
      reglas.recargar();
    } catch (e) {
      setErrorDialogo(e instanceof ApiError ? e.message : "No se pudo guardar.");
    } finally {
      setGuardando(false);
    }
  }

  async function alternar(r: Regla, activa: boolean) {
    setCambiando(r.id);
    setAviso(null);
    try {
      // No hay una ruta solo para encender y apagar: se reenvía la regla entera.
      await api.put(`/automations/rules/${r.id}`, { name: r.name, trigger: r.trigger, actions: r.actions, active: activa });
      reglas.recargar();
    } catch (e) {
      setAviso(e instanceof Error ? e.message : "No se pudo cambiar.");
    } finally {
      setCambiando(null);
    }
  }

  async function borrar() {
    if (!porBorrar) return;
    setBorrando(true);
    try {
      await api.del(`/automations/rules/${porBorrar.id}`);
      reglas.recargar();
      setEditando(null);
    } catch (e) {
      setAviso(e instanceof Error ? e.message : "No se pudo eliminar.");
    } finally {
      setBorrando(false);
      setPorBorrar(null);
    }
  }

  const abrirNueva = () => { setErrorDialogo(null); setEditando("nueva"); };

  return (
    <div className="pagina">
      <Cabecera
        etiqueta="automatizaciones"
        titular="Si pasa esto, *haz aquello*."
        entradilla="Reglas que reaccionan solas a lo que ocurre en el sistema."
        acciones={
          <button className="btn btn-principal" onClick={abrirNueva}>
            <Icono nombre="plus" tamano={16} />
            Nueva regla
          </button>
        }
      />

      {(reglas.error ?? disparadores.error) && <Alerta tipo="error">{reglas.error ?? disparadores.error}</Alerta>}
      {aviso && <Alerta tipo="error" alCerrar={() => setAviso(null)}>{aviso}</Alerta>}

      {reglas.cargando && !reglas.datos ? (
        <Esqueletos cantidad={2} />
      ) : lista.length === 0 ? (
        <Vacio
          icono="bolt"
          titular="Aún no hay *reglas*."
          accion={
            <button className="btn btn-principal" onClick={abrirNueva}>
              <Icono nombre="plus" tamano={16} />
              Crear la primera
            </button>
          }
        >
          Por ejemplo: cuando alguien se registre, avísame por Telegram y crea un evento para revisarlo.
        </Vacio>
      ) : (
        <div className="rejilla">
          {lista.map((r, i) => (
            <TarjetaRegla
              key={r.id}
              regla={r}
              indice={i + 1}
              disparador={nombreDelDisparador(r.trigger)}
              cambiando={cambiando === r.id}
              alAlternar={(a) => void alternar(r, a)}
              alEditar={() => { setErrorDialogo(null); setEditando(r); }}
              alEliminar={() => setPorBorrar(r)}
            />
          ))}
        </div>
      )}

      {editando && disparadores.datos && (
        <DialogoRegla
          key={editando === "nueva" ? "nueva" : editando.id}
          regla={editando === "nueva" ? null : editando}
          disparadores={disparadores.datos}
          guardando={guardando}
          error={errorDialogo}
          alGuardar={(i) => void guardar(i)}
          alEliminar={editando !== "nueva" ? () => setPorBorrar(editando) : undefined}
          alCerrar={() => setEditando(null)}
        />
      )}

      {porBorrar && (
        <Confirmar
          titulo={`¿Eliminar «${porBorrar.name}»?`}
          texto="La regla dejará de ejecutarse y se borrará también su historial."
          etiquetaConfirmar="Eliminar"
          peligro
          trabajando={borrando}
          alConfirmar={() => void borrar()}
          alCancelar={() => setPorBorrar(null)}
        />
      )}
    </div>
  );
}

function TarjetaRegla({ regla: r, indice, disparador, cambiando, alAlternar, alEditar, alEliminar }: {
  regla: Regla;
  indice: number;
  disparador: string;
  cambiando: boolean;
  alAlternar(activa: boolean): void;
  alEditar(): void;
  alEliminar(): void;
}) {
  const [verHistorial, setVerHistorial] = useState(false);
  // Solo se pide el historial cuando se abre: la mayoría de las veces no se mira.
  const historial = useDatos<EjecucionRegla[]>(verHistorial ? `/automations/rules/${r.id}/runs?limit=5` : null);

  return (
    <article
      className={`tarjeta cristal subir tono-${r.active ? "acento" : "aviso"}${r.active ? "" : " atenuada"}`}
      style={{ "--i": indice } as React.CSSProperties}
      aria-label={`${r.name}: ${r.active ? "activa" : "en pausa"}`}
    >
      <div className="tarjeta-cab">
        <span className="tarjeta-avatar"><Icono nombre="bolt" tamano={18} /></span>
        <div className="tarjeta-titulo">
          <span className="tarjeta-nombre" title={r.name}>{r.name}</span>
          <span className="tarjeta-sub">cuando: {disparador}</span>
        </div>
        <div className="tarjeta-iconos">
          <button className="accion" onClick={alEditar} aria-label={`Editar ${r.name}`} title="Editar"><Icono nombre="edit" tamano={16} /></button>
          <button className="accion peligro" onClick={alEliminar} aria-label={`Eliminar ${r.name}`} title="Eliminar"><Icono nombre="trash" tamano={16} /></button>
        </div>
      </div>

      <ol className="pasos-regla">
        {r.actions.map((a, i) => {
          const info = infoDeAccion(a.type);
          return (
            <li key={i}>
              <span className="ficha">
                <Icono nombre={info?.icono ?? "bolt"} tamano={11} />
                {info?.corta ?? a.type}
              </span>
              <span className="paso-texto" title={resumenDeAccion(a)}>{resumenDeAccion(a) || "—"}</span>
            </li>
          );
        })}
      </ol>

      <div className="tarjeta-pie tarjeta-pie-partido">
        <button className="btn btn-pequeno btn-fantasma" onClick={() => setVerHistorial((v) => !v)} aria-expanded={verHistorial}>
          <Icono nombre="clock" tamano={13} />
          {verHistorial ? "Ocultar historial" : "Historial"}
        </button>
        <Interruptor activo={r.active} etiqueta={`Activar la regla «${r.name}»`} deshabilitado={cambiando} alCambiar={alAlternar} />
      </div>

      {verHistorial && (
        <ul className="historial-mini" aria-label="Últimas veces que saltó">
          {historial.cargando && !historial.datos && <li className="texto-apagado">Cargando…</li>}
          {historial.datos?.length === 0 && <li className="texto-apagado">Todavía no ha saltado nunca.</li>}
          {historial.datos?.map((e, i) => (
            <li key={i} className={e.ok ? "" : "con-error"}>
              <span className="ejecucion-punto" aria-label={e.ok ? "bien" : "falló"} />
              <span className="ejecucion-salida" title={e.error ?? ""}>{e.ok ? "Se ejecutó bien" : e.error}</span>
              <span className="ejecucion-cuando">{hace(e.ranAt)}</span>
            </li>
          ))}
        </ul>
      )}
    </article>
  );
}
