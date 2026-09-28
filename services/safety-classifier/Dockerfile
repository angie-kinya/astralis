FROM docker.io/library/python:3.12-slim
WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .
# Train at build time so the image ships a model and the container starts
# ready. The training data is in the repo, so the build is reproducible.
RUN python train.py

RUN adduser --disabled-password --uid 10001 astralis && chown -R astralis /app
USER astralis
EXPOSE 8000
CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8000"]
