---
name: save
description: Guarda un archivo local (por ejemplo un CLAUDE.md) como documento en Home Lab, con historial. Úsala cuando el usuario escriba /homelab:save o pida guardar un documento en su homelab.
disable-model-invocation: true
argument-hint: "<archivo> [ruta en Home Lab]"
---

# Guardar un archivo en Home Lab

Argumentos: `$ARGUMENTS` — el primero es el archivo local; el segundo, opcional, es la ruta con la que se guardará en Home Lab (por ejemplo `proyectos/homelab/CLAUDE.md`).

1. Lee el archivo con la herramienta Read. Si no existe o no es texto, díselo al usuario y para.
2. **Revisa que no lleve secretos** (contraseñas, tokens, claves de API, cadenas de conexión con usuario y clave, claves privadas). Si encuentras alguno, enséñale dónde y pregúntale si quiere guardarlo igualmente; por defecto no lo guardes.
3. Decide la ruta de destino: la que dé el usuario o, si no da ninguna, propón `proyectos/<nombre-de-la-carpeta-del-proyecto>/<nombre-del-archivo>` y confírmala. La ruta usa «/» para las carpetas y debe acabar en `.md`.
4. Guarda el contenido con la herramienta `save_doc` del servidor MCP `homelab`, con un `message` corto que diga qué se guarda. No pases `shared`: se guarda en privado; si el usuario quiere compartirlo, que lo haga desde la pantalla de Home Lab.
5. Responde con la ruta y el número de versión que devolvió la herramienta. Si dice que el token es de solo lectura, explica que hace falta uno con permiso de escritura.

Guardar sobre un documento existente crea una versión nueva, no lo destruye: el historial se conserva.
