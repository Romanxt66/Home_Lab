import { useEffect, useMemo, useState } from "react";

import { api } from "../api/client";
import type { EstadoUsuario, RolUsuario, UsuarioAdmin } from "../api/tipos";
import { Avatar } from "../ui/Avatar";
import { Cabecera } from "../ui/Cabecera";
import { Confirmar } from "../ui/Confirmar";
import { Icono } from "../ui/Iconos";
import { Resumen } from "../ui/Resumen";
import { hace } from "../ui/tiempo";

type Filtro = EstadoUsuario | "todos";

const ETIQUETA_ESTADO: Record<EstadoUsuario, string> = {
  pending: "Pendiente",
  approved: "Aprobado",
  rejected: "Rechazado",
};

const TONO_ESTADO: Record<EstadoUsuario, "aviso" | "acento" | "peligro"> = {
  pending: "aviso",
  approved: "acento",
  rejected: "peligro",
};

/** Lo que se está a punto de hacer y espera confirmación. */
type Pendiente =
  | { tipo: "rechazar"; usuario: UsuarioAdmin }
  | { tipo: "rol"; usuario: UsuarioAdmin; rol: RolUsuario };

/** Administración de cuentas: aprobar registros, revocar accesos y repartir roles. */
export function Usuarios() {
  const [lista, setLista] = useState<UsuarioAdmin[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  // null = aún no ha elegido: se decide solo, según haya o no pendientes.
  const [elegido, setElegido] = useState<Filtro | null>(null);

  const [ocupado, setOcupado] = useState<string | null>(null);
  const [pendiente, setPendiente] = useState<Pendiente | null>(null);
  const [trabajando, setTrabajando] = useState(false);

  useEffect(() => {
    let vigente = true;

    api
      .get<UsuarioAdmin[]>("/users")
      .then((datos) => {
        if (vigente) setLista(datos);
      })
      .catch((e) => {
        if (!vigente) return;
        setError(e instanceof Error ? e.message : "No se pudieron cargar los usuarios.");
        setLista([]);
      });

    return () => {
      vigente = false;
    };
  }, []);

  const cuentas = useMemo(() => {
    const l = lista ?? [];
    return {
      total: l.length,
      pending: l.filter((u) => u.status === "pending").length,
      approved: l.filter((u) => u.status === "approved").length,
      rejected: l.filter((u) => u.status === "rejected").length,
    };
  }, [lista]);

  // Si hay cuentas esperando, lo primero que se quiere ver es eso.
  const filtro: Filtro = elegido ?? (cuentas.pending > 0 ? "pending" : "todos");

  const visibles = (lista ?? []).filter((u) => filtro === "todos" || u.status === filtro);

  /** Cambia una cuenta en la lista por la que devolvió el servidor. */
  const reemplazar = (nueva: UsuarioAdmin) =>
    setLista((l) => (l ?? []).map((u) => (u.id === nueva.id ? nueva : u)));

  async function aprobar(u: UsuarioAdmin) {
    setOcupado(u.id);
    setError(null);
    try {
      reemplazar(await api.post<UsuarioAdmin>(`/users/${u.id}/approve`));
    } catch (e) {
      setError(e instanceof Error ? e.message : "No se pudo aprobar.");
    } finally {
      setOcupado(null);
    }
  }

  async function confirmar() {
    if (!pendiente) return;
    setTrabajando(true);
    setError(null);
    try {
      const { usuario } = pendiente;
      reemplazar(
        pendiente.tipo === "rechazar"
          ? await api.post<UsuarioAdmin>(`/users/${usuario.id}/reject`)
          : await api.put<UsuarioAdmin>(`/users/${usuario.id}/role`, { role: pendiente.rol }),
      );
    } catch (e) {
      setError(e instanceof Error ? e.message : "No se pudo completar la acción.");
    } finally {
      setTrabajando(false);
      setPendiente(null);
    }
  }

  const cargando = lista === null;
  const alternar = (f: Filtro) => setElegido(filtro === f ? "todos" : f);

  return (
    <div className="pagina">
      <Cabecera
        etiqueta="usuarios"
        titular="Quién *entra*, tú decides."
        entradilla={
          cuentas.pending > 0
            ? `${cuentas.pending === 1 ? "Hay una cuenta esperando" : `Hay ${cuentas.pending} cuentas esperando`} tu aprobación.`
            : "Aprueba registros, revoca accesos y reparte permisos."
        }
      />

      {error && (
        <div className="alerta alerta-error subir" role="alert">
          <Icono nombre="alert" tamano={16} />
          <span>{error}</span>
        </div>
      )}

      {cargando ? (
        <div className="rejilla" aria-busy="true" aria-label="Cargando usuarios">
          {[0, 1, 2].map((i) => (
            <div key={i} className="cristal esqueleto" />
          ))}
        </div>
      ) : (
        <>
          <div className="resumenes subir" style={{ "--i": 1 } as React.CSSProperties}>
            <Resumen
              etiqueta="pendientes"
              valor={cuentas.pending}
              icono="clock"
              tono={cuentas.pending > 0 ? "aviso" : "neutro"}
              seleccionado={filtro === "pending"}
              alPulsar={() => alternar("pending")}
            />
            <Resumen
              etiqueta="aprobados"
              valor={cuentas.approved}
              icono="check"
              tono="acento"
              seleccionado={filtro === "approved"}
              alPulsar={() => alternar("approved")}
            />
            <Resumen
              etiqueta="rechazados"
              valor={cuentas.rejected}
              icono="close"
              tono={cuentas.rejected > 0 ? "peligro" : "neutro"}
              seleccionado={filtro === "rejected"}
              alPulsar={() => alternar("rejected")}
            />
            <Resumen
              etiqueta="cuentas"
              valor={cuentas.total}
              icono="user"
              seleccionado={filtro === "todos"}
              alPulsar={() => setElegido("todos")}
            />
          </div>

          {visibles.length === 0 ? (
            <div className="cristal vacio subir">
              <p>
                {filtro === "pending"
                  ? "No hay nadie esperando. Todo al día."
                  : "No hay cuentas con ese estado."}
              </p>
              <button className="btn btn-pequeno" onClick={() => setElegido("todos")}>
                Ver todas
              </button>
            </div>
          ) : (
            <div className="rejilla">
              {visibles.map((u, i) => (
                <TarjetaUsuario
                  key={u.id}
                  usuario={u}
                  indice={i + 2}
                  ocupado={ocupado === u.id}
                  alAprobar={() => void aprobar(u)}
                  alRechazar={() => setPendiente({ tipo: "rechazar", usuario: u })}
                  alCambiarRol={(rol) => setPendiente({ tipo: "rol", usuario: u, rol })}
                />
              ))}
            </div>
          )}
        </>
      )}

      {pendiente && (
        <Confirmar
          titulo={tituloDe(pendiente)}
          texto={textoDe(pendiente)}
          etiquetaConfirmar={etiquetaDe(pendiente)}
          peligro={pendiente.tipo === "rechazar"}
          trabajando={trabajando}
          alConfirmar={() => void confirmar()}
          alCancelar={() => setPendiente(null)}
        />
      )}
    </div>
  );
}

function nombreDe(u: UsuarioAdmin): string {
  return u.name ?? u.email;
}

function tituloDe(p: Pendiente): string {
  const n = nombreDe(p.usuario);
  if (p.tipo === "rechazar") {
    return p.usuario.status === "approved" ? `¿Revocar el acceso de ${n}?` : `¿Rechazar a ${n}?`;
  }
  return p.rol === "superadmin" ? `¿Hacer administrador a ${n}?` : `¿Quitar los permisos de administrador a ${n}?`;
}

function textoDe(p: Pendiente): string {
  if (p.tipo === "rechazar") {
    return "No podrá entrar. Si te lo piensas mejor, podrás volver a aprobarlo más tarde.";
  }
  return p.rol === "superadmin"
    ? "Podrá aprobar o rechazar cuentas y cambiar la configuración de todos los módulos."
    : "Seguirá pudiendo entrar, pero ya no podrá administrar cuentas ni configuración.";
}

function etiquetaDe(p: Pendiente): string {
  if (p.tipo === "rechazar") return p.usuario.status === "approved" ? "Revocar acceso" : "Rechazar";
  return p.rol === "superadmin" ? "Hacer administrador" : "Quitar permisos";
}

interface TarjetaProps {
  usuario: UsuarioAdmin;
  indice: number;
  ocupado: boolean;
  alAprobar(): void;
  alRechazar(): void;
  alCambiarRol(rol: RolUsuario): void;
}

function TarjetaUsuario({ usuario: u, indice, ocupado, alAprobar, alRechazar, alCambiarRol }: TarjetaProps) {
  const esAdmin = u.role === "superadmin";

  return (
    <article
      className={`tarjeta usuario cristal subir tono-${TONO_ESTADO[u.status]}`}
      style={{ "--i": indice } as React.CSSProperties}
      aria-label={`${nombreDe(u)}: ${ETIQUETA_ESTADO[u.status]}`}
    >
      <div className="tarjeta-cab">
        <Avatar nombre={nombreDe(u)} imagen={u.picture} tamano={42} />
        <div className="tarjeta-titulo">
          <span className="tarjeta-nombre" title={nombreDe(u)}>
            {nombreDe(u)}
          </span>
          {u.name && (
            <span className="tarjeta-sub" title={u.email}>
              {u.email}
            </span>
          )}
        </div>
        {u.isSelf && <span className="ficha ficha-acento">tú</span>}
      </div>

      <div className="fichas">
        <span className="ficha ficha-estado">
          <span className="punto" />
          {ETIQUETA_ESTADO[u.status]}
        </span>
        <span className="ficha">
          {esAdmin && <Icono nombre="shield" tamano={11} />}
          {esAdmin ? "administrador" : "usuario"}
        </span>
        <span className="ficha">{u.authMethod === "google" ? "con Google" : "con contraseña"}</span>
        <span className="ficha">{hace(u.createdAt)}</span>
      </div>

      {u.isSelf ? (
        <p className="tarjeta-nota">Esta es tu cuenta: no puedes rechazarla ni cambiarte el rol.</p>
      ) : (
        <div className="tarjeta-pie">
          {u.status !== "approved" && (
            <button className="btn btn-principal btn-pequeno" onClick={alAprobar} disabled={ocupado}>
              <Icono nombre="check" tamano={14} />
              {ocupado ? "Aprobando…" : "Aprobar"}
            </button>
          )}

          {u.status === "approved" && (
            <button className="btn btn-pequeno" onClick={() => alCambiarRol(esAdmin ? "user" : "superadmin")}>
              <Icono nombre="shield" tamano={14} />
              {esAdmin ? "Quitar administrador" : "Hacer administrador"}
            </button>
          )}

          {u.status !== "rejected" && (
            <button className="btn btn-pequeno btn-fantasma btn-peligro-texto" onClick={alRechazar}>
              {u.status === "approved" ? "Revocar acceso" : "Rechazar"}
            </button>
          )}
        </div>
      )}
    </article>
  );
}
