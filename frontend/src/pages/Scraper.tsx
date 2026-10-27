import { useState } from "react";
import type { FormEvent } from "react";

import { api, ApiError } from "../api/client";
import type { ResultadoScraper } from "../api/tipos";
import { Alerta } from "../ui/Alerta";
import { Cabecera } from "../ui/Cabecera";
import { Codigo } from "../ui/Codigo";
import { Icono } from "../ui/Iconos";

const EJEMPLOS = [
  { etiqueta: "Titulares", selector: "h1, h2", atributo: "" },
  { etiqueta: "Enlaces", selector: "a", atributo: "href" },
  { etiqueta: "Imágenes", selector: "img", atributo: "src" },
];

/** Scraper: lee una página y saca lo que señale un selector CSS. */
export function Scraper() {
  const [url, setUrl] = useState("");
  const [selector, setSelector] = useState("");
  const [atributo, setAtributo] = useState("");
  const [leyendo, setLeyendo] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [resultado, setResultado] = useState<ResultadoScraper | null>(null);

  async function leer(e: FormEvent) {
    e.preventDefault();
    setLeyendo(true);
    setError(null);
    try {
      setResultado(await api.post<ResultadoScraper>("/scraper", { url, selector, attribute: atributo }));
    } catch (err) {
      setResultado(null);
      setError(err instanceof ApiError ? err.message : "No se pudo leer la página.");
    } finally {
      setLeyendo(false);
    }
  }

  const items = resultado?.items;
  const conSelector = selector.trim() !== "";

  return (
    <div className="pagina">
      <Cabecera
        etiqueta="scraper"
        titular="Saca *lo que importa*."
        entradilla="Lee cualquier página y extrae con un selector CSS justo lo que buscas."
      />

      <form className="cristal panel subir" style={{ "--i": 1 } as React.CSSProperties} onSubmit={leer}>
        <div className="campo">
          <label htmlFor="sc-url">Dirección</label>
          <input
            id="sc-url"
            type="url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="https://ejemplo.com"
            required
          />
        </div>

        <div className="fila">
          <div className="campo">
            <label htmlFor="sc-sel">Selector CSS</label>
            <input
              id="sc-sel"
              type="text"
              className="entrada-mono"
              value={selector}
              onChange={(e) => setSelector(e.target.value)}
              placeholder="vacío para ver la página entera"
              spellCheck={false}
            />
          </div>
          <div className="campo">
            <label htmlFor="sc-attr">Atributo</label>
            <input
              id="sc-attr"
              type="text"
              className="entrada-mono"
              value={atributo}
              onChange={(e) => setAtributo(e.target.value)}
              placeholder="vacío para el texto"
              spellCheck={false}
              disabled={!conSelector}
            />
          </div>
        </div>

        <div className="presets" role="group" aria-label="Ejemplos">
          {EJEMPLOS.map((ej) => (
            <button
              key={ej.etiqueta}
              type="button"
              className="ficha ficha-boton"
              onClick={() => {
                setSelector(ej.selector);
                setAtributo(ej.atributo);
              }}
            >
              {ej.etiqueta}
            </button>
          ))}
        </div>

        <div className="acciones-fila">
          <button type="submit" className="btn btn-principal" disabled={leyendo}>
            <Icono nombre={leyendo ? "refresh" : "search"} tamano={15} />
            {leyendo ? "Leyendo…" : "Leer página"}
          </button>
        </div>
      </form>

      {error && <Alerta tipo="error">{error}</Alerta>}

      {resultado && (
        <section className="subir" aria-label="Resultado" style={{ display: "grid", gap: "0.8rem" }}>
          <div className="fichas">
            <span className={`ficha ficha-estado ${resultado.status >= 400 ? "tono-peligro" : "tono-acento"}`}
              style={{ "--tono": resultado.status >= 400 ? "var(--peligro)" : "var(--acento)" } as React.CSSProperties}>
              <span className="punto" />
              HTTP {resultado.status}
            </span>
            {resultado.contentType && <span className="ficha">{resultado.contentType.split(";")[0]}</span>}
            {items && <span className="ficha">{items.length} {items.length === 1 ? "coincidencia" : "coincidencias"}</span>}
          </div>

          {items ? (
            items.length === 0 ? (
              <div className="cristal panel">
                <p className="texto-apagado">
                  El selector es válido pero no encontró nada en esa página. Quizá el contenido se carga con
                  JavaScript, que este lector no ejecuta.
                </p>
              </div>
            ) : (
              <Codigo titulo={`coincidencias · ${items.length}`} texto={items.join("\n")} />
            )
          ) : (
            <Codigo titulo="contenido (primeros 20.000 caracteres)" texto={resultado.raw ?? ""} />
          )}
        </section>
      )}
    </div>
  );
}
