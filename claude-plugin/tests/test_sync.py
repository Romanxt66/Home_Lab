"""Pruebas de scripts/sync.py contra un servidor falso. Se ejecutan con:

    python3 -m unittest discover -s claude-plugin/tests -v
"""
import contextlib
import io
import json
import os
import stat
import sys
import tempfile
import threading
import unittest
import zipfile
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from unittest import mock

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "scripts"))
import sync  # noqa: E402

TOKEN = "hl_" + "a" * 43


def hacer_zip(nombre, archivos, prefijo=None, extra=None):
    """Zip como el del servidor: todo dentro de una carpeta con el nombre de la skill."""
    prefijo = (nombre + "/") if prefijo is None else prefijo
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as z:
        for ruta, cont in archivos.items():
            z.writestr(prefijo + ruta, cont)
        for info, cont in (extra or []):
            z.writestr(info, cont)
    return buf.getvalue()


def skill_md(nombre):
    return "---\nname: %s\ndescription: Hace algo\n---\ncuerpo\n" % nombre


class Servidor:
    """Un Home Lab de mentira: manifiesto y descargas con contadores."""

    def __init__(self):
        self.skills = {}  # id -> {name, version, zip}
        self.descargas = []
        self.token_ok = TOKEN
        servidor = self

        class H(BaseHTTPRequestHandler):
            def log_message(self, *a):
                pass

            def do_GET(self):
                if self.headers.get("Authorization") != "Bearer " + servidor.token_ok:
                    self.send_response(401)
                    self.end_headers()
                    return
                if self.path == "/api/sync/manifest":
                    cuerpo = json.dumps({"skills": [
                        {"id": i, "name": s["name"], "version": s["version"], "files": 1, "bytes": 1, "updatedAt": "x"}
                        for i, s in servidor.skills.items()]}).encode()
                elif self.path.startswith("/api/sync/skills/") and self.path.split("/")[-1] in servidor.skills:
                    i = self.path.split("/")[-1]
                    servidor.descargas.append(i)
                    cuerpo = servidor.skills[i]["zip"]
                else:
                    self.send_response(404)
                    self.end_headers()
                    return
                self.send_response(200)
                self.send_header("Content-Length", str(len(cuerpo)))
                self.end_headers()
                self.wfile.write(cuerpo)

        self.httpd = ThreadingHTTPServer(("127.0.0.1", 0), H)
        self.url = "http://127.0.0.1:%d" % self.httpd.server_address[1]
        threading.Thread(target=lambda: self.httpd.serve_forever(poll_interval=0.01), daemon=True).start()

    def poner(self, id, nombre, version, archivos=None, zip_bytes=None):
        archivos = archivos if archivos is not None else {"SKILL.md": skill_md(nombre)}
        self.skills[id] = {"name": nombre, "version": version, "zip": zip_bytes or hacer_zip(nombre, archivos)}

    def cerrar(self):
        self.httpd.shutdown()
        self.httpd.server_close()


def leer(ruta, modo="r"):
    with open(ruta, modo) as f:
        return f.read()


def escribir(ruta, texto):
    with open(ruta, "w") as f:
        f.write(texto)


class Base(unittest.TestCase):
    def setUp(self):
        self.srv = Servidor()
        self.tmp = tempfile.TemporaryDirectory()
        self.dir = os.path.join(self.tmp.name, "skills")
        self.env = mock.patch.dict(os.environ, {"HOMELAB_URL": self.srv.url, "HOMELAB_TOKEN": TOKEN})
        self.env.start()

    def tearDown(self):
        self.env.stop()
        self.srv.cerrar()
        self.tmp.cleanup()

    def correr(self, *args):
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            codigo = sync.main(["--dir", self.dir, *args])
        return codigo, out.getvalue(), err.getvalue()

    def ruta(self, *p):
        return os.path.join(self.dir, *p)


class Sincronizacion(Base):
    def test_primera_vez_baja_todo_con_binarios_y_scripts(self):
        self.srv.poner("1", "mi-skill", 1, {"SKILL.md": skill_md("mi-skill"), "assets/logo.bin": "\x00\x01\x02", "scripts/run.sh": "#!/bin/sh\necho hola"})
        self.srv.skills["1"]["zip"] = hacer_zip("mi-skill", {"SKILL.md": skill_md("mi-skill"), "assets/logo.bin": "\x00\x01\x02", "scripts/run.sh": "#!/bin/sh\necho hola"})
        codigo, out, _ = self.correr()
        self.assertEqual(codigo, 0, out)
        self.assertIn("1 nuevas", out)
        self.assertTrue(os.path.isfile(self.ruta("mi-skill", "SKILL.md")))
        self.assertEqual(leer(self.ruta("mi-skill", "assets", "logo.bin"), "rb"), b"\x00\x01\x02")
        self.assertTrue(os.stat(self.ruta("mi-skill", "scripts", "run.sh")).st_mode & stat.S_IXUSR, "un script con shebang queda ejecutable")
        self.assertFalse(os.stat(self.ruta("mi-skill", "SKILL.md")).st_mode & stat.S_IXUSR)
        self.assertEqual(json.loads(leer(self.ruta(sync.ESTADO)))["skills"]["mi-skill"], {"id": "1", "version": 1})

    def test_segunda_vez_no_vuelve_a_bajar_nada(self):
        self.srv.poner("1", "mi-skill", 1)
        self.correr()
        self.assertEqual(len(self.srv.descargas), 1)
        codigo, out, _ = self.correr()
        self.assertEqual(codigo, 0)
        self.assertIn("al día", out)
        self.assertEqual(len(self.srv.descargas), 1, "una skill al día no se vuelve a descargar")

    def test_actualizar_reemplaza_y_quita_lo_que_ya_no_esta(self):
        self.srv.poner("1", "mi-skill", 1, {"SKILL.md": skill_md("mi-skill"), "viejo.md": "adiós"})
        self.correr()
        self.assertTrue(os.path.exists(self.ruta("mi-skill", "viejo.md")))
        self.srv.poner("1", "mi-skill", 2, {"SKILL.md": skill_md("mi-skill") + "NUEVO", "nuevo.md": "hola"})
        codigo, out, _ = self.correr()
        self.assertEqual(codigo, 0, out)
        self.assertIn("1 actualizadas", out)
        self.assertIn("NUEVO", leer(self.ruta("mi-skill", "SKILL.md")))
        self.assertFalse(os.path.exists(self.ruta("mi-skill", "viejo.md")), "lo que ya no está en la nueva versión desaparece")
        self.assertTrue(os.path.exists(self.ruta("mi-skill", "nuevo.md")))
        self.assertEqual([n for n in os.listdir(self.dir) if n.startswith(".") and n != sync.ESTADO], [], "sin restos temporales")

    def test_no_pisa_una_skill_local_ajena(self):
        os.makedirs(self.ruta("mi-skill"))
        escribir(self.ruta("mi-skill", "SKILL.md"), "LOCAL, MÍA")
        self.srv.poner("1", "mi-skill", 1)
        codigo, out, _ = self.correr()
        self.assertEqual(codigo, 0)
        self.assertIn("ya tienes una skill local", out)
        self.assertEqual(leer(self.ruta("mi-skill", "SKILL.md")), "LOCAL, MÍA", "no se toca lo que no es nuestro")
        self.assertEqual(self.srv.descargas, [])
        # Con --force sí.
        self.correr("--force")
        self.assertIn("cuerpo", leer(self.ruta("mi-skill", "SKILL.md")))

    def test_dry_run_no_escribe_nada(self):
        self.srv.poner("1", "mi-skill", 1)
        codigo, out, _ = self.correr("--dry-run")
        self.assertEqual(codigo, 0)
        self.assertIn("se bajaría", out)
        self.assertFalse(os.path.exists(self.dir), "ni siquiera crea la carpeta")
        self.assertEqual(self.srv.descargas, [])

    def test_check_cuenta_pendientes_sin_escribir(self):
        self.srv.poner("1", "a", 1)
        self.srv.poner("2", "b", 1)
        codigo, out, _ = self.correr("--check")
        r = json.loads(out)
        self.assertEqual((codigo, r["total"], sorted(r["pendientes"])), (0, 2, ["a", "b"]))
        self.assertFalse(os.path.exists(self.dir))
        self.correr()
        self.srv.poner("1", "a", 2)
        self.assertEqual(json.loads(self.correr("--check")[1])["pendientes"], ["a"])

    def test_prune_solo_quita_lo_que_bajo_este_script(self):
        self.srv.poner("1", "mia-remota", 1)
        os.makedirs(self.ruta("local-propia"))
        escribir(self.ruta("local-propia", "SKILL.md"), "x")
        self.correr()
        del self.srv.skills["1"]
        _, out, _ = self.correr()
        self.assertIn("--prune", out)
        self.assertTrue(os.path.isdir(self.ruta("mia-remota")), "sin --prune no se borra nada")
        self.correr("--prune")
        self.assertFalse(os.path.exists(self.ruta("mia-remota")))
        self.assertTrue(os.path.isdir(self.ruta("local-propia")), "una skill local que no bajó el script no se toca jamás")
        self.assertNotIn("mia-remota", json.loads(leer(self.ruta(sync.ESTADO)))["skills"])

    def test_quiet_calla_si_todo_esta_al_dia(self):
        self.srv.poner("1", "a", 1)
        self.correr()
        self.assertEqual(self.correr("--quiet")[1], "")


class Hostiles(Base):
    def assert_rechazada(self, nombre, zip_bytes, texto):
        self.srv.poner("9", nombre, 1, zip_bytes=zip_bytes)
        codigo, out, _ = self.correr()
        self.assertEqual(codigo, 1, out)
        self.assertIn(texto, out)
        self.assertFalse(os.path.exists(self.ruta(nombre)), "una skill rechazada no deja nada")
        fuera = [n for n in os.listdir(self.tmp.name) if n != "skills"]
        self.assertEqual(fuera, [], "y nada se escribió fuera de la carpeta")

    def test_zip_slip(self):
        self.assert_rechazada("ok", hacer_zip("ok", {"SKILL.md": skill_md("ok"), "../../escape.txt": "x"}), "no permitida")

    def test_ruta_absoluta_o_fuera_de_su_carpeta(self):
        self.assert_rechazada("ok", hacer_zip("ok", {"SKILL.md": skill_md("ok")}, extra=[("/etc/cron.d/x", "x")]), "fuera de su carpeta")

    def test_ruta_de_otra_skill(self):
        self.assert_rechazada("ok", hacer_zip("ok", {"SKILL.md": skill_md("ok")}, extra=[("otra-skill/SKILL.md", "x")]), "fuera de su carpeta")

    def test_enlace_simbolico(self):
        info = zipfile.ZipInfo("ok/enlace")
        info.external_attr = (stat.S_IFLNK | 0o777) << 16
        self.assert_rechazada("ok", hacer_zip("ok", {"SKILL.md": skill_md("ok")}, extra=[(info, "/etc/passwd")]), "enlace simbólico")

    def test_sin_skill_md(self):
        self.assert_rechazada("ok", hacer_zip("ok", {"otro.md": "x"}), "SKILL.md")

    def test_no_es_un_zip(self):
        self.assert_rechazada("ok", b"esto no es un zip", "no es un zip")

    def test_archivo_enorme(self):
        with mock.patch.object(sync, "MAX_ARCHIVO", 1000):
            self.assert_rechazada("ok", hacer_zip("ok", {"SKILL.md": skill_md("ok"), "grande.bin": "x" * 5000}), "pesa demasiado")

    def test_total_enorme(self):
        with mock.patch.object(sync, "MAX_TOTAL", 3000):
            self.assert_rechazada("ok", hacer_zip("ok", {"SKILL.md": skill_md("ok"), "a": "x" * 2000, "b": "x" * 2000}), "pesa demasiado")

    def test_nombre_malicioso_en_el_manifiesto(self):
        for malo in ["../escape", "a/b", ".oculta", "MAYUS", "con espacio", "", "x" * 80]:
            self.srv.skills.clear()
            self.srv.poner("9", malo, 1, zip_bytes=hacer_zip("ok", {"SKILL.md": skill_md("ok")}))
            codigo, out, _ = self.correr()
            self.assertEqual(codigo, 1, (malo, out))
            self.assertIn("no es seguro", out)
        self.assertEqual([n for n in os.listdir(self.tmp.name)], [], "ni siquiera se crea la carpeta de destino")
        self.assertEqual(self.srv.descargas, [], "ni se descarga lo que tiene un nombre inválido")

    def test_una_actualizacion_rota_no_estropea_la_skill_que_ya_tenias(self):
        self.srv.poner("1", "ok", 1)
        self.correr()
        buena = leer(self.ruta("ok", "SKILL.md"))
        self.srv.poner("1", "ok", 2, zip_bytes=hacer_zip("ok", {"SKILL.md": skill_md("ok"), "../../escape": "x"}))
        codigo, out, _ = self.correr()
        self.assertEqual(codigo, 1)
        self.assertEqual(leer(self.ruta("ok", "SKILL.md")), buena, "la versión anterior sigue intacta")
        self.assertEqual(json.loads(leer(self.ruta(sync.ESTADO)))["skills"]["ok"]["version"], 1)

    def test_un_error_en_una_no_impide_las_demas(self):
        self.srv.poner("1", "buena", 1)
        self.srv.poner("2", "rota", 1, zip_bytes=b"basura")
        self.srv.poner("3", "tambien-buena", 1)
        codigo, out, _ = self.correr()
        self.assertEqual(codigo, 1)
        self.assertTrue(os.path.isdir(self.ruta("buena")) and os.path.isdir(self.ruta("tambien-buena")))
        self.assertIn("2 nuevas, 0 actualizadas, 1 con error", out)


class Conexion(Base):
    def test_sin_token(self):
        with mock.patch.dict(os.environ, {"HOMELAB_TOKEN": ""}):
            codigo, _, err = self.correr()
        self.assertEqual(codigo, 2)
        self.assertIn("HOMELAB_TOKEN", err)

    def test_token_rechazado(self):
        self.srv.token_ok = "otro"
        codigo, _, err = self.correr()
        self.assertEqual(codigo, 2)
        self.assertIn("revocado", err)
        self.assertNotIn(TOKEN, err, "el token no debe salir en los mensajes")

    def test_servidor_inalcanzable(self):
        with mock.patch.dict(os.environ, {"HOMELAB_URL": "http://127.0.0.1:1"}):
            codigo, _, err = self.correr()
        self.assertEqual(codigo, 2)
        self.assertIn("No se pudo conectar", err)

    def test_url_mal_formada(self):
        with mock.patch.dict(os.environ, {"HOMELAB_URL": "ftp://x"}):
            self.assertEqual(self.correr()[0], 2)

    def test_sin_skills(self):
        codigo, out, _ = self.correr()
        self.assertEqual(codigo, 0)
        self.assertIn("todavía", out)


if __name__ == "__main__":
    unittest.main()
