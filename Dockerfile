FROM golang:1.16-alpine

COPY . /go/src/github.com/cyverse-de/search
WORKDIR /go/src/github.com/cyverse-de/search
ENV CGO_ENABLED=0
RUN go install -v github.com/cyverse-de/search

ENTRYPOINT ["search"]
CMD ["--help"]
EXPOSE 60000

ARG git_commit=unknown
ARG version="2.9.0"
ARG descriptive_version=unknown

LABEL org.cyverse.git-ref="$git_commit"
LABEL org.cyverse.version="$version"
LABEL org.cyverse.descriptive-version="$descriptive_version"
LABEL org.label-schema.vcs-ref="$git_commit"
LABEL org.label-schema.vcs-url="https://github.com/cyverse-de/search"
LABEL org.label-schema.version="$descriptive_version"
