import { useState } from "react";

import { api, ApiError } from "../api/client";
import type { ConexionDb } from "../api/tipos";
import { Alerta } from "../ui/Alerta";
import { Cabecera } from "../ui/Cabecera";
import { Confirmar } from "../ui/Confirmar";
import { Icono } from "../ui/Iconos";
import { Esqueletos, Vacio } from "../ui/Vacio";
import { useDatos } from "../ui/useDatos";
import { DialogoConexion } from "./dbadmin/DialogoConexion";
import { Explorador } from "./dbadmin/Explorador";

/** Administrador de bases de datos PostgreSQL: conexiones guardadas y un explorador. */
export function DbAdmin() {
  const conexiones = useDatos<ConexionDb[]>("/db-admin/connections");
  const [abierta, setAbierta] = useState<string | null>(null);
  const [editando, setEditando] = useState<ConexionDb | "nueva" | null>(null);
  const [guardando, setGuardando] = useState(false);
  const [errorDialogo, setErrorDialogo] = useState<string | null>(null);
  const [porBorrar, setPorBorrar] = useState<ConexionDb | null>(null);
  const [borrando, setBorrando] = useState(false);
  const [probando, setProbando] = useState<string | null>(null);
  const [aviso, setAviso] = useState<{ tipo: "info" | "error"; texto: string } | null>(null);

  const lista = conexiones.datos ?? [];
  const activa = abierta ? lista.find((c) => c.id === abierta) ?? null : null;

  async function guardar(datos: { name: string; connectionUrl: string; readOnly: boolean }) {
    setGuardando(true);
    setErrorDialogo(null);
    try {
      if (editando && editando !== "nueva") await api.put(`/db-admin/connections/${editando.id}`, datos);
      else await api.post("/db-admin/connections", datos);
      setEditando(null);
      conexiones.recargar();
    } catch (e) {
      setErrorDialogo(e instanceof ApiError ? e.message : "No se pudo guardar.");
    } finally {
      setGuardando(false);
    }
  }

  async function probar(c: ConexionDb) {
    setProbando(c.id);
    setAviso(null);
    try {
      const r = await api.post<{ version: string }>(`/db-admin/connections/${c.id}/test`);
      setAviso({ tipo: "info", texto: `${c.name}: conexión correcta. ${r.version}` });
    } catch (e) {
      setAviso({ tipo: "error", texto: `${c.name}: ${e instanceof ApiError ? e.message : "no se pudo conectar."}` });
    } finally {
      setProbando(null);
    }
  }

  async function borrar() {
    if (!porBorrar) return;
    setBorrando(true);
    try {
      await api.del(`/db-admin/connections/${porBorrar.id}`);
      if (abierta === porBorrar.id) setAbierta(null);
      conexiones.recargar();
    } catch (e) {
      setAviso({ tipo: "error", texto: e instanceof Error ? e.message : "No se pudo eliminar." });
    } finally {
      setBorrando(false);
      setPorBorrar(null);
    }
  }

  const abrirNueva = () => { setErrorDialogo(null); setEditando("nueva"); };

  return (
    <div className="pagina">
      <Cabecera
        etiqueta="administrador de bases de datos"
        titular="Tus bases, *a mano*."
        entradilla="Explora tablas, edita filas y lanza consultas contra tus PostgreSQL."
        acciones={!activa && (
          <button className="btn btn-principal" onClick={abrirNueva}>
            <Icono nombre="plus" tamano={16} />
            Nueva conexión
          </button>
        )}
      />

      {conexiones.error && <Alerta tipo="error">{conexiones.error}</Alerta>}
      {aviso && <Alerta tipo={aviso.tipo} alCerrar={() => setAviso(null)}>{aviso.texto}</Alerta>}

      {activa ? (
        <Explorador key={activa.id} conexion={activa} alCerrar={() => setAbierta(null)} />
      ) : conexiones.cargando && !conexiones.datos ? (
        <Esqueletos cantidad={2} />
      ) : lista.length === 0 ? (
        <Vacio icono="database" titular="Aún no hay *conexiones*." accion={<button className="btn btn-principal" onClick={abrirNueva}><Icono nombre="plus" tamano={16} />Añadir la primera</button>}>
          Guarda la cadena de conexión de una base PostgreSQL para explorarla desde aquí.
        </Vacio>
      ) : (
        <div className="rejilla">
          {lista.map((c, i) => (
            <article key={c.id} className="tarjeta cristal subir" style={{ "--i": i + 1 } as React.CSSProperties} aria-label={c.name}>
              <div className="tarjeta-cab">
                <span className="tarjeta-avatar"><Icono nombre="database" tamano={18} /></span>
                <div className="tarjeta-titulo">
                  <span className="tarjeta-nombre" title={c.name}>{c.name}</span>
                  <span className="tarjeta-sub" title={c.host}>{c.host}</span>
                </div>
                <div className="tarjeta-iconos">
                  <button className="accion" onClick={() => { setErrorDialogo(null); setEditando(c); }} aria-label={`Editar ${c.name}`} title="Editar"><Icono nombre="edit" tamano={16} /></button>
                  <button className="accion peligro" onClick={() => setPorBorrar(c)} aria-label={`Eliminar ${c.name}`} title="Eliminar"><Icono nombre="trash" tamano={16} /></button>
                </div>
              </div>
              <div className="fichas">
                <span className="ficha">{c.readOnly ? "solo lectura" : "lectura y escritura"}</span>
              </div>
              <div className="tarjeta-pie acciones-tarjeta">
                <button className="btn btn-pequeno btn-principal" onClick={() => setAbierta(c.id)}>
                  <Icono nombre="search" tamano={13} />
                  Explorar
                </button>
                <button className="btn btn-pequeno btn-fantasma" onClick={() => void probar(c)} disabled={probando === c.id}>
                  <Icono nombre={probando === c.id ? "refresh" : "pulse"} tamano={13} />
                  {probando === c.id ? "Probando…" : "Probar"}
                </button>
              </div>
            </article>
          ))}
        </div>
      )}

      {editando && (
        <DialogoConexion
          key={editando === "nueva" ? "nueva" : editando.id}
          conexion={editando === "nueva" ? null : editando}
          guardando={guardando}
          error={errorDialogo}
          alGuardar={(d) => void guardar(d)}
          alCerrar={() => setEditando(null)}
        />
      )}
      {porBorrar && (
        <Confirmar
          titulo={`¿Eliminar «${porBorrar.name}»?`}
          texto="Solo se olvida la conexión guardada. La base de datos no se toca."
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
