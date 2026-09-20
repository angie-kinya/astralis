import os
from dataclasses import dataclass, field

import httpx
from tenacity import retry, stop_after_attempt, wait_exponential

OPENALEX = "https://api.openalex.org/works"


@dataclass
class Paper:
    source: str
    external_id: str
    title: str
    abstract: str
    url: str
    year: int | None = None
    authors: list[str] = field(default_factory=list)


def _deinvert(index: dict | None) -> str:
    if not index:
        return ""
    slots: dict[int, str] = {}
    for word, positions in index.items():
        for p in positions:
            slots[p] = word
    return " ".join(slots[i] for i in sorted(slots))


@retry(stop=stop_after_attempt(3), wait=wait_exponential(min=1, max=10))
def fetch_openalex(query: str, limit: int = 25) -> list[Paper]:
    params = {
        "search": query,
        "per-page": min(limit, 50),
        "filter": "has_abstract:true,type:article",
        "sort": "cited_by_count:desc",  # a crude but effective quality proxy
        "mailto": os.environ.get("OPENALEX_MAILTO", "astralis@example.com"),
    }
    with httpx.Client(timeout=30) as client:
        r = client.get(OPENALEX, params=params)
        r.raise_for_status()
        results = r.json().get("results", [])

    papers: list[Paper] = []
    for w in results:
        abstract = _deinvert(w.get("abstract_inverted_index"))
        if len(abstract) < 200:
            continue  # too thin to chunk into anything meaningful
        papers.append(
            Paper(
                source="openalex",
                external_id=w["id"].rsplit("/", 1)[-1],
                title=w.get("title") or "Untitled",
                abstract=abstract,
                url=w.get("doi") or w["id"],
                year=w.get("publication_year"),
                authors=[
                    a["author"]["display_name"]
                    for a in w.get("authorships", [])[:8]
                    if a.get("author")
                ],
            )
        )
    return papers
