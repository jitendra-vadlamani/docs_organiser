# Stage 1: Frontend Builder
FROM node:18-alpine AS frontend-builder
WORKDIR /app/ui
COPY ui/package*.json ./
RUN npm install
COPY ui/ .
# Fix for missing tsc in path if needed, though npm run build should handle it
RUN npm run build

# Stage 2: Backend Builder
FROM golang:alpine AS backend-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy built frontend assets so they can be served/embedded if needed
# Although the Go code currently expects them at ui/dist
COPY --from=frontend-builder /app/ui/dist ./ui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o docs_organiser main.go

# Stage 3: Final Image
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=backend-builder /app/docs_organiser .
COPY --from=backend-builder /app/ui/dist ./ui/dist

# Expose ports
EXPOSE 8080 8081

# Command to run the binary
ENTRYPOINT ["./docs_organiser"]
