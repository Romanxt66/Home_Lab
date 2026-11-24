---
name: doc
description: Trae un documento .md de Home Lab al proyecto actual (por ejemplo un CLAUDE.md). Úsala cuando el usuario escriba /homelab:doc o pida traer un documento de su homelab a esta carpeta.
disable-model-invocation: true
argument-hint: "<ruta en Home Lab> [destino local]"
---

# Traer un documento de Home Lab

Argumentos: `$ARGUMENTS` — el primero es la ruta del documento en Home Lab (por ejemplo `proyectos/homelab/CLAUDE.md`); el segundo, opcional, es dónde escribirlo en esta carpeta.

1. Si el usuario no dio la ruta, llama a `list_docs` (servidor MCP `homelab`) y enséñale las opciones para que elija.
2. Lee el documento con `read_doc`. Si dice que no existe, muestra las rutas con `list_docs`.
3. Decide el destino: el que dé el usuario o, si no, `./<nombre-del-archivo>` (por ejemplo `./CLAUDE.md`).
4. **Si el destino ya existe, no lo sobrescribas sin preguntar**: enséñale las diferencias en pocas líneas y espera su respuesta.
5. Escribe el archivo y confirma la ruta.

Un `CLAUDE.md` es un archivo que Claude obedece en cada sesión: solo traes documentos que son del propio usuario (los que salen de `list_docs` sin `include_shared`). Si el usuario pide uno de otra persona, avísale de que es de un tercero y de que conviene leerlo entero antes de dejarlo en el proyecto.
