import { useEffect, useState } from "react";
import { BrowserRouter, NavLink, Navigate, Route, Routes, useLocation } from "react-router-dom";

import { ProveedorSesion, useSesion } from "./api/sesion";
import { Acceso } from "./pages/Acceso";
import { Calendario } from "./pages/Calendario";
import { Automatizaciones } from "./pages/Automatizaciones";
import { Coolify } from "./pages/Coolify";
import { DbAdmin } from "./pages/DbAdmin";
import { Correo } from "./pages/Correo";
import { GitHub } from "./pages/GitHub";
import { Inicio } from "./pages/Inicio";
import { Scraper } from "./pages/Scraper";
import { Tareas } from "./pages/Tareas";
import { Telegram } from "./pages/Telegram";
import { Usuarios } from "./pages/Usuarios";
import { Asistente } from "./ui/Asistente";
import { Avatar } from "./ui/Avatar";
import { BarraPestanas, NavSuperior } from "./ui/Navegacion";
import { Button, Glass, Icon, useTheme } from "./design";
import { Icono } from "./ui/Iconos";
import { useSecciones } from "./secciones";
import { Marca } from "./ui/Marca";

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
    <div className="cr-shell">
      <Cabecera />
      <main className="cr-main">
        <Routes>
          <Route path="/" element={<Inicio />} />
          <Route path="/calendar" element={<Calendario />} />
          <Route path="/scheduler" element={<Tareas />} />
          <Route path="/automations" element={<Automatizaciones />} />
          <Route path="/coolify" element={<Coolify />} />
          <Route path="/db-admin" element={<DbAdmin />} />
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
      <div className="cr-bottom">
        <BarraInferior />
      </div>
      <Asistente />
    </div>
  );
}

/** Barra superior: marca, navegación con píldora, tema y la persona que ha entrado. */
function Cabecera() {
  const { usuario, salir } = useSesion();
  const secciones = useSecciones();
  const { choice, cycle } = useTheme();
  const nombre = usuario?.name ?? usuario?.email ?? "";
  const nombreTema = choice === "system" ? "del sistema" : choice === "light" ? "claro" : "oscuro";

  return (
    <Glass as="header" className="cr-topbar">
      <NavLink to="/" className="cr-brand" aria-label="Home Lab, ir al inicio">
        <Marca tamano={34} />
        <span className="cr-wide-only">Home Lab</span>
      </NavLink>

      <NavSuperior secciones={secciones} />
      <span className="cr-spacer cr-solo-movil" />

      <Button
        variant="ghost"
        onClick={cycle}
        aria-label={`Cambiar el tema, ahora ${nombreTema}`}
        title={`Tema ${nombreTema}`}
      >
        <Icon name={choice === "light" ? "sun" : "moon"} />
      </Button>
      <span className="cr-wide-only cabecera-usuario" title={nombre}>
        <Avatar nombre={nombre} tamano={34} />
      </span>
      <Button variant="ghost" onClick={() => void salir()} aria-label="Cerrar sesión" title="Cerrar sesión">
        <Icon name="logout" />
      </Button>
    </Glass>
  );
}

/** Las que van siempre a mano en la barra de móvil; el resto cuelga de «más». */
const PRINCIPALES = ["/", "/calendar", "/coolify", "/db-admin"];

/** En móvil la navegación baja aquí: cuatro pestañas y «más», que abre todas las secciones. */
function BarraInferior() {
  const { usuario, salir } = useSesion();
  const secciones = useSecciones();
  const { pathname } = useLocation();

  // La hoja recuerda en qué pantalla se abrió: al navegar deja de coincidir y
  // se cierra sola, sin taparle al usuario la pantalla a la que acaba de ir.
  const [abiertaEn, setAbiertaEn] = useState<string | null>(null);
  const abierto = abiertaEn === pathname;

  useEffect(() => {
    if (!abierto) return;
    const cerrar = (e: KeyboardEvent) => e.key === "Escape" && setAbiertaEn(null);
    window.addEventListener("keydown", cerrar);
    return () => window.removeEventListener("keydown", cerrar);
  }, [abierto]);

  const principales = secciones.filter((s) => PRINCIPALES.includes(s.ruta));
  // «Más» se marca cuando la pantalla actual es una de las que viven dentro.
  const enMas = !principales.some((s) => s.ruta === pathname);

  return (
    <>
      {abierto && (
        <>
          <div className="hoja-fondo" onClick={() => setAbiertaEn(null)} aria-hidden />
          <Glass strong className="hoja" role="dialog" aria-label="Todas las secciones">
            <div className="hoja-rejilla">
              {secciones.map((s) => (
                <NavLink key={s.ruta} to={s.ruta} end className={({ isActive }) => (isActive ? "activo" : "")}>
                  <Icono nombre={s.icono} tamano={20} />
                  {s.etiqueta}
                </NavLink>
              ))}
            </div>
            <div className="hoja-pie">
              <span className="label cr-muted">{usuario?.name ?? usuario?.email}</span>
              <Button size="sm" variant="ghost" icon="logout" onClick={() => void salir()}>
                Cerrar sesión
              </Button>
            </div>
          </Glass>
        </>
      )}

      <BarraPestanas
        secciones={principales}
        final={
          <button
            type="button"
            className="cr-tab"
            aria-current={enMas || abierto ? "page" : undefined}
            aria-expanded={abierto}
            onClick={() => setAbiertaEn(abierto ? null : pathname)}
          >
            <Icono nombre="list" tamano={20} />
            más
          </button>
        }
      />
    </>
  );
}
