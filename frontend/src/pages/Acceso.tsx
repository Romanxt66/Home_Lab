import { useEffect, useState } from "react";
import type { FormEvent } from "react";

import { api, urlApi } from "../api/client";
import { useSesion } from "../api/sesion";
import { Icono } from "../ui/Iconos";
import { Marca } from "../ui/Marca";

/**
 * Lo que el servidor deja en la dirección al volver de Google: un estado (la
 * cuenta espera aprobación) o un error. Esperar no es un fallo, así que se
 * cuenta con otro tono.
 */
function mensajeDeLaDireccion(): { tipo: "info" | "error"; texto: string } | null {
  const q = new URLSearchParams(window.location.search);

  switch (q.get("estado")) {
    case "registered":
      return { tipo: "info", texto: "Te has registrado. Un administrador tiene que aprobar tu cuenta antes de que puedas entrar." };
    case "pending":
      return { tipo: "info", texto: "Tu cuenta sigue pendiente de aprobación. Te dejarán entrar en cuanto un administrador la revise." };
    case "rejected":
      return { tipo: "error", texto: "A esta cuenta se le ha denegado el acceso." };
  }

  const error = q.get("error");
  return error ? { tipo: "error", texto: error } : null;
}

/** Pantalla de inicio de sesión. */
export function Acceso() {
  const { entrar } = useSesion();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [enviando, setEnviando] = useState(false);
  const [hayGoogle, setHayGoogle] = useState(false);
  const [aviso] = useState(mensajeDeLaDireccion);

  // El botón de Google solo se ofrece si el servidor tiene sus credenciales.
  useEffect(() => {
    api
      .get<{ google: boolean }>("/auth/proveedores")
      .then((p) => setHayGoogle(p.google))
      .catch(() => setHayGoogle(false));
  }, []);

  async function alEnviar(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setEnviando(true);
    try {
      await entrar(email, password);
      // No hace falta navegar: al haber sesión, App pinta el panel.
    } catch (err) {
      setError(err instanceof Error ? err.message : "No se pudo iniciar sesión.");
    } finally {
      setEnviando(false);
    }
  }

  const mensaje = error ? { tipo: "error" as const, texto: error } : aviso;

  return (
    <div className="acceso">
      <form className="acceso-caja" onSubmit={alEnviar}>
        <div className="marca">
          <Marca />
          <span>
            <span className="marca-nombre">Home Lab</span>
            <span className="etiqueta marca-sub">panel de herramientas</span>
          </span>
        </div>

        <span className="etiqueta">acceso</span>
        <h1>
          Bienvenido <em>de vuelta</em>.
        </h1>
        <p>Entra con la misma cuenta que usas en el Lab.</p>

        {mensaje && (
          <div
            className={`alerta ${mensaje.tipo === "error" ? "alerta-error" : "alerta-info"}`}
            role={mensaje.tipo === "error" ? "alert" : "status"}
          >
            <Icono nombre={mensaje.tipo === "error" ? "alert" : "info"} tamano={16} />
            <span>{mensaje.texto}</span>
          </div>
        )}

        <div className="campo">
          <label htmlFor="email">Correo</label>
          <input
            id="email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="username"
            required
          />
        </div>

        <div className="campo">
          <label htmlFor="password">Contraseña</label>
          <input
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
          />
        </div>

        <button type="submit" className="btn btn-principal btn-ancho" disabled={enviando}>
          {enviando ? "Entrando…" : "Entrar"}
        </button>

        {hayGoogle && (
          <>
            <div className="separador">o</div>
            {/* Un enlace y no un botón: Google redirige, no hay nada que fetch. */}
            <a className="btn btn-ancho" href={urlApi("/auth/google/start")}>
              <Icono nombre="user" tamano={16} />
              Continuar con Google
            </a>
          </>
        )}
      </form>
    </div>
  );
}
