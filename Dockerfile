ARG ARCH
FROM ${ARCH}alpine:3.13

ARG K0S_VERSION

RUN apk add --no-cache --no-scripts ca-certificates
RUN update-ca-certificates
RUN apk add bash coreutils findutils iptables curl tini

RUN curl -sSLf https://get.k0s.sh | K0S_VERSION=${K0S_VERSION} sh

ENV KUBECONFIG=/var/lib/k0s/pki/admin.conf

ADD docker-entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/sbin/tini", "--", "/bin/sh", "/entrypoint.sh" ]


CMD ["k0s", "controller", "--enable-worker"]
