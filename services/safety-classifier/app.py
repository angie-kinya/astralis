"""Crisis classifier served over HTTP.

Its own service because it has a different language, different dependencies,
and a different deployment cadence to the Go API. The Go side treats it as
optional: if this is down, the rules layer still runs and chat stays up.

It deliberately does NOT decide anything. It returns a probability; the Go
service owns the threshold. Policy in one place means you can retune safety
without redeploying a Python container.
"""
import pathlib

import joblib
from fastapi import FastAPI
from pydantic import BaseModel, Field

MODEL_PATH = pathlib.Path(__file__).parent / "models" / "crisis_clf.joblib"

app = FastAPI(title="Astralis Safety Classifier", docs_url=None, redoc_url=None)
_model = None


def get_model():
    """Lazy-load once per process. Loading in a module-level statement would
    make the container fail to start rather than fail a health check."""
    global _model
    if _model is None:
        if not MODEL_PATH.exists():
            raise RuntimeError("model not trained -- run: python train.py")
        _model = joblib.load(MODEL_PATH)
    return _model


class ScoreRequest(BaseModel):
    text: str = Field(min_length=1, max_length=4000)


@app.get("/healthz")
def healthz():
    return {"status": "ok", "model_trained": MODEL_PATH.exists()}


@app.post("/score")
def score(req: ScoreRequest):
    """Return P(crisis) in [0,1]. Never logs the text -- this endpoint sees
    the most sensitive sentences in the whole system."""
    proba = get_model().predict_proba([req.text])[0][1]
    return {"score": float(proba)}
