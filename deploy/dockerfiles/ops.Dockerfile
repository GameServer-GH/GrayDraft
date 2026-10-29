FROM python:3.11-slim
WORKDIR /app
COPY internal/ops/requirements.txt .
RUN pip install --no-cache-dir -i https://pypi.tuna.tsinghua.edu.cn/simple -r requirements.txt
COPY internal/ops .
EXPOSE 18003
CMD ["uvicorn", "api.main:app", "--host", "0.0.0.0", "--port", "18003"]
