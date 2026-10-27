import { createContext, useCallback, useContext, useEffect, useState } from "react";
import type { ReactNode } from "react";

import { api, SinSesionError } from "./client";
import type { Usuario } from "./tipos";

interface Sesion {
  usuario: Usuario | null;
  /** true mientras se comprueba si hay sesión al cargar la página. */
  cargando: boolean;
  entrar(email: string, password: string): Promise<void>;
  salir(): Promise<void>;
}

const Contexto = createContext<Sesion | null>(null);

export function ProveedorSesion({ children }: { children: ReactNode }) {
  const [usuario, setUsuario] = useState<Usuario | null>(null);
  const [cargando, setCargando] = useState(true);

  // Al abrir la aplicación se pregunta quién soy: la cookie puede venir de
  // una visita anterior o del propio Lab en Next, que firma igual.
  useEffect(() => {
    api
      .get<Usuario>("/auth/me")
      .then(setUsuario)
      .catch((e) => {
        // Que no haya sesión es lo normal, no un fallo que reportar.
        if (!(e instanceof SinSesionError)) console.error(e);
        setUsuario(null);
      })
      .finally(() => setCargando(false));
  }, []);

  const entrar = useCallback(async (email: string, password: string) => {
    setUsuario(await api.post<Usuario>("/auth/login", { email, password }));
  }, []);

  const salir = useCallback(async () => {
    await api.post("/auth/logout");
    setUsuario(null);
  }, []);

  return (
    <Contexto.Provider value={{ usuario, cargando, entrar, salir }}>
      {children}
    </Contexto.Provider>
  );
}

export function useSesion(): Sesion {
  const valor = useContext(Contexto);
  if (!valor) {
    throw new Error("useSesion debe usarse dentro de <ProveedorSesion>");
  }
  return valor;
}
