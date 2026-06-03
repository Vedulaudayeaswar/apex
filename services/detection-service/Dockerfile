FROM python:3.11-slim

WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends \
    libgl1 libglib2.0-0 && rm -rf /var/lib/apt/lists/*

# CPU-only PyTorch (~250MB). Default pip torch pulls ~2.5GB of NVIDIA CUDA libs.
COPY requirements.txt .
RUN pip install --no-cache-dir torch torchvision --index-url https://download.pytorch.org/whl/cpu \
    && pip install --no-cache-dir -r requirements.txt

COPY app ./app
ENV PYTHONPATH=/app
ENV VIDEO_SOURCE=0

EXPOSE 8090
CMD ["python", "-m", "app.main"]
