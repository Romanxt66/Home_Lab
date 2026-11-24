import { useState } from "react";
import { useSearchParams } from "react-router-dom";

import { api, ApiError } from "../api/client";
import type { ConsentimientoOAuth } from "../api/tipos";
import { useSesion } from "../api/sesion";
import { Button, Glass } from "../design";
import { Alerta } from "../ui/Alerta";
import { Cabecera } from "../ui/Cabecera";
import { Icono } from "../ui/Iconos";
import { Esqueletos } from "../ui/Vacio";
import { useDatos } from "../ui/useDatos";

// Los parámetros que llegan del cliente y que el servidor vuelve a validar al decidir.
const PARAMETROS = ["response_type", "client_id", "redirect_uri", "state", "code_challenge", "code_challenge_method", "scope", "resource"] as const;

/**
 * Consentimiento de OAuth: una aplicación (claude.ai, Claude Code…) pide entrar al
 * espacio de trabajo. La persona ve quién es, a dónde volverá y elige el permiso.
 * Nada se concede hasta pulsar «Autorizar».
 */
export function AutorizarOAuth() {
  const { usuario } = useSesion();
  const [busqueda] = useSearchParams();
  const params = Object.fromEntries(PARAMETROS.map((k) => [k, busqueda.get(k) ?? ""]));
  const tiene = params.client_id !== "";

  const peticion = useDatos<ConsentimientoOAuth>(tiene ? `/oauth/request?${new URLSearchParams(params)}` : null);
  const [permiso, setPermiso] = useState<"read" | "write">("read");
  const [enviando, setEnviando] = useState<"si" | "no" | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function decidir(aprobar: boolean) {
    setEnviando(aprobar ? "si" : "no");
    setError(null);
    try {
      const r = await api.post<{ redirect: string }>("/oauth/approve", { ...params, approve: aprobar, permission: permiso });
      // Se sale del sitio hacia la dirección que el servidor ya comprobó contra la registrada.
      window.location.assign(r.redirect);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "No se pudo completar la autorización.");
      setEnviando(null);
    }
  }

  const c = peticion.datos;
  const nombre = usuario?.name ?? usuario?.email ?? "";

  return (
    <div className="pagina">
      <Cabecera etiqueta="conexión" titular="¿Dejas que *entre*?" entradilla="Una aplicación quiere acceder a tu espacio de trabajo de Home Lab." />

      {!tiene ? (
        <Alerta tipo="error">Esta página se abre desde la aplicación que quiere conectarse. Vuelve a ella y empieza de nuevo.</Alerta>
      ) : peticion.cargando && !c ? (
        <Esqueletos cantidad={2} />
      ) : peticion.error || !c ? (
        <Alerta tipo="error">
          La solicitud no es válida: {peticion.error ?? "no se pudo comprobar"}. Vuelve a la aplicación y empieza de nuevo.
        </Alerta>
      ) : (
        <Glass strong padded className="consentimiento">
          <div className="consentimiento-quien">
            <span className="tarjeta-avatar"><Icono nombre="link" tamano={20} /></span>
            <div>
              <h2 className="heading">{c.clientName}</h2>
              <p className="label cr-muted">
                {c.loopback ? "una aplicación en tu propio equipo" : <>te devolverá a <strong>{c.redirectHost}</strong></>}
              </p>
            </div>
          </div>

          <p className="body cr-muted">
            Podrá leer tus skills y documentos .md. Lo que otras personas compartan contigo solo lo verá si se lo pide expresamente. Entras como <strong>{nombre}</strong>.
          </p>

          <fieldset style={{ border: 0, padding: 0, margin: 0 }}>
            <legend className="label cr-muted" style={{ marginBottom: "var(--space-2)" }}>Permiso que concedes</legend>
            <label className={`permiso${permiso === "read" ? " elegido" : ""}`}>
              <input type="radio" name="permiso" checked={permiso === "read"} onChange={() => setPermiso("read")} />
              <span>
                <span className="label">Solo leer (recomendado)</span>
                <span className="caption cr-muted">Puede listar y leer tus skills y documentos.</span>
              </span>
            </label>
            <label className={`permiso${permiso === "write" ? " elegido" : ""}`}>
              <input type="radio" name="permiso" checked={permiso === "write"} onChange={() => setPermiso("write")} />
              <span>
                <span className="label">Leer y guardar documentos</span>
                <span className="caption cr-muted">
                  Además puede crear y actualizar documentos .md; el historial se conserva.
                  {c.requestedWrite ? " Esta aplicación lo ha pedido." : ""}
                </span>
              </span>
            </label>
          </fieldset>

          {error && <Alerta tipo="error">{error}</Alerta>}

          <div className="consentimiento-acciones">
            <Button variant="ghost" onClick={() => void decidir(false)} disabled={enviando !== null}>
              {enviando === "no" ? "Cancelando…" : "Cancelar"}
            </Button>
            <Button variant="primary" onClick={() => void decidir(true)} disabled={enviando !== null}>
              <Icono nombre="check" tamano={15} />
              {enviando === "si" ? "Autorizando…" : "Autorizar"}
            </Button>
          </div>
          <p className="caption cr-muted">Podrás quitarle el acceso cuando quieras desde Espacio de trabajo → Claude.</p>
        </Glass>
      )}
    </div>
  );
}
