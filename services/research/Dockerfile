FROM python:3.12-slim
WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .
RUN adduser --disabled-password --uid 10001 astralis && chown -R astralis /app
USER astralis

# Render's cron runtime invokes this once per schedule tick and exits.
CMD ["python", "main.py", "--all"]
