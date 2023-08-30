
FROM golang:1.20

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./

# Copy the source code. Note the slash at the end, as explained in
# https://docs.docker.com/engine/reference/builder/#copy
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /dump-log

# Run
CMD ["/dump-log", "/tmp/mbg-log-file"]

