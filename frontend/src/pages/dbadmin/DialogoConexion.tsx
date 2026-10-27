import { useState } from "react";

import type { ConexionDb } from "../../api/tipos";
import { Dialogo } from "../../ui/Dialogo";

interface Props {
  conexion: ConexionDb | null;
  guardando: boolean;
  error: string | null;
  alGuardar(datos: { name: string; connectionUrl: string; readOnly: boolean }): void;
  alCerrar(): void;
}

export function DialogoConexion({ conexion, guardando, error, alGuardar, alCerrar }: Props) {
  const [nombre, setNombre] = useState(conexion?.name ?? "");
  const [url, setUrl] = useState("");
  const [soloLectura, setSoloLectura] = useState(conexion?.readOnly ?? true);

  return (
    <Dialogo
      etiqueta={conexion ? "editar conexión" : "nueva conexión"}
      titular={conexion ? "Ajusta *la conexión*." : "Conecta una *base*."}
      error={error}
      enviando={guardando}
      alEnviar={() => alGuardar({ name: nombre, connectionUrl: url, readOnly: soloLectura })}
      alCerrar={alCerrar}
    >
      <div className="campo">
        <label htmlFor="cx-nombre">Nombre</label>
        <input id="cx-nombre" type="text" value={nombre} onChange={(e) => setNombre(e.target.value)} placeholder="Producción" required autoFocus />
      </div>
      <div className="campo">
        <label htmlFor="cx-url">Cadena de conexión</label>
        <input
          id="cx-url"
          type="password"
          className="entrada-mono"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="postgresql://usuario:clave@servidor:5432/base"
          autoComplete="off"
          spellCheck={false}
          required
        />
        <span className="campo-ayuda">
          {conexion
            ? `Ahora apunta a ${conexion.host}. La cadena no se vuelve a mostrar, así que hay que escribirla otra vez.`
            : "Solo PostgreSQL. Se guarda en el servidor y no se vuelve a mostrar."}
        </span>
      </div>
      <label className="casilla">
        <input type="checkbox" checked={soloLectura} onChange={(e) => setSoloLectura(e.target.checked)} />
        Solo lectura: no deja modificar nada desde aquí
      </label>
    </Dialogo>
  );
}
