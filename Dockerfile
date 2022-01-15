ARG ARCH
FROM ${ARCH}alpine:3.15

ARG K0S_VERSION

RUN apk add --no-cache bash coreutils findutils iptables curl tini

RUN curl -sSLf https://get.k0s.sh | K0S_VERSION=${K0S_VERSION} sh

ENV KUBECONFIG=/var/lib/k0s/pki/admin.conf

ADD docker-entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/sbin/tini", "--", "/bin/sh", "/entrypoint.sh" ]


CMD ["k0s", "controller", "--enable-worker"]
