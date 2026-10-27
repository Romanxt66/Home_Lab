import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";

import { api, ApiError, urlApi } from "../api/client";
import type { CuentaGmail, Plantilla, ResumenEnvio } from "../api/tipos";
import { Alerta } from "../ui/Alerta";
import { Avatar } from "../ui/Avatar";
import { Cabecera } from "../ui/Cabecera";
import { Confirmar } from "../ui/Confirmar";
import { Dialogo } from "../ui/Dialogo";
import { Icono } from "../ui/Iconos";
import { Segmentado } from "../ui/Segmentado";
import { Esqueletos, Vacio } from "../ui/Vacio";
import { useDatos } from "../ui/useDatos";
import { hace } from "../ui/tiempo";

type Pestana = "enviar" | "plantillas" | "cuentas";

/** Los huecos {{variable}} que usa un texto, sin repetir y en orden. */
function variablesDe(texto: string): string[] {
  const vistas = new Set<string>();
  for (const m of texto.matchAll(/\{\{\s*([\w.]+)\s*\}\}/g)) vistas.add(m[1]);
  return [...vistas];
}

/** Lo que dejó el servidor en la dirección al volver de conectar Gmail. */
function avisoDeLaDireccion(): { tipo: "info" | "error"; texto: string } | null {
  const q = new URLSearchParams(window.location.search);
  const conectada = q.get("conectada");
  const error = q.get("error");

  if (conectada) return { tipo: "info", texto: `Cuenta ${conectada} conectada. Ya puedes enviar desde ella.` };
  if (error) return { tipo: "error", texto: error };
  return null;
}

/** Correo: enviar mensajes con plantillas y desde las cuentas de Gmail conectadas. */
export function Correo() {
  const plantillas = useDatos<Plantilla[]>("/email/templates");
  const cuentas = useDatos<CuentaGmail[]>("/email/accounts");

  const [pestana, setPestana] = useState<Pestana>(() => (avisoDeLaDireccion() ? "cuentas" : "enviar"));
  const [aviso, setAviso] = useState(() => avisoDeLaDireccion());

  // Ya se leyó: se limpia la dirección para que recargar no lo repita. Va en
  // un efecto, que corre DESPUÉS de leerla; en un inicializador de estado, el
  // modo estricto lo ejecutaría dos veces y la segunda ya no encontraría nada.
  useEffect(() => {
    if (window.location.search) window.history.replaceState(null, "", window.location.pathname);
  }, []);

  // ── Formulario de envío ───────────────────────────────────────────────
  const [para, setPara] = useState("");
  const [asunto, setAsunto] = useState("");
  const [cuerpo, setCuerpo] = useState("");
  const [desde, setDesde] = useState("");
  const [valores, setValores] = useState<Record<string, string>>({});
  const [enviando, setEnviando] = useState(false);
  const [resultado, setResultado] = useState<{ tipo: "info" | "error"; texto: string } | null>(null);

  const variables = useMemo(() => variablesDe(`${asunto}\n${cuerpo}`), [asunto, cuerpo]);

  // ── Plantillas y cuentas ──────────────────────────────────────────────
  const [editando, setEditando] = useState<Plantilla | "nueva" | null>(null);
  const [guardando, setGuardando] = useState(false);
  const [errorDialogo, setErrorDialogo] = useState<string | null>(null);
  const [plantillaPorBorrar, setPlantillaPorBorrar] = useState<Plantilla | null>(null);
  const [cuentaPorQuitar, setCuentaPorQuitar] = useState<CuentaGmail | null>(null);
  const [borrando, setBorrando] = useState(false);

  async function enviar(e: FormEvent) {
    e.preventDefault();
    setEnviando(true);
    setResultado(null);
    try {
      const r = await api.post<ResumenEnvio>("/email/send", {
        recipients: para,
        subject: asunto,
        body: cuerpo,
        variables: valores,
        googleAccountId: desde,
      });
      setResultado({
        tipo: "info",
        texto: r.failed > 0 ? `Enviados ${r.sent}; fallaron ${r.failed}. Revisa el registro de envíos.` : `Enviado a ${r.sent} ${r.sent === 1 ? "destinatario" : "destinatarios"}.`,
      });
    } catch (err) {
      setResultado({ tipo: "error", texto: err instanceof ApiError ? err.message : "No se pudo enviar." });
    } finally {
      setEnviando(false);
    }
  }

  function usarPlantilla(p: Plantilla) {
    setAsunto(p.subject);
    setCuerpo(p.body);
    setValores({});
    setResultado(null);
    setPestana("enviar");
  }

  async function guardarPlantilla(datos: { name: string; subject: string; body: string }) {
    setGuardando(true);
    setErrorDialogo(null);
    try {
      if (editando && editando !== "nueva") await api.put(`/email/templates/${editando.id}`, datos);
      else await api.post("/email/templates", datos);
      setEditando(null);
      plantillas.recargar();
    } catch (err) {
      setErrorDialogo(err instanceof ApiError ? err.message : "No se pudo guardar.");
    } finally {
      setGuardando(false);
    }
  }

  async function borrar() {
    setBorrando(true);
    try {
      if (plantillaPorBorrar) {
        await api.del(`/email/templates/${plantillaPorBorrar.id}`);
        plantillas.recargar();
      }
      if (cuentaPorQuitar) {
        await api.del(`/email/accounts/${cuentaPorQuitar.id}`);
        if (desde === cuentaPorQuitar.id) setDesde("");
        cuentas.recargar();
      }
    } catch (err) {
      setAviso({ tipo: "error", texto: err instanceof Error ? err.message : "No se pudo borrar." });
    } finally {
      setBorrando(false);
      setPlantillaPorBorrar(null);
      setCuentaPorQuitar(null);
    }
  }

  const listaPlantillas = plantillas.datos ?? [];
  const listaCuentas = cuentas.datos ?? [];

  return (
    <div className="pagina">
      <Cabecera
        etiqueta="correo"
        titular="Escribe una vez, *envía a todos*."
        entradilla="Plantillas con huecos para rellenar, y envío desde tu propio Gmail o desde el correo del sistema."
      />

      {aviso && <Alerta tipo={aviso.tipo} alCerrar={() => setAviso(null)}>{aviso.texto}</Alerta>}
      {(plantillas.error || cuentas.error) && <Alerta tipo="error">{plantillas.error ?? cuentas.error}</Alerta>}

      <div className="subir" style={{ "--i": 1 } as React.CSSProperties}>
        <Segmentado<Pestana>
          etiqueta="Secciones del correo"
          valor={pestana}
          alCambiar={setPestana}
          opciones={[
            { valor: "enviar", etiqueta: "Enviar" },
            { valor: "plantillas", etiqueta: `Plantillas · ${listaPlantillas.length}` },
            { valor: "cuentas", etiqueta: `Cuentas · ${listaCuentas.length}` },
          ]}
        />
      </div>

      {pestana === "enviar" && (
        <form className="cristal panel subir" style={{ "--i": 2 } as React.CSSProperties} onSubmit={enviar}>
          {listaPlantillas.length > 0 && (
            <div className="campo">
              <label htmlFor="co-plantilla">Partir de una plantilla</label>
              <select
                id="co-plantilla"
                value=""
                onChange={(e) => {
                  const p = listaPlantillas.find((x) => x.id === e.target.value);
                  if (p) usarPlantilla(p);
                }}
              >
                <option value="">Elige una…</option>
                {listaPlantillas.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
            </div>
          )}

          <div className="campo">
            <label htmlFor="co-para">Para</label>
            <textarea
              id="co-para"
              value={para}
              onChange={(e) => setPara(e.target.value)}
              rows={2}
              placeholder="ana@ejemplo.com, luis@ejemplo.com"
              required
            />
            <span className="campo-ayuda">Se manda un mensaje a cada uno: nadie ve las direcciones de los demás.</span>
          </div>

          <div className="campo">
            <label htmlFor="co-asunto">Asunto</label>
            <input id="co-asunto" type="text" value={asunto} onChange={(e) => setAsunto(e.target.value)} required />
          </div>

          <div className="campo">
            <label htmlFor="co-cuerpo">Mensaje</label>
            <textarea id="co-cuerpo" value={cuerpo} onChange={(e) => setCuerpo(e.target.value)} rows={7} required />
            <span className="campo-ayuda">
              Usa <span className="codigo-en-linea">{"{{nombre}}"}</span> para dejar un hueco que rellenar.
            </span>
          </div>

          {variables.length > 0 && (
            <div className="fila">
              {variables.map((v) => (
                <div className="campo" key={v}>
                  <label htmlFor={`co-var-${v}`}>{v}</label>
                  <input
                    id={`co-var-${v}`}
                    type="text"
                    value={valores[v] ?? ""}
                    onChange={(e) => setValores((o) => ({ ...o, [v]: e.target.value }))}
                  />
                </div>
              ))}
            </div>
          )}

          <div className="campo">
            <label htmlFor="co-desde">Enviar desde</label>
            <select id="co-desde" value={desde} onChange={(e) => setDesde(e.target.value)}>
              <option value="">El correo del sistema</option>
              {listaCuentas.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.email}
                </option>
              ))}
            </select>
          </div>

          {resultado && <Alerta tipo={resultado.tipo} alCerrar={() => setResultado(null)}>{resultado.texto}</Alerta>}

          <div className="acciones-fila">
            <button type="submit" className="btn btn-principal" disabled={enviando}>
              <Icono nombre="send" tamano={15} />
              {enviando ? "Enviando…" : "Enviar"}
            </button>
          </div>
        </form>
      )}

      {pestana === "plantillas" &&
        (plantillas.cargando && !plantillas.datos ? (
          <Esqueletos cantidad={2} />
        ) : listaPlantillas.length === 0 ? (
          <Vacio
            icono="mail"
            titular="Aún no tienes *plantillas*."
            accion={
              <button className="btn btn-principal" onClick={() => { setErrorDialogo(null); setEditando("nueva"); }}>
                <Icono nombre="plus" tamano={16} />
                Crear la primera
              </button>
            }
          >
            Una plantilla guarda el asunto y el mensaje con huecos, para no escribir lo mismo cada vez.
          </Vacio>
        ) : (
          <>
            <div className="acciones-fila subir">
              <button className="btn btn-principal" onClick={() => { setErrorDialogo(null); setEditando("nueva"); }}>
                <Icono nombre="plus" tamano={16} />
                Nueva plantilla
              </button>
            </div>
            <div className="rejilla">
              {listaPlantillas.map((p, i) => (
                <article key={p.id} className="tarjeta cristal subir" style={{ "--i": i + 2 } as React.CSSProperties}>
                  <div className="tarjeta-cab">
                    <span className="tarjeta-avatar"><Icono nombre="mail" tamano={18} /></span>
                    <div className="tarjeta-titulo">
                      <span className="tarjeta-nombre" title={p.name}>{p.name}</span>
                      <span className="tarjeta-sub" title={p.subject}>{p.subject}</span>
                    </div>
                    <div className="tarjeta-iconos">
                      <button className="accion" onClick={() => { setErrorDialogo(null); setEditando(p); }} aria-label={`Editar ${p.name}`} title="Editar">
                        <Icono nombre="edit" tamano={16} />
                      </button>
                      <button className="accion peligro" onClick={() => setPlantillaPorBorrar(p)} aria-label={`Eliminar ${p.name}`} title="Eliminar">
                        <Icono nombre="trash" tamano={16} />
                      </button>
                    </div>
                  </div>
                  <p className="salida">{p.body}</p>
                  <div className="fichas">
                    {p.variables.map((v) => (<span key={v} className="ficha">{`{{${v}}}`}</span>))}
                    <span className="ficha">{hace(p.updatedAt)}</span>
                  </div>
                  <div className="tarjeta-pie">
                    <button className="btn btn-pequeno" onClick={() => usarPlantilla(p)}>
                      <Icono nombre="send" tamano={13} />
                      Usar
                    </button>
                  </div>
                </article>
              ))}
            </div>
          </>
        ))}

      {pestana === "cuentas" &&
        (cuentas.cargando && !cuentas.datos ? (
          <Esqueletos cantidad={2} />
        ) : (
          <>
            <div className="acciones-fila subir">
              {/* Un enlace y no un botón: Google redirige, no hay nada que pedir con fetch. */}
              <a className="btn btn-principal" href={urlApi("/email/google/start")}>
                <Icono nombre="link" tamano={15} />
                Conectar una cuenta de Google
              </a>
            </div>

            {listaCuentas.length === 0 ? (
              <Vacio icono="user" titular="Ninguna cuenta *conectada*.">
                Conecta tu Gmail para enviar desde tu propia dirección. Solo se pide permiso para enviar, nunca para leer tu correo.
              </Vacio>
            ) : (
              <div className="rejilla">
                {listaCuentas.map((c, i) => (
                  <article key={c.id} className="tarjeta cristal subir" style={{ "--i": i + 2 } as React.CSSProperties}>
                    <div className="tarjeta-cab">
                      <Avatar nombre={c.name ?? c.email} imagen={c.picture} tamano={42} />
                      <div className="tarjeta-titulo">
                        <span className="tarjeta-nombre">{c.name ?? c.email}</span>
                        {c.name && <span className="tarjeta-sub">{c.email}</span>}
                      </div>
                      <button className="accion peligro" onClick={() => setCuentaPorQuitar(c)} aria-label={`Desconectar ${c.email}`} title="Desconectar">
                        <Icono nombre="trash" tamano={16} />
                      </button>
                    </div>
                  </article>
                ))}
              </div>
            )}
          </>
        ))}

      {editando && (
        <DialogoPlantilla
          key={editando === "nueva" ? "nueva" : editando.id}
          plantilla={editando === "nueva" ? null : editando}
          guardando={guardando}
          error={errorDialogo}
          alGuardar={(d) => void guardarPlantilla(d)}
          alCerrar={() => setEditando(null)}
        />
      )}

      {(plantillaPorBorrar || cuentaPorQuitar) && (
        <Confirmar
          titulo={plantillaPorBorrar ? `¿Eliminar «${plantillaPorBorrar.name}»?` : `¿Desconectar ${cuentaPorQuitar?.email}?`}
          texto={
            plantillaPorBorrar
              ? "La plantilla se borrará. Los mensajes ya enviados no se ven afectados."
              : "Dejarás de poder enviar desde esa cuenta. Las tareas que la usen volverán a enviar con el correo del sistema."
          }
          etiquetaConfirmar={plantillaPorBorrar ? "Eliminar" : "Desconectar"}
          peligro
          trabajando={borrando}
          alConfirmar={() => void borrar()}
          alCancelar={() => { setPlantillaPorBorrar(null); setCuentaPorQuitar(null); }}
        />
      )}
    </div>
  );
}

function DialogoPlantilla({ plantilla, guardando, error, alGuardar, alCerrar }: {
  plantilla: Plantilla | null;
  guardando: boolean;
  error: string | null;
  alGuardar(d: { name: string; subject: string; body: string }): void;
  alCerrar(): void;
}) {
  const [nombre, setNombre] = useState(plantilla?.name ?? "");
  const [asunto, setAsunto] = useState(plantilla?.subject ?? "");
  const [cuerpo, setCuerpo] = useState(plantilla?.body ?? "");
  const huecos = variablesDe(`${asunto}\n${cuerpo}`);

  return (
    <Dialogo
      etiqueta={plantilla ? "editar plantilla" : "nueva plantilla"}
      titular={plantilla ? "Ajusta *esta plantilla*." : "Escribe *una vez*."}
      error={error}
      enviando={guardando}
      alEnviar={() => alGuardar({ name: nombre, subject: asunto, body: cuerpo })}
      alCerrar={alCerrar}
    >
      <div className="campo">
        <label htmlFor="pl-nombre">Nombre</label>
        <input id="pl-nombre" type="text" value={nombre} onChange={(e) => setNombre(e.target.value)} required autoFocus />
      </div>
      <div className="campo">
        <label htmlFor="pl-asunto">Asunto</label>
        <input id="pl-asunto" type="text" value={asunto} onChange={(e) => setAsunto(e.target.value)} required />
      </div>
      <div className="campo">
        <label htmlFor="pl-cuerpo">Mensaje</label>
        <textarea id="pl-cuerpo" value={cuerpo} onChange={(e) => setCuerpo(e.target.value)} rows={7} required />
        <span className="campo-ayuda">
          {huecos.length > 0 ? `Huecos detectados: ${huecos.join(", ")}.` : "Usa {{nombre}} para dejar un hueco que rellenar al enviar."}
        </span>
      </div>
    </Dialogo>
  );
}
