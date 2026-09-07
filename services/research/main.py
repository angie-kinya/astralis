"""Astralis research ingestion -- the agent's self-research loop.

Run it on a schedule and the knowledge base grows without anyone touching
model weights. This is what "self-improving" means in Phase 1.

    python main.py --query "occupational burnout banking employees" --limit 25
    python main.py --all          # every standing query below
    python main.py --all --dry-run   # fetch + chunk, no embedding spend
"""
import argparse
import logging
import sys

from dotenv import load_dotenv

from db import connect, insert_chunks, knowledge_base_size, upsert_doc
from pipeline import chunk_text, embed_batch
from sources import fetch_openalex

load_dotenv()
logging.basicConfig(level=logging.INFO, format="%(levelname)s %(message)s")
log = logging.getLogger("ingest")

# The agent's curriculum. Widen this list as the project's interests widen --
# it is the single highest-leverage file in the whole research service.
STANDING_QUERIES = [
    "occupational burnout banking employees",
    "Maslach Burnout Inventory validation",
    "workplace stress financial services",
    "job demands resources model burnout",
    "burnout intervention effectiveness randomised controlled",
    "employee wellbeing Kenya Africa workplace",
    "recovery from work detachment sleep",
    "machine learning burnout prediction",
]


def ingest(query: str, limit: int, dry_run: bool = False) -> dict:
    stats = {"fetched": 0, "new_docs": 0, "chunks": 0}

    papers = fetch_openalex(query, limit)
    stats["fetched"] = len(papers)
    log.info("query=%r fetched=%d", query, len(papers))

    with connect() as conn:
        for paper in papers:
            with conn.cursor() as cur:
                doc_id, is_new = upsert_doc(cur, paper)
                if not is_new:
                    continue  # already embedded on an earlier run

                chunks = chunk_text(f"{paper.title}. {paper.abstract}")
                if not chunks:
                    continue

                if dry_run:
                    log.info("[dry-run] %d chunks from %.60s", len(chunks), paper.title)
                    conn.rollback()
                    continue

                vectors = embed_batch(chunks)
                stats["chunks"] += insert_chunks(cur, doc_id, chunks, vectors)
                stats["new_docs"] += 1

            # Commit per paper: a crash halfway through keeps everything before it.
            conn.commit()
            log.info("ingested %.70s", paper.title)
    return stats


def main() -> int:
    ap = argparse.ArgumentParser(description="Astralis research ingestion")
    ap.add_argument("--query")
    ap.add_argument("--limit", type=int, default=25)
    ap.add_argument("--all", action="store_true", help="run every standing query")
    ap.add_argument("--dry-run", action="store_true", help="skip the embedding API")
    args = ap.parse_args()

    queries = STANDING_QUERIES if args.all else [args.query]
    if not queries or queries == [None]:
        ap.error("provide --query or --all")

    total = {"fetched": 0, "new_docs": 0, "chunks": 0}
    for q in queries:
        try:
            for k, v in ingest(q, args.limit, args.dry_run).items():
                total[k] += v
        except Exception:
            # One bad query -- a 429, a schema change upstream -- must not
            # abort the other seven.
            log.exception("query failed: %s", q)

    with connect() as conn, conn.cursor() as cur:
        docs, chunks = knowledge_base_size(cur)
    log.info("run complete %s | knowledge base now %d docs / %d chunks",
             total, docs, chunks)
    return 0


if __name__ == "__main__":
    sys.exit(main())
