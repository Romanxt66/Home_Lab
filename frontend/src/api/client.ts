/**
 * Cliente HTTP contra la API en Go.
 *
 * Todo pasa por aquí para que el manejo de errores y las credenciales se
 * decidan en un solo sitio. Ningún componente llama a `fetch` directamente.
 */

const BASE = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

/** Error con el mensaje que la API preparó para mostrar al usuario. */
export class ApiError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

/** 401: no hay sesión. Se trata aparte porque obliga a volver al login. */
export class SinSesionError extends ApiError {
  constructor(message: string) {
    super(message, 401);
    this.name = "SinSesionError";
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  let res: Response;

  try {
    res = await fetch(`${BASE}/api${path}`, {
      ...init,
      // Imprescindible: la sesión va en una cookie HttpOnly y sin esto el
      // navegador no la manda, porque la API está en otro origen.
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        ...init.headers,
      },
    });
  } catch {
    // Distinguir "no hay red" de "la API respondió con error" importa: el
    // mensaje que ve el usuario es distinto y la solución también.
    throw new ApiError("No se pudo conectar con el servidor.", 0);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  const cuerpo = await res.json().catch(() => null);

  if (!res.ok) {
    const mensaje =
      (cuerpo as { error?: string } | null)?.error ?? "Ocurrió un error inesperado.";
    if (res.status === 401) throw new SinSesionError(mensaje);
    throw new ApiError(mensaje, res.status);
  }

  return cuerpo as T;
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "POST", body: body ? JSON.stringify(body) : undefined }),
  put: <T>(path: string, body: unknown) =>
    request<T>(path, { method: "PUT", body: JSON.stringify(body) }),
  del: (path: string) => request<void>(path, { method: "DELETE" }),
};
