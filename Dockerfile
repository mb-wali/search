FROM discoenv/golang-base:master

#ENV CONF_TEMPLATE=/go/src/github.com/cyverse-de/search/jobservices.yml.tmpl
#ENV CONF_FILENAME=jobservices.yml
#ENV PROGRAM=search

ARG git_commit=unknown
ARG version="2.9.0"
ARG descriptive_version=unknown

LABEL org.cyverse.git-ref="$git_commit"
LABEL org.cyverse.version="$version"
LABEL org.cyverse.descriptive-version="$descriptive_version"

COPY . /go/src/github.com/cyverse-de/search
RUN go install -v -ldflags="-X main.appver=$version -X main.gitref=$git_commit" github.com/cyverse-de/search

# Remove when there's an actual config file and such set up
ENTRYPOINT ["/go/bin/search"]

EXPOSE 60000
LABEL org.label-schema.vcs-ref="$git_commit"
LABEL org.label-schema.vcs-url="https://github.com/cyverse-de/search"
LABEL org.label-schema.version="$descriptive_version"
