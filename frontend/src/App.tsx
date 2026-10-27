import { BrowserRouter, NavLink, Navigate, Route, Routes } from "react-router-dom";

import { ProveedorSesion, useSesion } from "./api/sesion";
import { Acceso } from "./pages/Acceso";
import { Calendario } from "./pages/Calendario";
import { Inicio } from "./pages/Inicio";
import { Icono } from "./ui/Iconos";
import type { NombreIcono } from "./ui/Iconos";
import { Marca } from "./ui/Marca";

/** Las secciones del panel. Añadir un módulo es añadir una línea aquí. */
const SECCIONES: { ruta: string; etiqueta: string; corta: string; icono: NombreIcono }[] = [
  { ruta: "/", etiqueta: "Inicio", corta: "inicio", icono: "home" },
  { ruta: "/calendar", etiqueta: "Calendario", corta: "agenda", icono: "calendar" },
];

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

  // Sin esta espera se vería el acceso un instante aunque hubiera sesión.
  if (cargando) {
    return (
      <div className="cargando-pantalla" role="status" aria-label="Cargando">
        <Marca tamano={44} />
      </div>
    );
  }

  if (!usuario) return <Acceso />;

  return (
    <div className="app">
      <Lateral />
      <main className="contenido">
        <Routes>
          <Route path="/" element={<Inicio />} />
          <Route path="/calendar" element={<Calendario />} />
          {/* Una ruta que no existe lleva al inicio. */}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
      <BarraInferior />
    </div>
  );
}

function iniciales(texto: string): string {
  const partes = texto.trim().split(/[\s@.]+/).filter(Boolean);
  return ((partes[0]?.[0] ?? "?") + (partes[1]?.[0] ?? "")).toUpperCase();
}

function Lateral() {
  const { usuario, salir } = useSesion();
  const nombre = usuario?.name ?? usuario?.email ?? "";

  return (
    <aside className="lateral">
      <div className="marca">
        <Marca />
        <span>
          <span className="marca-nombre">Home Lab</span>
          <span className="etiqueta marca-sub">panel de herramientas</span>
        </span>
      </div>

      <nav aria-label="Secciones">
        <span className="etiqueta menu-titulo">menú</span>
        <div className="menu">
          {SECCIONES.map((s) => (
            <NavLink key={s.ruta} to={s.ruta} end className={({ isActive }) => (isActive ? "activo" : "")}>
              <Icono nombre={s.icono} />
              {s.etiqueta}
            </NavLink>
          ))}
        </div>
      </nav>

      <div className="lateral-pie">
        <div className="lateral-usuario">
          <span className="avatar" aria-hidden="true">
            {iniciales(nombre)}
          </span>
          <span style={{ minWidth: 0 }}>
            <b>{nombre}</b>
            <span className="etiqueta">
              {usuario?.role === "superadmin" ? "administrador" : "usuario"}
            </span>
          </span>
        </div>
        <button className="btn btn-fantasma btn-ancho" onClick={() => void salir()}>
          <Icono nombre="logout" tamano={16} />
          Cerrar sesión
        </button>
      </div>
    </aside>
  );
}

/** En móvil la barra lateral desaparece y la navegación baja aquí. */
function BarraInferior() {
  const { salir } = useSesion();

  return (
    <nav className="barra-inferior" aria-label="Secciones">
      {SECCIONES.map((s) => (
        <NavLink key={s.ruta} to={s.ruta} end className={({ isActive }) => (isActive ? "activo" : "")}>
          <Icono nombre={s.icono} tamano={20} />
          {s.corta}
        </NavLink>
      ))}
      <button onClick={() => void salir()}>
        <Icono nombre="logout" tamano={20} />
        salir
      </button>
    </nav>
  );
}
