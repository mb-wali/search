FROM golang:1.16-alpine

COPY . /go/src/github.com/cyverse-de/search
WORKDIR /go/src/github.com/cyverse-de/search

# copy config file 
COPY search.yaml /etc/iplant/de/search.yaml

ENV CGO_ENABLED=0
RUN go install -v github.com/cyverse-de/search

ENTRYPOINT ["search", "--config", "/etc/iplant/de/search.yaml"]
# CMD ["--help"]
EXPOSE 60011

ARG git_commit=unknown
ARG version="2.9.0"
ARG descriptive_version=unknown

LABEL org.cyverse.git-ref="$git_commit"
LABEL org.cyverse.version="$version"
LABEL org.cyverse.descriptive-version="$descriptive_version"
LABEL org.label-schema.vcs-ref="$git_commit"
LABEL org.label-schema.vcs-url="https://github.com/cyverse-de/search"
LABEL org.label-schema.version="$descriptive_version"


# build
# docker build -t mbwali/search:latest .

# run
# docker run -it -p 60011:60011 mbwali/search:latest

# config
# /etc/iplant/de/search.yaml
