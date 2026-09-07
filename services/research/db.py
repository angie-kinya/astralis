"""Database access for the ingestion job.

Every write is idempotent. This job runs on a schedule and will re-encounter
the same papers on every pass; ON CONFLICT turns a re-run from a
duplicate-row disaster into a no-op.
"""
import os

import psycopg


def connect():
    return psycopg.connect(os.environ["DATABASE_URL"])


def upsert_doc(cur, paper) -> tuple[str, bool]:
    """Insert a paper if we have never seen it.

    Returns (doc_id, is_new). is_new=False means the abstract is already
    chunked and embedded, so the caller can skip the embedding API call
    entirely -- by far the biggest cost saving in this pipeline.
    """
    cur.execute(
        """
        INSERT INTO research_docs (source, external_id, title, authors, year, url, abstract)
        VALUES (%s,%s,%s,%s,%s,%s,%s)
        ON CONFLICT (source, external_id) DO NOTHING
        RETURNING id
        """,
        (paper.source, paper.external_id, paper.title,
         paper.authors, paper.year, paper.url, paper.abstract),
    )
    row = cur.fetchone()
    if row:
        return row[0], True

    # DO NOTHING means no row came back, so look up the id we collided with.
    cur.execute(
        "SELECT id FROM research_docs WHERE source=%s AND external_id=%s",
        (paper.source, paper.external_id),
    )
    return cur.fetchone()[0], False


def insert_chunks(cur, doc_id: str, chunks: list[str], vectors: list[list[float]]) -> int:
    """Write chunk text + embedding. pgvector accepts the literal '[1,2,3]'."""
    written = 0
    for i, (content, vec) in enumerate(zip(chunks, vectors)):
        cur.execute(
            """
            INSERT INTO research_chunks (doc_id, chunk_index, content, embedding)
            VALUES (%s,%s,%s,%s)
            ON CONFLICT (doc_id, chunk_index) DO NOTHING
            """,
            (doc_id, i, content, "[" + ",".join(f"{v:.6f}" for v in vec) + "]"),
        )
        written += cur.rowcount
    return written


def knowledge_base_size(cur) -> tuple[int, int]:
    """Used by the CLI summary line so a run visibly reports growth."""
    cur.execute("SELECT count(*) FROM research_docs")
    docs = cur.fetchone()[0]
    cur.execute("SELECT count(*) FROM research_chunks")
    return docs, cur.fetchone()[0]
