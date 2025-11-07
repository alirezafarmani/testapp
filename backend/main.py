from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles
from fastapi.responses import FileResponse
import logging, sys, os
from pythonjsonlogger import jsonlogger
from pydantic import BaseModel
import uuid

# JSON logging
logger = logging.getLogger()
handler = logging.StreamHandler(sys.stdout)
formatter = jsonlogger.JsonFormatter('%(asctime)s %(levelname)s %(message)s')
handler.setFormatter(formatter)
logger.addHandler(handler)
logger.setLevel(logging.INFO)

app = FastAPI(title="Test App (in-memory)")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # برای توسعه محلی آسان است؛ در prod محدودش کن
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# in-memory store (ephemeral)
_items = {}

class Item(BaseModel):
    name: str
    value: int

@app.get("/health")
async def health():
    logger.info({"event":"healthcheck","status":"ok"})
    return {"status":"ok"}

@app.post("/items", status_code=201)
async def create_item(item: Item):
    _id = str(uuid.uuid4())
    doc = {"_id": _id, "name": item.name, "value": item.value}
    _items[_id] = doc
    logger.info({"event":"create_item","item":doc})
    return doc

@app.get("/items")
async def list_items():
    docs = list(_items.values())
    logger.info({"event":"list_items","count": len(docs)})
    return docs

@app.get("/items/{item_id}")
async def get_item(item_id: str):
    doc = _items.get(item_id)
    if not doc:
        logger.info({"event":"get_item","item_id": item_id, "found": False})
        raise HTTPException(status_code=404, detail="not found")
    logger.info({"event":"get_item","item_id": item_id, "found": True})
    return doc

# static files serving (if frontend is present inside image at /app/frontend)
#frontend_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), "frontend"))

#frontend_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "frontend"))

frontend_dir = os.getenv("FRONTEND_DIR", os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "frontend")))

if not os.path.isabs(frontend_dir):
    frontend_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "frontend"))

if os.path.isdir(frontend_dir):
    app.mount("/static", StaticFiles(directory=frontend_dir), name="static")
    index_path = os.path.join(frontend_dir, "index.html")
    if os.path.isfile(index_path):
        @app.get("/", include_in_schema=False)
        async def root_index():
            return FileResponse(index_path)
    logger.info({"event":"static_mounted","path": frontend_dir})
else:
    logger.info({"event":"static_mount_skipped","checked": frontend_dir})
