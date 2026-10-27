import { BrowserRouter, NavLink, Navigate, Route, Routes } from "react-router-dom";

import { ProveedorSesion, useSesion } from "./api/sesion";
import { Acceso } from "./pages/Acceso";
import { Calendario } from "./pages/Calendario";
import { Automatizaciones } from "./pages/Automatizaciones";
import { Correo } from "./pages/Correo";
import { GitHub } from "./pages/GitHub";
import { Inicio } from "./pages/Inicio";
import { Scraper } from "./pages/Scraper";
import { Tareas } from "./pages/Tareas";
import { Telegram } from "./pages/Telegram";
import { Usuarios } from "./pages/Usuarios";
import { Avatar } from "./ui/Avatar";
import { Icono } from "./ui/Iconos";
import type { NombreIcono } from "./ui/Iconos";
import { Marca } from "./ui/Marca";

/** Las secciones del panel. Añadir un módulo es añadir una línea aquí. */
const SECCIONES: { ruta: string; etiqueta: string; corta: string; icono: NombreIcono; soloAdmin?: boolean }[] = [
  { ruta: "/", etiqueta: "Inicio", corta: "inicio", icono: "home" },
  { ruta: "/calendar", etiqueta: "Calendario", corta: "agenda", icono: "calendar" },
  { ruta: "/automations", etiqueta: "Automatizaciones", corta: "reglas", icono: "bolt" },
  { ruta: "/email-automation", etiqueta: "Correo", corta: "correo", icono: "mail" },
  { ruta: "/github", etiqueta: "GitHub", corta: "github", icono: "git" },
  { ruta: "/telegram", etiqueta: "Telegram", corta: "avisos", icono: "telegram" },
  { ruta: "/scraper", etiqueta: "Scraper", corta: "scraper", icono: "code" },
  { ruta: "/scheduler", etiqueta: "Tareas programadas", corta: "tareas", icono: "clock" },
  { ruta: "/users", etiqueta: "Usuarios", corta: "gente", icono: "shield", soloAdmin: true },
];

/** Las secciones que ve quien ha entrado: las de administración, solo el administrador. */
function useSecciones() {
  const { usuario } = useSesion();
  return SECCIONES.filter((s) => !s.soloAdmin || usuario?.role === "superadmin");
}

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
          <Route path="/scheduler" element={<Tareas />} />
          <Route path="/automations" element={<Automatizaciones />} />
          <Route path="/email-automation" element={<Correo />} />
          <Route path="/github" element={<GitHub />} />
          <Route path="/telegram" element={<Telegram />} />
          <Route path="/scraper" element={<Scraper />} />
          {/* El servidor ya rechaza a quien no es administrador; esto solo
              evita enseñarle una pantalla que no puede usar. */}
          <Route
            path="/users"
            element={usuario.role === "superadmin" ? <Usuarios /> : <Navigate to="/" replace />}
          />
          {/* Una ruta que no existe lleva al inicio. */}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
      <BarraInferior />
    </div>
  );
}

function Lateral() {
  const { usuario, salir } = useSesion();
  const secciones = useSecciones();
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
          {secciones.map((s) => (
            <NavLink key={s.ruta} to={s.ruta} end className={({ isActive }) => (isActive ? "activo" : "")}>
              <Icono nombre={s.icono} />
              {s.etiqueta}
            </NavLink>
          ))}
        </div>
      </nav>

      <div className="lateral-pie">
        <div className="lateral-usuario">
          <Avatar nombre={nombre} />
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
  const secciones = useSecciones();

  return (
    <nav className="barra-inferior" aria-label="Secciones">
      {secciones.map((s) => (
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
