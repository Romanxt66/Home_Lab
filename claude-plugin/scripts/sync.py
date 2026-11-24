#!/usr/bin/env python3
"""Baja tus skills de Home Lab a ~/.claude/skills.

Solo usa la biblioteca estándar. Se configura con variables de entorno:

  HOMELAB_TOKEN  el token personal (obligatorio; nunca se pasa por argumento,
                 porque los argumentos se ven en la lista de procesos)
  HOMELAB_URL    dirección de Home Lab (por defecto https://lab.softlane.click)

Cuida tres cosas, porque descarga archivos de un servidor y los deja donde
Claude los obedecerá:

  * Nunca pisa una skill local que no haya creado él (la reconoce por el archivo
    de estado .homelab-sync.json). Con --force sí.
  * Solo acepta nombres de skill seguros y rutas dentro de su carpeta; un zip con
    «../» o enlaces simbólicos se rechaza.
  * Reemplaza cada skill de golpe (se extrae aparte y se renombra): una descarga
    cortada no deja una skill a medias.
"""
import argparse
import io
import json
import os
import re
import shutil
import stat
import sys
import tempfile
import urllib.error
import urllib.request
import zipfile

URL_POR_DEFECTO = "https://lab.softlane.click"
ESTADO = ".homelab-sync.json"
NOMBRE_VALIDO = re.compile(r"^[a-z0-9][a-z0-9_-]{0,63}$")

# Mismos límites que el servidor (un poco más holgados): defienden de un zip hostil.
MAX_ZIP = 15 << 20
MAX_TOTAL = 30 << 20
MAX_ARCHIVOS = 600
MAX_ARCHIVO = 10 << 20
TIEMPO_ESPERA = 20


class Fallo(Exception):
    """Un error que se explica a la persona tal cual, sin traza."""


def config():
    token = os.environ.get("HOMELAB_TOKEN", "").strip()
    if not token:
        raise Fallo(
            "Falta HOMELAB_TOKEN. Crea un token en Home Lab (Espacio de trabajo → Claude) y expórtalo:\n"
            '  export HOMELAB_TOKEN="hl_…"'
        )
    url = os.environ.get("HOMELAB_URL", URL_POR_DEFECTO).strip().rstrip("/")
    if not url.startswith(("https://", "http://")):
        raise Fallo("HOMELAB_URL debe empezar por https:// (o http:// en local).")
    return url, token


def pedir(url, token, maximo):
    req = urllib.request.Request(url, headers={"Authorization": "Bearer " + token, "User-Agent": "homelab-sync/1"})
    try:
        with urllib.request.urlopen(req, timeout=TIEMPO_ESPERA) as r:
            datos = r.read(maximo + 1)
            if len(datos) > maximo:
                raise Fallo("La respuesta del servidor es demasiado grande.")
            return datos, r.headers
    except urllib.error.HTTPError as e:
        if e.code == 401:
            raise Fallo("El servidor rechazó el token: no es válido o fue revocado. Crea otro en Home Lab.")
        if e.code == 404:
            raise Fallo("Home Lab no tiene la ruta de sincronización: ¿está actualizado el servidor?")
        raise Fallo("Home Lab respondió con el error %d." % e.code)
    except (urllib.error.URLError, TimeoutError, OSError) as e:
        raise Fallo("No se pudo conectar con %s (%s)." % (url.split("/api/")[0], getattr(e, "reason", e)))


def cargar_estado(carpeta):
    try:
        with open(os.path.join(carpeta, ESTADO), encoding="utf-8") as f:
            d = json.load(f)
        return d.get("skills", {}) if isinstance(d, dict) else {}
    except (OSError, ValueError):
        return {}


def guardar_estado(carpeta, skills):
    os.makedirs(carpeta, exist_ok=True)
    fd, tmp = tempfile.mkstemp(dir=carpeta, prefix=".estado-")
    with os.fdopen(fd, "w", encoding="utf-8") as f:
        json.dump({"skills": skills}, f, indent=2, ensure_ascii=False)
    os.replace(tmp, os.path.join(carpeta, ESTADO))


def extraer(datos, destino, nombre):
    """Extrae el zip de una skill en `destino` (que no existe aún), validándolo."""
    try:
        zf = zipfile.ZipFile(io.BytesIO(datos))
    except zipfile.BadZipFile:
        raise Fallo("«%s»: el servidor devolvió algo que no es un zip." % nombre)
    infos = [i for i in zf.infolist() if not i.is_dir()]
    if not infos or len(infos) > MAX_ARCHIVOS:
        raise Fallo("«%s»: el zip está vacío o tiene demasiados archivos." % nombre)

    prefijo = nombre + "/"
    total = 0
    os.makedirs(destino)
    base = os.path.realpath(destino)
    for info in infos:
        ruta = info.filename.replace("\\", "/")
        if stat.S_ISLNK(info.external_attr >> 16):
            raise Fallo("«%s»: el zip contiene un enlace simbólico." % nombre)
        if not ruta.startswith(prefijo):
            raise Fallo("«%s»: el zip trae una ruta fuera de su carpeta: %r." % (nombre, ruta[:60]))
        relativa = ruta[len(prefijo):]
        partes = relativa.split("/")
        if not relativa or relativa.startswith("/") or ":" in relativa or any(p in ("", ".", "..") for p in partes):
            raise Fallo("«%s»: ruta no permitida en el zip: %r." % (nombre, ruta[:60]))
        if info.file_size > MAX_ARCHIVO:
            raise Fallo("«%s»: un archivo pesa demasiado." % nombre)

        objetivo = os.path.realpath(os.path.join(base, *partes))
        if not objetivo.startswith(base + os.sep):
            raise Fallo("«%s»: ruta que se sale de su carpeta: %r." % (nombre, ruta[:60]))
        os.makedirs(os.path.dirname(objetivo), exist_ok=True)
        # Se cuenta lo realmente leído, no lo que dice la cabecera (bombas de descompresión).
        with zf.open(info) as origen:
            contenido = origen.read(MAX_ARCHIVO + 1)
        if len(contenido) > MAX_ARCHIVO:
            raise Fallo("«%s»: un archivo pesa demasiado." % nombre)
        total += len(contenido)
        if total > MAX_TOTAL:
            raise Fallo("«%s»: la skill descomprimida pesa demasiado." % nombre)
        with open(objetivo, "wb") as f:
            f.write(contenido)
        # El servidor no guarda permisos; los scripts con shebang se dejan ejecutables.
        if contenido.startswith(b"#!"):
            os.chmod(objetivo, 0o755)
    if not os.path.isfile(os.path.join(base, "SKILL.md")):
        raise Fallo("«%s»: el zip no trae SKILL.md." % nombre)


def reemplazar(carpeta, nombre, datos):
    """Extrae aparte y cambia de golpe, sin dejar nunca la skill a medias."""
    final = os.path.join(carpeta, nombre)
    nueva = os.path.join(carpeta, ".%s.nueva-%d" % (nombre, os.getpid()))
    vieja = os.path.join(carpeta, ".%s.vieja-%d" % (nombre, os.getpid()))
    shutil.rmtree(nueva, ignore_errors=True)
    try:
        extraer(datos, nueva, nombre)
        if os.path.isdir(final):
            os.rename(final, vieja)
        os.rename(nueva, final)
    except BaseException:
        if os.path.isdir(vieja) and not os.path.isdir(final):
            os.rename(vieja, final)  # se deshace si falló a medias
        raise
    finally:
        shutil.rmtree(nueva, ignore_errors=True)
        shutil.rmtree(vieja, ignore_errors=True)


def planificar(manifiesto, estado, carpeta, force):
    """Decide qué hacer con cada skill. Devuelve una lista de (accion, item)."""
    plan = []
    for item in manifiesto:
        nombre = item.get("name", "")
        if not NOMBRE_VALIDO.match(nombre):
            plan.append(("invalida", item))
            continue
        final = os.path.join(carpeta, nombre)
        existe = os.path.isdir(final)
        previo = estado.get(nombre)
        if existe and (previo is None or previo.get("id") != item.get("id")) and not force:
            plan.append(("conflicto", item))
        elif existe and previo and previo.get("id") == item.get("id") and previo.get("version") == item.get("version"):
            plan.append(("al_dia", item))
        elif existe:
            plan.append(("actualizar", item))
        else:
            plan.append(("nueva", item))
    return plan


def main(argv=None):
    p = argparse.ArgumentParser(description="Baja tus skills de Home Lab a ~/.claude/skills.")
    p.add_argument("--dir", default=os.path.join(os.path.expanduser("~"), ".claude", "skills"), help="carpeta de destino")
    p.add_argument("--dry-run", action="store_true", help="solo enseña qué haría")
    p.add_argument("--force", action="store_true", help="pisa también skills locales con el mismo nombre que no creó este script")
    p.add_argument("--prune", action="store_true", help="quita las que bajó antes y ya no están en Home Lab")
    p.add_argument("--check", action="store_true", help="solo cuenta cuántas hay nuevas o cambiadas (JSON) y no escribe nada")
    p.add_argument("--quiet", action="store_true", help="solo escribe si hay algo que decir")
    a = p.parse_args(argv)

    try:
        url, token = config()
        crudo, _ = pedir(url + "/api/sync/manifest", token, 2 << 20)
        try:
            manifiesto = json.loads(crudo)["skills"]
        except (ValueError, KeyError, TypeError):
            raise Fallo("La respuesta del servidor no es la esperada.")
        carpeta = os.path.abspath(os.path.expanduser(a.dir))
        estado = cargar_estado(carpeta)
        plan = planificar(manifiesto, estado, carpeta, a.force)

        if a.check:
            pendientes = [i["name"] for acc, i in plan if acc in ("nueva", "actualizar")]
            print(json.dumps({"total": len(manifiesto), "pendientes": pendientes, "conflictos": [i["name"] for acc, i in plan if acc == "conflicto"]}))
            return 0

        nuevas = actualizadas = 0
        errores = []
        salida = []
        for accion, item in plan:
            n = item.get("name", "?")
            if accion == "invalida":
                salida.append("  ✗ %r: el servidor dio un nombre que no es seguro; se omite." % n[:40])
                errores.append(n)
            elif accion == "conflicto":
                salida.append("  ! %s: ya tienes una skill local con ese nombre que no vino de Home Lab; se deja como está (--force para pisarla)." % n)
            elif accion == "al_dia":
                if not a.quiet:
                    salida.append("  = %s (v%s): al día" % (n, item["version"]))
            elif a.dry_run:
                salida.append("  %s %s (v%s): se %s" % ("+" if accion == "nueva" else "↑", n, item["version"], "bajaría" if accion == "nueva" else "actualizaría"))
            else:
                try:
                    datos, _ = pedir(url + "/api/sync/skills/" + item["id"], token, MAX_ZIP)
                    os.makedirs(carpeta, exist_ok=True)
                    reemplazar(carpeta, n, datos)
                    estado[n] = {"id": item["id"], "version": item["version"]}
                    guardar_estado(carpeta, estado)  # tras cada skill: si algo falla, lo hecho queda anotado
                    if accion == "nueva":
                        nuevas += 1
                        salida.append("  + %s (v%s): descargada" % (n, item["version"]))
                    else:
                        actualizadas += 1
                        salida.append("  ↑ %s (v%s): actualizada" % (n, item["version"]))
                except Fallo as e:
                    errores.append(n)
                    salida.append("  ✗ %s" % e)

        remotos = {i.get("name") for i in manifiesto}
        for n in sorted(set(estado) - remotos):
            if a.prune and not a.dry_run:
                if NOMBRE_VALIDO.match(n):
                    shutil.rmtree(os.path.join(carpeta, n), ignore_errors=True)
                estado.pop(n, None)
                guardar_estado(carpeta, estado)
                salida.append("  - %s: ya no está en Home Lab; quitada" % n)
            else:
                salida.append("  ? %s: ya no está en Home Lab (--prune para quitarla de aquí)" % n)

        if salida or not a.quiet:
            print("Home Lab → %s" % carpeta)
            print("\n".join(salida) if salida else "  (no tienes skills guardadas en Home Lab todavía)")
            if not a.dry_run:
                print("%d nuevas, %d actualizadas%s." % (nuevas, actualizadas, ", %d con error" % len(errores) if errores else ""))
                if nuevas or actualizadas:
                    print("Las skills nuevas se cargan al abrir una sesión nueva de Claude Code.")
        return 1 if errores else 0
    except Fallo as e:
        print("Error: %s" % e, file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
