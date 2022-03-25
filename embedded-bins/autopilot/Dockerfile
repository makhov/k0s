ARG BUILDIMAGE=golang:1.17-alpine
FROM $BUILDIMAGE AS build

ARG VERSION
ARG BUILD_GO_TAGS
ARG BUILD_GO_CGO_ENABLED
ARG BUILD_SHIM_GO_CGO_ENABLED
ARG BUILD_GO_FLAGS
ARG BUILD_GO_LDFLAGS
ARG BUILD_GO_LDFLAGS_EXTRA
ENV GOPATH=/go

RUN apk upgrade -U -a && apk add \
	build-base git openssh make

COPY key /root/.ssh/id_rsa
RUN echo -e "Host github.com\n\tStrictHostKeyChecking no\n" >> /root/.ssh/config
RUN mkdir -p $GOPATH/src/github.com/k0sproject/autopilot
# RUN git clone -b v$VERSION --depth=1 git@github.com:k0sproject/autopilot.git $GOPATH/src/github.com/k0sproject/autopilot
RUN git clone -b 13-update --depth=1 git@github.com:k0sproject/autopilot.git $GOPATH/src/github.com/k0sproject/autopilot
WORKDIR /go/src/github.com/k0sproject/autopilot
RUN go build -o autopilot ./cmd/autopilot
RUN ls -la

FROM scratch
COPY --from=build /go/src/github.com/k0sproject/autopilot/autopilot /bin/
CMD ["/bin/autopilot"]
