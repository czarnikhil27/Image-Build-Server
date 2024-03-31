# Start from the official golang base image
FROM golang:latest


# Set necessary environment variables needed for our image
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64


RUN apt-get update && \
    apt-get install -y curl && \
    apt-get install -y git && \
    apt-get install -y awscli && \
    apt-get install -y docker.io

WORKDIR /home/app

COPY . /home/app/

RUN go mod tidy
RUN go mod vendor

RUN chmod +x main.sh
RUN chmod +x main.go

ENTRYPOINT [ "/home/app/main.sh" ]
