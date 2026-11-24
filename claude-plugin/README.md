# Plugin de Claude Code para Home Lab

Enlaza Claude Code con el **espacio de trabajo** de Home Lab: lee tus skills y
documentos, guarda documentos y baja tus skills a `~/.claude/skills`.

## Qué trae

| Pieza | Para qué |
|---|---|
| **Servidor MCP** `homelab` | Las herramientas `list_skills`, `get_skill`, `read_skill_file`, `list_docs`, `read_doc` y (con un token de escritura) `save_doc`. |
| `/homelab:sync` | Baja tus skills nuevas o cambiadas a `~/.claude/skills`, con sus fuentes e imágenes. |
| `/homelab:doc <ruta> [destino]` | Trae un documento (por ejemplo un `CLAUDE.md`) a la carpeta actual. |
| `/homelab:save <archivo> [ruta]` | Guarda un archivo local como documento, con historial. Antes revisa que no lleve secretos. |
| Skill `workspace` | Le explica a Claude cuándo y cómo usar las herramientas. |
| Aviso al iniciar sesión | Si hay skills nuevas en Home Lab lo dice, sin bajarlas. Con `HOMELAB_AUTOSYNC=1` las baja solo. |

## Puesta en marcha

1. En Home Lab, *Espacio de trabajo → Claude → Nuevo token*. Elige «solo leer» si
   solo vas a leer y sincronizar, o «leer y guardar documentos» para `/homelab:save`.
2. Exporta el token (y la dirección, si no es la de por defecto) **antes de abrir
   Claude Code**, por ejemplo en tu `~/.bashrc` o `~/.zshrc`:

   ```bash
   export HOMELAB_TOKEN="hl_…"
   export HOMELAB_URL="https://lab.softlane.click"   # opcional: es el valor por defecto
   ```

3. Instala el plugin. La forma más simple es enlazarlo en la carpeta de skills,
   desde donde se carga solo en la sesión siguiente:

   ```bash
   ln -s /ruta/a/Home_Lab/claude-plugin ~/.claude/skills/homelab
   ```

   (Para probarlo sin instalar: `claude --plugin-dir /ruta/a/Home_Lab/claude-plugin`.)

4. En Claude Code, `/mcp` debe enseñar `homelab` como conectado y `/homelab:sync --dry-run`
   te dice qué bajaría.

## Cómo cuida lo que baja

Una skill es texto que Claude obedece, así que el sincronizador es prudente:

- **No pisa** una skill local que no haya creado él (lo sabe por
  `~/.claude/skills/.homelab-sync.json`). `--force` lo permite expresamente.
- Solo baja **tus** skills, nunca las que comparten otras personas.
- Acepta únicamente nombres seguros y rutas dentro de la carpeta de la skill:
  rechaza zips con `../`, rutas absolutas, enlaces simbólicos o tamaños absurdos.
- Cada skill se extrae aparte y se cambia **de golpe**: una descarga cortada no
  deja una skill a medias, y si una actualización falla se conserva la anterior.
- El token se lee solo de la variable de entorno: nunca va en argumentos (se verían
  en la lista de procesos) ni en los mensajes de error.

## Opciones de `sync.py`

`--dry-run` (solo enseña), `--prune` (quita las que ya no están en Home Lab, solo
las que bajó él), `--force`, `--check` (cuenta pendientes en JSON), `--quiet`,
`--dir` (otra carpeta de destino). Se puede usar sin Claude:
`python3 scripts/sync.py --dry-run`.

## Pruebas

```bash
python3 -m unittest discover -s claude-plugin/tests -v
claude plugin validate claude-plugin --strict
```

Las pruebas levantan un Home Lab de mentira y cubren la sincronización, los zips
hostiles (zip-slip, enlaces, bombas), los conflictos, las actualizaciones rotas y
el aviso de inicio.
