/** Tipos que devuelve la API. Reflejan los DTO de Go uno a uno. */

export interface Usuario {
  id: string;
  email: string;
  name: string | null;
  role: string;
}

export interface Evento {
  id: string;
  title: string;
  description: string | null;
  start: string;
  end: string | null;
  allDay: boolean;
  location: string | null;
  color: string | null;
  remindMinutesBefore: number | null;
}

/** Lo que se envía al crear o editar un evento. */
export interface EventoInput {
  title: string;
  description: string | null;
  start: string;
  end: string | null;
  allDay: boolean;
  location: string | null;
  remindMinutesBefore: number | null;
}

export type RolUsuario = "user" | "superadmin";
export type EstadoUsuario = "pending" | "approved" | "rejected";

/** Una cuenta tal como la ve la administración. Nunca lleva la contraseña. */
export interface UsuarioAdmin {
  id: string;
  email: string;
  name: string | null;
  picture: string | null;
  role: RolUsuario;
  status: EstadoUsuario;
  /** Las cuentas que solo entran con Google no tienen contraseña. */
  authMethod: "google" | "password";
  createdAt: string;
  /** Es la cuenta de quien mira: no se puede rechazar ni cambiar de rol. */
  isSelf: boolean;
}

export type AccionTarea = "email" | "scrape";

/** Parámetros de una tarea. Cada acción espera los suyos; el resto se conserva. */
export interface ParametrosTarea {
  // email
  recipients?: string;
  subject?: string;
  body?: string;
  googleAccountId?: string;
  // scrape
  url?: string;
  selector?: string;
  attribute?: string;
  [otro: string]: unknown;
}

export interface Tarea {
  id: string;
  name: string;
  cron: string;
  action: AccionTarea;
  params: ParametrosTarea;
  enabled: boolean;
  createdAt: string;
}

export interface TareaInput {
  name: string;
  cron: string;
  action: AccionTarea;
  params: ParametrosTarea;
  enabled: boolean;
}

export interface EjecucionTarea {
  id: string;
  jobId: string;
  jobName: string;
  status: "success" | "error";
  output: string | null;
  startedAt: string;
}

export interface CuentaGmail {
  id: string;
  email: string;
  name: string | null;
  picture: string | null;
}

export interface ConfigTelegram {
  id: string;
  provider: string;
  recipient: string;
  active: boolean;
  hasCredential: boolean;
}

export interface ResultadoScraper {
  status: number;
  contentType: string;
  items?: string[];
  raw?: string;
}

export interface Plantilla {
  id: string;
  name: string;
  subject: string;
  body: string;
  variables: string[];
  updatedAt: string;
}

export interface ResumenEnvio {
  sent: number;
  failed: number;
}

export interface ConfigGitHub {
  id: string;
  username: string;
  hasToken: boolean;
}

export interface PerfilGitHub {
  login: string;
  name: string | null;
  avatarUrl: string;
  bio: string | null;
  htmlUrl: string;
  publicRepos: number;
  followers: number;
  following: number;
  company: string | null;
  location: string | null;
  blog: string | null;
}

export interface RepoGitHub {
  id: number;
  name: string;
  fullName: string;
  description: string | null;
  url: string;
  homepage: string | null;
  isPrivate: boolean;
  isFork: boolean;
  isArchived: boolean;
  language: string | null;
  stars: number;
  forks: number;
  openIssues: number;
  topics: string[];
  pushedAt: string | null;
  defaultBranch: string;
}

export interface ResumenGitHub {
  profile: PerfilGitHub;
  repos: RepoGitHub[];
}

export type TipoAccionRegla = "telegram" | "calendar_event" | "n8n_webhook";

export interface AccionRegla {
  type: TipoAccionRegla;
  /** Todos los valores son texto, también los números: así se guardan. */
  config: Record<string, string>;
}

export interface Regla {
  id: string;
  name: string;
  trigger: string;
  active: boolean;
  actions: AccionRegla[];
  createdAt: string;
}

export interface ReglaInput {
  name: string;
  trigger: string;
  active: boolean;
  actions: AccionRegla[];
}

export interface DisparadorRegla {
  type: string;
  label: string;
  description: string;
  variables: string[];
}

export interface EjecucionRegla {
  ok: boolean;
  error: string | null;
  ranAt: string;
}

export interface DeteccionDespliegue {
  runtime: string;
  framework: string;
  packageManager: string;
  runtimeVersion: string;
  installCommand: string;
  buildCommand: string;
  startCommand: string;
  port: number;
  isStatic: boolean;
  publishDir: string;
  needsDatabase: boolean;
  databaseType: string;
  hasDockerfile: boolean;
  hasCompose: boolean;
  recommendedBuildPack: string;
  notes: string[];
}

export interface AnalisisDespliegue {
  owner: string;
  repo: string;
  branch: string;
  baseDir: string;
  detection: DeteccionDespliegue;
  dockerfile: string;
  compose: string | null;
}

export interface ConfigCoolify {
  id: string;
  baseUrl: string;
  hasToken: boolean;
}

export interface AppCoolify {
  uuid: string;
  name: string;
  description: string | null;
  fqdn: string | null;
  status: string;
  state: string;
  stateLabel: string;
  healthy: boolean | null;
  gitRepository: string | null;
  gitBranch: string | null;
  buildPack: string | null;
  projectName: string | null;
  environmentName: string | null;
  portsExposes: string;
  installCommand: string;
  buildCommand: string;
  startCommand: string;
  baseDirectory: string;
}

export interface RecursoCoolify {
  uuid: string;
  name: string;
  type: string;
  status: string;
  state: string;
  stateLabel: string;
}

export interface ProyectoCoolify {
  uuid: string;
  name: string;
  description: string | null;
}

export interface ServidorCoolify {
  uuid: string;
  name: string;
  ip: string;
  reachable: boolean;
}

export interface ResumenCoolify {
  apps: AppCoolify[];
  projects: ProyectoCoolify[];
  servers: ServidorCoolify[];
  databases: RecursoCoolify[];
  services: RecursoCoolify[];
}

export interface CatalogoCoolify {
  databaseTypes: { id: string; label: string }[];
  buildPacks: { id: string; label: string }[];
}

export interface DespliegueCoolify {
  uuid: string;
  applicationName: string;
  serverName: string;
  status: "queued" | "in_progress" | "finished" | "failed" | "cancelled";
  statusLabel: string;
  rawStatus: string;
  commit: string;
  commitMessage: string;
  createdAt: string;
}

export interface VariableCoolify {
  uuid: string;
  key: string;
  value: string;
  isBuildTime: boolean;
  isPreview: boolean;
  isLiteral: boolean;
}

export interface EntornoCoolify {
  uuid: string;
  name: string;
}
