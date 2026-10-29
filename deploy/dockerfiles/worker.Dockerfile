FROM python:3.11-slim
WORKDIR /app
COPY internal/worker/requirements.txt .
RUN pip install --no-cache-dir -i https://pypi.tuna.tsinghua.edu.cn/simple -r requirements.txt
COPY internal/worker .
EXPOSE 18001
CMD ["uvicorn", "api.main:app", "--host", "0.0.0.0", "--port", "18001"]
