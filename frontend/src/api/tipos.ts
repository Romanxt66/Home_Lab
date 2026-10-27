/** Tipos que devuelve la API. Reflejan los DTO de Go uno a uno. */

export interface Usuario {
  id: string;
  email: string;
  name: string | null;
  role: string;
}
