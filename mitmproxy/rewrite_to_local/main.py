# rewrite_to_local.py

import os
from mitmproxy import http, ctx
from dotenv import load_dotenv

# Load .env from the same directory (only in dev)
load_dotenv()

# Read from environment (fallbacks are optional)
STAGING_HOST = os.getenv("STAGING_HOST", "staging.api.example.com")
LOCAL_ADDRESS = os.getenv("LOCAL_ADDRESS", "127.0.0.1")
LOCAL_PORT = int(os.getenv("LOCAL_PORT",    "5000"))

MATCH_PREFIX = "/stg/v1/ubah-izin/"


def load(loader):
    ctx.log.info(
        f"[rewrite_to_local] proxying {STAGING_HOST} → {LOCAL_ADDRESS}:{LOCAL_PORT}"
    )


def request(flow: http.HTTPFlow) -> None:
    req = flow.request
    if req.pretty_host == STAGING_HOST and req.path.startswith(MATCH_PREFIX):
        # Trim the prefix from path
        trimmed_path = req.path[len(MATCH_PREFIX)-1:]  # keeps the starting `/`
        req.path = trimmed_path
        req.host = LOCAL_ADDRESS
        req.port = LOCAL_PORT
        req.scheme = "http"
        req.headers["Host"] = STAGING_HOST
        ctx.log.info(
            f"↪ {req.method} {req.url} → http://{LOCAL_ADDRESS}:{LOCAL_PORT}{req.path}"
        )
    else:
        ctx.log.info(
            f"[MITM] Ignored {req.method} {req.pretty_host}{req.path}")


def response(flow: http.HTTPFlow) -> None:
    # (optional) mutate responses here
    pass
