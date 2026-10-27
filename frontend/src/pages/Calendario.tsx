import { useEffect, useMemo, useState } from "react";

import { api, ApiError } from "../api/client";
import type { Evento, EventoInput } from "../api/tipos";
import { Cabecera } from "../ui/Cabecera";
import { Confirmar } from "../ui/Confirmar";
import { Icono } from "../ui/Iconos";
import { Resumen } from "../ui/Resumen";
import { DialogoEvento } from "./calendario/DialogoEvento";
import { DIAS, celdasDelMes, diaLargo, hhmm, mismoDia, nombreDelMes, ymd } from "./calendario/fechas";

/** Una sola lista vacía: crear una nueva en cada render haría recalcular todo lo memorizado. */
const SIN_EVENTOS: Evento[] = [];

/** Cuántos eventos caben en una celda antes de decir «+N más». */
const POR_CELDA = 3;

/** Calendario mensual con recordatorios. */
export function Calendario() {
  // «Hoy» se fija al abrir la página: una pestaña que pasa la medianoche abierta
  // se quedará un día atrás hasta recargar, que es mejor que repintar a ciegas.
  const [hoy] = useState(() => new Date());
  const [año, setAño] = useState(hoy.getFullYear());
  const [mes, setMes] = useState(hoy.getMonth()); // 0-11

  // Se guarda con la clave de lo que se pidió: si no coincide con lo que se
  // mira ahora, es que todavía se está cargando. Así no hace falta un estado
  // «cargando» aparte que sincronizar a mano.
  const [datos, setDatos] = useState<{ clave: string; eventos: Evento[] } | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Diálogo: null = cerrado. Al crear lleva el día pulsado; al editar, el evento.
  const [dialogo, setDialogo] = useState<{ evento: Evento | null; dia: Date } | null>(null);
  const [guardando, setGuardando] = useState(false);
  const [errorDialogo, setErrorDialogo] = useState<string | null>(null);

  const [porEliminar, setPorEliminar] = useState<Evento | null>(null);
  const [eliminando, setEliminando] = useState(false);

  // Subir este contador vuelve a pedir el mes: tras guardar o borrar.
  const [version, setVersion] = useState(0);
  const recargar = () => setVersion((v) => v + 1);

  const clave = `${año}-${mes}-${version}`;
  const eventos = datos?.eventos ?? SIN_EVENTOS;
  const cargando = datos?.clave !== clave;

  useEffect(() => {
    // Si se cambia de mes rápido, la respuesta del mes anterior puede llegar
    // después y pisar la buena: la bandera descarta lo que ya no se mira.
    let vigente = true;

    api
      .get<Evento[]>(`/calendar/events?year=${año}&month=${mes + 1}`)
      .then((lista) => {
        if (!vigente) return;
        setDatos({ clave, eventos: lista });
        setError(null);
      })
      .catch((e) => {
        if (!vigente) return;
        setError(e instanceof Error ? e.message : "No se pudo cargar el calendario.");
        // Se marca como cargado, con lo que hubiera, para no quedarse en «cargando».
        setDatos((d) => ({ clave, eventos: d?.eventos ?? [] }));
      });

    return () => {
      vigente = false;
    };
  }, [año, mes, clave]);

  function moverMes(delta: number) {
    const f = new Date(año, mes + delta, 1);
    setAño(f.getFullYear());
    setMes(f.getMonth());
  }

  function irAHoy() {
    const f = new Date();
    setAño(f.getFullYear());
    setMes(f.getMonth());
  }

  async function guardar(input: EventoInput) {
    setGuardando(true);
    setErrorDialogo(null);
    try {
      if (dialogo?.evento) {
        await api.put<Evento>(`/calendar/events/${dialogo.evento.id}`, input);
      } else {
        await api.post<Evento>("/calendar/events", input);
      }
      setDialogo(null);
      recargar();
    } catch (e) {
      // El mensaje de validación viene del dominio en Go y ya está pensado
      // para leerse: se muestra tal cual.
      setErrorDialogo(e instanceof ApiError ? e.message : "No se pudo guardar.");
    } finally {
      setGuardando(false);
    }
  }

  async function eliminar() {
    if (!porEliminar) return;
    setEliminando(true);
    try {
      await api.del(`/calendar/events/${porEliminar.id}`);
      setPorEliminar(null);
      setDialogo(null);
      recargar();
    } catch (e) {
      setError(e instanceof Error ? e.message : "No se pudo eliminar.");
      setPorEliminar(null);
    } finally {
      setEliminando(false);
    }
  }

  const celdas = useMemo(() => celdasDelMes(año, mes), [año, mes]);

  // Eventos por día, con la clave en hora local: un evento a las 23:30 de
  // Bogotá es del día anterior en UTC, y la rejilla debe poner el de Bogotá.
  const porDia = useMemo(() => {
    const mapa = new Map<string, Evento[]>();
    for (const e of eventos) {
      const clave = ymd(new Date(e.start));
      mapa.set(clave, [...(mapa.get(clave) ?? []), e]);
    }
    for (const lista of mapa.values()) {
      lista.sort((a, b) => Number(a.allDay === b.allDay ? 0 : a.allDay ? -1 : 1) || a.start.localeCompare(b.start));
    }
    return mapa;
  }, [eventos]);

  const delMes = useMemo(
    () =>
      eventos.filter((e) => {
        const d = new Date(e.start);
        return d.getFullYear() === año && d.getMonth() === mes;
      }),
    [eventos, año, mes],
  );

  const agenda = useMemo(() => {
    const dias = new Map<string, Evento[]>();
    // Ordenados por hora: la API no garantiza el orden, y una agenda
    // desordenada es peor que ninguna.
    const ordenados = [...delMes].sort((x, y) => x.start.localeCompare(y.start));
    for (const e of ordenados) {
      const clave = ymd(new Date(e.start));
      dias.set(clave, [...(dias.get(clave) ?? []), e]);
    }
    return [...dias.entries()].sort(([a], [b]) => a.localeCompare(b));
  }, [delMes]);

  const abrirNuevo = (dia: Date) => {
    setErrorDialogo(null);
    setDialogo({ evento: null, dia });
  };
  const abrirEditar = (e: Evento) => {
    setErrorDialogo(null);
    setDialogo({ evento: e, dia: new Date(e.start) });
  };

  return (
    <div className="pagina">
      <Cabecera
        etiqueta="calendario"
        titular="Tu tiempo, *bien ordenado*."
        entradilla="Apunta lo que viene y deja que te avisemos a tiempo."
        acciones={
          <button className="btn btn-principal" onClick={() => abrirNuevo(new Date())}>
            <Icono nombre="plus" tamano={16} />
            Nuevo evento
          </button>
        }
      />

      {error && (
        <div className="alerta alerta-error subir" role="alert">
          <Icono nombre="alert" tamano={16} />
          <span>{error}</span>
        </div>
      )}

      <div className="resumenes subir" style={{ "--i": 1 } as React.CSSProperties}>
        <Resumen etiqueta="eventos este mes" valor={delMes.length} icono="calendar" tono="acento" />
        <Resumen
          etiqueta="con recordatorio"
          valor={delMes.filter((e) => e.remindMinutesBefore != null).length}
          icono="bell"
        />
        <Resumen etiqueta="todo el día" valor={delMes.filter((e) => e.allDay).length} icono="clock" />
      </div>

      <section className="cristal calendario subir" style={{ "--i": 2 } as React.CSSProperties} aria-busy={cargando}>
        <div className="calendario-barra">
          <div className="calendario-nav">
            <button className="accion" onClick={() => moverMes(-1)} aria-label="Mes anterior" title="Mes anterior">
              <Icono nombre="chevronL" tamano={18} />
            </button>
            <h2 className="calendario-mes">{nombreDelMes(año, mes)}</h2>
            <button className="accion" onClick={() => moverMes(1)} aria-label="Mes siguiente" title="Mes siguiente">
              <Icono nombre="chevronR" tamano={18} />
            </button>
          </div>
          <button className="btn btn-pequeno" onClick={irAHoy}>
            Hoy
          </button>
        </div>

        <div className="rejilla-mes" role="grid" aria-label={nombreDelMes(año, mes)}>
          {DIAS.map((d) => (
            <div key={d} className="etiqueta dia-nombre" role="columnheader">
              {d}
            </div>
          ))}

          {celdas.map((dia) => {
            const lista = porDia.get(ymd(dia)) ?? [];
            const fuera = dia.getMonth() !== mes;

            return (
              <div
                key={ymd(dia)}
                role="gridcell"
                tabIndex={0}
                className={`celda${fuera ? " fuera" : ""}${mismoDia(dia, hoy) ? " hoy" : ""}`}
                onClick={() => abrirNuevo(dia)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    abrirNuevo(dia);
                  }
                }}
                aria-label={`${dia.toLocaleDateString("es-CO", { day: "numeric", month: "long" })}: ${lista.length} ${lista.length === 1 ? "evento" : "eventos"}`}
              >
                <span className="celda-numero">{dia.getDate()}</span>

                {lista.slice(0, POR_CELDA).map((e) => (
                  <button
                    key={e.id}
                    className={`chip-evento${e.allDay ? " todo-el-dia" : ""}`}
                    onClick={(ev) => {
                      ev.stopPropagation();
                      abrirEditar(e);
                    }}
                    title={e.title}
                  >
                    {!e.allDay && <b>{hhmm(new Date(e.start))}</b>}
                    {e.title}
                  </button>
                ))}

                {lista.length > POR_CELDA && <span className="celda-mas">+{lista.length - POR_CELDA} más</span>}
              </div>
            );
          })}
        </div>
      </section>

      {/* En móvil la rejilla es demasiado estrecha para leerse: se sustituye por una agenda. */}
      <section className="agenda" aria-label="Agenda del mes">
        {agenda.length === 0 && !cargando ? (
          <div className="cristal vacio">
            <span className="vacio-icono">
              <Icono nombre="calendar" tamano={24} />
            </span>
            <h2>
              Un mes <em>despejado</em>.
            </h2>
            <p>No hay nada apuntado en {nombreDelMes(año, mes)}.</p>
            <button className="btn btn-principal" onClick={() => abrirNuevo(new Date(año, mes, 1))}>
              <Icono nombre="plus" tamano={16} />
              Apuntar algo
            </button>
          </div>
        ) : (
          agenda.map(([clave, lista], i) => (
            <div key={clave} className="agenda-dia subir" style={{ "--i": i } as React.CSSProperties}>
              <span className="etiqueta">{diaLargo(new Date(`${clave}T12:00`))}</span>
              {lista.map((e) => (
                <button key={e.id} className="cristal tarjeta-evento" onClick={() => abrirEditar(e)}>
                  <span className="tarjeta-evento-hora">{e.allDay ? "todo el día" : hhmm(new Date(e.start))}</span>
                  <span className="tarjeta-evento-titulo">{e.title}</span>
                  {e.location && (
                    <span className="tarjeta-evento-lugar">
                      <Icono nombre="pin" tamano={12} />
                      {e.location}
                    </span>
                  )}
                </button>
              ))}
            </div>
          ))
        )}
      </section>

      {dialogo && (
        <DialogoEvento
          // La clave fuerza un diálogo nuevo (con su estado inicial limpio)
          // cada vez que se abre, para otro evento o para otro día.
          key={dialogo.evento?.id ?? `nuevo-${ymd(dialogo.dia)}`}
          evento={dialogo.evento}
          diaInicial={dialogo.dia}
          guardando={guardando}
          error={errorDialogo}
          alGuardar={(input) => void guardar(input)}
          alEliminar={dialogo.evento ? () => setPorEliminar(dialogo.evento) : undefined}
          alCerrar={() => {
            setDialogo(null);
            setErrorDialogo(null);
          }}
        />
      )}

      {porEliminar && (
        <Confirmar
          titulo={`¿Eliminar «${porEliminar.title}»?`}
          texto="El evento desaparecerá del calendario y no se enviará su recordatorio."
          etiquetaConfirmar="Eliminar"
          peligro
          trabajando={eliminando}
          alConfirmar={() => void eliminar()}
          alCancelar={() => setPorEliminar(null)}
        />
      )}
    </div>
  );
}
