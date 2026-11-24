#!/usr/bin/env python3
"""Al abrir una sesión: avisa si hay skills nuevas en Home Lab, o las baja si HOMELAB_AUTOSYNC=1.

Nunca estorba: sin token no hace nada, y si el servidor no responde (o tarda más
de unos segundos) la sesión empieza igual, en silencio.
"""
import contextlib
import io
import json
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "scripts"))
import sync  # noqa: E402


def responder(contexto=None):
    if not contexto:
        print("{}")
        return
    print(json.dumps({"hookSpecificOutput": {"hookEventName": "SessionStart", "additionalContext": contexto}}, ensure_ascii=False))


def ejecutar(argumentos):
    """Corre el sincronizador capturando lo que escribe. Devuelve (código, texto)."""
    salida = io.StringIO()
    with contextlib.redirect_stdout(salida), contextlib.redirect_stderr(io.StringIO()):
        codigo = sync.main(argumentos)
    return codigo, salida.getvalue().strip()


def main():
    try:
        sys.stdin.read()  # el evento no hace falta, pero se vacía la entrada
    except OSError:
        pass
    if not os.environ.get("HOMELAB_TOKEN", "").strip():
        responder()
        return 0
    sync.TIEMPO_ESPERA = 5  # que una red lenta no retrase el inicio de la sesión

    try:
        if os.environ.get("HOMELAB_AUTOSYNC") == "1":
            codigo, texto = ejecutar(["--quiet"])
            if texto:
                responder("Home Lab sincronizó tus skills al abrir la sesión:\n" + texto)
            else:
                responder()
            return 0

        codigo, texto = ejecutar(["--check"])
        if codigo != 0:
            responder()
            return 0
        estado = json.loads(texto)
        if estado["pendientes"]:
            nombres = ", ".join(estado["pendientes"][:8])
            responder(
                "Home Lab: hay %d skill(s) nuevas o actualizadas en tu espacio de trabajo (%s). "
                "Si el usuario quiere traerlas, puede ejecutar /homelab:sync. No las sincronices por tu cuenta."
                % (len(estado["pendientes"]), nombres)
            )
        else:
            responder()
    except Exception:  # un fallo aquí nunca debe impedir que la sesión empiece
        responder()
    return 0


if __name__ == "__main__":
    sys.exit(main())
