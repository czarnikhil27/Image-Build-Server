
FROM golang:latest


ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

ARG GIT_REPOSITORY_URL

RUN git clone "$GIT_REPOSITORY_URL" /home/app

WORKDIR /home/app

COPY . /home/app/

RUN go mod tidy
RUN go mod vendor


EXPOSE 8080

ENTRYPOINT [ "/home/app/main.go" ]
