#!/usr/bin/env python3
"""
Static file server for the Blofeld NN web UI.
Adds COOP/COEP headers required for ONNX Runtime Web (SharedArrayBuffer / WASM threads).
"""

import http.server
import os
from pathlib import Path

PORT = 8080

class Handler(http.server.SimpleHTTPRequestHandler):
    def end_headers(self):
        self.send_header("Cross-Origin-Opener-Policy", "same-origin")
        self.send_header("Cross-Origin-Embedder-Policy", "credentialless")
        super().end_headers()

    def log_message(self, fmt, *args):
        print(f"  {self.address_string()} {fmt % args}")


os.chdir(Path(__file__).parent)
print(f"Blofeld NN UI → http://localhost:{PORT}")
http.server.HTTPServer(("", PORT), Handler).serve_forever()
