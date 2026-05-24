FROM oven/bun:1.3.1-alpine

WORKDIR /app

COPY frontend/package.json frontend/bun.lockb ./
RUN bun install --frozen-lockfile

COPY frontend ./
COPY docs /docs

EXPOSE 15175

CMD ["bun", "run", "dev", "--host", "0.0.0.0", "--port", "15175"]
