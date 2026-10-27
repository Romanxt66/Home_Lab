import { useSesion } from "../api/sesion";
import { Cabecera } from "../ui/Cabecera";
import { Icono } from "../ui/Iconos";

/** Pantalla de entrada. Hasta que lleguen los módulos, lo dice con claridad. */
export function Inicio() {
  const { usuario } = useSesion();
  const nombre = (usuario?.name ?? usuario?.email ?? "").split(" ")[0];

  return (
    <div className="pagina">
      <Cabecera
        etiqueta="inicio"
        titular={nombre ? `Hola, *${nombre}*.` : "Tu laboratorio, *a mano*."}
        entradilla="Tu panel de herramientas."
      />

      <div className="cristal vacio subir" style={{ "--i": 1 } as React.CSSProperties}>
        <span className="vacio-icono">
          <Icono nombre="home" tamano={24} />
        </span>
        <h2>
          Aquí irán <em>tus módulos</em>.
        </h2>
        <p>
          El panel está listo para recibirlos: cada uno aparecerá en el menú en cuanto tenga su pantalla.
        </p>
      </div>
    </div>
  );
}
