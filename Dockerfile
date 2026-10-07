# --- Frontend (SvelteKit SPA) ---
FROM node:22-alpine AS frontend
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# --- Backend (Go) ---
FROM golang:1.26-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/createclinic ./cmd/createclinic \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/createplatformadmin ./cmd/createplatformadmin \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/resetpassword ./cmd/resetpassword \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/encryptfields ./cmd/encryptfields \
 && mkdir -p /out/uploads

# --- Runtime: one small image serving the API and the built frontend ---
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /out/server /out/createclinic /out/createplatformadmin /out/resetpassword /out/encryptfields /app/
COPY --from=frontend /app/build /app/public
# Carpeta de adjuntos con dueño "nonroot": el volumen nombrado hereda este dueño al crearse.
COPY --from=backend --chown=65532:65532 /out/uploads /app/data/uploads
ARG BUILD_COMMIT=
ENV STATIC_DIR=/app/public ADDR=:8080 BUILD_COMMIT=$BUILD_COMMIT
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD ["/app/server", "healthcheck"]
ENTRYPOINT ["/app/server"]
