"""Pruebas del aviso de inicio de sesión (hooks-handlers/on-session-start.py)."""
import contextlib
import importlib.util
import io
import json
import os
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(__file__))
from test_sync import TOKEN, Servidor, leer  # noqa: E402

RUTA = os.path.join(os.path.dirname(__file__), "..", "hooks-handlers", "on-session-start.py")


def cargar():
    spec = importlib.util.spec_from_file_location("hook", RUTA)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


class Aviso(unittest.TestCase):
    def setUp(self):
        self.srv = Servidor()
        self.tmp = tempfile.TemporaryDirectory()
        self.dir = os.path.join(self.tmp.name, "skills")
        self.env = mock.patch.dict(os.environ, {"HOMELAB_URL": self.srv.url, "HOMELAB_TOKEN": TOKEN}, clear=False)
        self.env.start()
        os.environ.pop("HOMELAB_AUTOSYNC", None)
        # El hook usa la carpeta por defecto (~/.claude/skills): se redirige a una temporal.
        self.home = mock.patch("os.path.expanduser", lambda p: p.replace("~", self.tmp.name))
        self.home.start()
        self.hook = cargar()

    def tearDown(self):
        self.home.stop()
        self.env.stop()
        self.srv.cerrar()
        self.tmp.cleanup()

    def correr(self):
        out = io.StringIO()
        with contextlib.redirect_stdout(out), mock.patch("sys.stdin", io.StringIO("{}")):
            codigo = self.hook.main()
        return codigo, json.loads(out.getvalue())

    def carpeta(self):
        return os.path.join(self.tmp.name, ".claude", "skills")

    def test_sin_token_no_hace_nada(self):
        with mock.patch.dict(os.environ, {"HOMELAB_TOKEN": ""}):
            self.assertEqual(self.correr(), (0, {}))

    def test_avisa_de_las_pendientes_sin_bajarlas(self):
        self.srv.poner("1", "alfa", 1)
        self.srv.poner("2", "beta", 1)
        codigo, r = self.correr()
        ctx = r["hookSpecificOutput"]["additionalContext"]
        self.assertEqual(codigo, 0)
        self.assertEqual(r["hookSpecificOutput"]["hookEventName"], "SessionStart")
        self.assertIn("2 skill", ctx)
        self.assertIn("alfa", ctx)
        self.assertIn("/homelab:sync", ctx)
        self.assertIn("No las sincronices por tu cuenta", ctx)
        self.assertEqual(self.srv.descargas, [], "sin autosync no se baja nada")
        self.assertFalse(os.path.exists(self.carpeta()))

    def test_si_todo_esta_al_dia_no_gasta_contexto(self):
        self.srv.poner("1", "alfa", 1)
        with mock.patch.dict(os.environ, {"HOMELAB_AUTOSYNC": "1"}):
            self.correr()
        self.assertEqual(self.correr(), (0, {}))

    def test_autosync_baja_y_lo_cuenta(self):
        self.srv.poner("1", "alfa", 1)
        with mock.patch.dict(os.environ, {"HOMELAB_AUTOSYNC": "1"}):
            codigo, r = self.correr()
        self.assertIn("alfa", r["hookSpecificOutput"]["additionalContext"])
        self.assertTrue(os.path.isfile(os.path.join(self.carpeta(), "alfa", "SKILL.md")))
        self.assertEqual(self.srv.descargas, ["1"])

    def test_servidor_caido_no_estorba(self):
        with mock.patch.dict(os.environ, {"HOMELAB_URL": "http://127.0.0.1:1"}):
            self.assertEqual(self.correr(), (0, {}))

    def test_token_rechazado_no_estorba(self):
        self.srv.token_ok = "otro"
        self.assertEqual(self.correr(), (0, {}))

    def test_nunca_lanza_una_excepcion(self):
        with mock.patch.object(self.hook.sync, "main", side_effect=RuntimeError("boom")):
            self.assertEqual(self.correr(), (0, {}))


if __name__ == "__main__":
    unittest.main()
