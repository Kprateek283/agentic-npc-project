# --- build stage ---
FROM golang:1.24-alpine AS build
WORKDIR /src

# Cache module downloads. GOWORK=off builds backend-go as a standalone module (the repo's
# go.work / vendor live one level up and are not copied into this build context).
ENV GOWORK=off
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

# --- runtime stage ---
FROM alpine:3.20
WORKDIR /app
ENV GAMEDATA_DIR=/app/gamedata
# gamedata is read from disk at startup (seeder + quest/emotion managers) via GAMEDATA_DIR.
COPY --from=build /out/server /app/server
COPY --from=gamedata . /app/gamedata
EXPOSE 8080
ENTRYPOINT ["/app/server"]
