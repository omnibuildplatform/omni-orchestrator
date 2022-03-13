FROM golang:alpine3.13 as builder

LABEL maintainer="tommylike<tommylikehu@gmail.com>"
WORKDIR /app
COPY . /app
RUN go mod download
RUN CGO_ENABLED=0 go build -o omni-repository

FROM alpine/git:v2.30.2
ARG user=app
ARG group=app
ARG home=/app
RUN addgroup -S ${group} && adduser -S ${user} -G ${group} -h ${home}

USER ${user}
WORKDIR ${home}
COPY --chown=${user} --from=builder /app/omni-repository .
COPY --chown=${user} ./config/app.toml ./config/
# to fix the directory permission issue
RUN mkdir -p ${home}/logs $$ -p ${home}/data
VOLUME ["${home}/logs","${home}/data"]

ENV PATH="${home}:${PATH}"
ENV APP_ENV="prod"
EXPOSE 8080
ENTRYPOINT ["/app/omni-repository"]
