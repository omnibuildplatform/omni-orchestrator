FROM golang:alpine3.13 as builder

LABEL maintainer="tommylike<tommylikehu@gmail.com>"
ARG GIT_TAG
ARG GIT_COMMIT
ARG RELEASED_AT
WORKDIR /app
COPY . /app
RUN go mod download
RUN CGO_ENABLED=0 go build -ldflags "-X main.Tag=$GIT_TAG -X main.CommitID=$GIT_COMMIT -X main.ReleaseAt=$RELEASED_AT" -o omni-orchestrator

FROM alpine/git:v2.30.2
ARG user=app
ARG group=app
ARG home=/app
RUN addgroup -S ${group} && adduser -S ${user} -G ${group} -h ${home}

USER ${user}
WORKDIR ${home}
COPY --chown=${user} --from=builder /app/omni-orchestrator .
COPY --chown=${user} ./config/app.toml ./config/
# to fix the directory permission issue
RUN mkdir -p ${home}/logs
VOLUME ["${home}/logs"]

ENV PATH="${home}:${PATH}"
ENV APP_ENV="prod"
EXPOSE 8080
ENTRYPOINT ["/app/omni-orchestrator"]
