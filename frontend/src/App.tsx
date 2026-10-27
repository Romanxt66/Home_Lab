import { BrowserRouter, NavLink, Navigate, Route, Routes } from "react-router-dom";

import { ProveedorSesion, useSesion } from "./api/sesion";
import { Acceso } from "./pages/Acceso";
import { Uptime } from "./pages/Uptime";

export default function App() {
  return (
    <ProveedorSesion>
      <BrowserRouter>
        <Raiz />
      </BrowserRouter>
    </ProveedorSesion>
  );
}

/** Decide qué se ve: el acceso o el panel. */
function Raiz() {
  const { usuario, cargando } = useSesion();

  // Sin esta espera se vería el login un instante aunque hubiera sesión.
  if (cargando) {
    return (
      <div className="acceso">
        <span style={{ color: "var(--texto-4)" }}>Cargando…</span>
      </div>
    );
  }

  if (!usuario) return <Acceso />;

  return (
    <div className="app">
      <Lateral />
      <main className="contenido">
        <Routes>
          <Route path="/uptime" element={<Uptime />} />
          {/* Mientras solo hay un módulo, cualquier otra ruta lleva a él. */}
          <Route path="*" element={<Navigate to="/uptime" replace />} />
        </Routes>
      </main>
    </div>
  );
}

function Lateral() {
  const { usuario, salir } = useSesion();

  return (
    <aside className="lateral">
      <div className="marca">
        <span className="marca-logo" />
        <span>
          <span className="marca-nombre">Home Lab</span>
          <span className="marca-sub">Panel de herramientas</span>
        </span>
      </div>

      <nav>
        <div className="menu-titulo">Menú principal</div>
        <div className="menu">
          <NavLink to="/uptime" className={({ isActive }) => (isActive ? "activo" : "")}>
            Monitor de uptime
          </NavLink>
        </div>
      </nav>

      <div className="lateral-pie">
        <div className="lateral-usuario">
          <b>{usuario?.name ?? usuario?.email}</b>
          {usuario?.role === "superadmin" ? "Administrador" : "Usuario"}
        </div>
        <button className="btn btn-ancho" onClick={() => void salir()}>
          Cerrar sesión
        </button>
      </div>
    </aside>
  );
}
