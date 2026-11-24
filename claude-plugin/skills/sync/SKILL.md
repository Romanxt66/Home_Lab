---
name: sync
description: Descarga tus skills guardadas en Home Lab a ~/.claude/skills (solo las nuevas o cambiadas). Úsala cuando el usuario escriba /homelab:sync o pida bajar o sincronizar sus skills del homelab.
disable-model-invocation: true
argument-hint: "[--dry-run] [--prune] [--force]"
allowed-tools: Bash(python3 *sync.py*)
---

# Sincronizar skills de Home Lab

Ejecuta este comando tal cual y cuéntale al usuario el resultado en pocas líneas:

```bash
python3 "${CLAUDE_PLUGIN_ROOT}/scripts/sync.py" $ARGUMENTS
```

Reglas:

- Pasa **solo** las opciones que el usuario haya escrito y que estén en esta lista: `--dry-run` (enseña qué haría sin tocar nada), `--prune` (quita las que ya no están en Home Lab), `--force` (pisa skills locales con el mismo nombre). Nada más.
- Si no hay opciones, no añadas ninguna. Si el usuario duda, sugiérele empezar con `--dry-run`.
- El script nunca pisa una skill local que no haya bajado él, salvo con `--force`; si te dice que omitió alguna por eso, explícalo y no uses `--force` por tu cuenta.
- Si el error es que falta `HOMELAB_TOKEN`, explica cómo crearlo: en Home Lab, *Espacio de trabajo → Claude → Nuevo token*, y exportarlo con `export HOMELAB_TOKEN="hl_…"` antes de abrir Claude Code.
- Termina recordando que las skills nuevas se cargan al abrir una sesión nueva.
