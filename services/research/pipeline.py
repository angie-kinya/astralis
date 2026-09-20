import os

import httpx
from tenacity import retry, stop_after_attempt, wait_exponential

CHUNK_SIZE = 1200
CHUNK_OVERLAP = 150


def chunk_text(
    text: str, size: int = CHUNK_SIZE, overlap: int = CHUNK_OVERLAP
) -> list[str]:
    text = " ".join(text.split())  # normalise the whitespace PDFs leave behind
    if len(text) <= size:
        return [text]

    chunks, start = [], 0
    while start < len(text):
        end = start + size
        if end < len(text):
            # Prefer a sentence boundary in the last quarter of the window.
            # Cutting mid-sentence is what makes retrieved context read like
            # nonsense in the prompt.
            boundary = text.rfind(". ", start + int(size * 0.75), end)
            if boundary != -1:
                end = boundary + 1
        chunks.append(text[start:end].strip())
        # max(..., start + 1) guarantees forward progress. Without it, a
        # pathological overlap value gives you an infinite loop.
        start = max(end - overlap, start + 1)
    return [c for c in chunks if len(c) > 80]


@retry(stop=stop_after_attempt(3), wait=wait_exponential(min=2, max=20))
def embed_batch(texts: list[str]) -> list[list[float]]:
    """One request for many texts, not one request each.

    Batching is the difference between an ingestion run that takes a minute and
    one that takes twenty, at the same cost per token.
    """
    model = os.environ.get("EMBED_MODEL", "text-embedding-3-small")
    key = os.environ["OPENAI_API_KEY"]
    with httpx.Client(timeout=60) as client:
        r = client.post(
            "https://api.openai.com/v1/embeddings",
            headers={"Authorization": f"Bearer {key}"},
            json={"model": model, "input": texts},
        )
        r.raise_for_status()
        data = r.json()["data"]
    # Sort by index. Same trap as the Go embedder: the API does not promise
    # response order, and silently mismatched vectors produce no error at all.
    return [d["embedding"] for d in sorted(data, key=lambda d: d["index"])]
