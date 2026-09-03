FROM python:3.12-slim

WORKDIR /app
COPY ai/requirements.txt ./requirements.txt
RUN pip install --no-cache-dir -r requirements.txt

COPY ai/app ./app

ENV AI_PORT=8090
EXPOSE 8090

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8090"]
