import { useState } from "react";
import type { FormEvent } from "react";

import { useSesion } from "../api/sesion";

/** Pantalla de inicio de sesión. */
export function Acceso() {
  const { entrar } = useSesion();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [enviando, setEnviando] = useState(false);

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

  return (
    <div className="acceso">
      <form className="acceso-caja" onSubmit={alEnviar}>
        <div className="marca" style={{ padding: 0, marginBottom: "1.25rem" }}>
          <span className="marca-logo" />
          <span>
            <span className="marca-nombre">Home Lab</span>
            <span className="marca-sub">Panel de herramientas</span>
          </span>
        </div>

        <h1>Inicia sesión</h1>
        <p>Entra con la misma cuenta que usas en el Lab.</p>

        {error && <div className="alerta alerta-error">{error}</div>}

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
      </form>
    </div>
  );
}
