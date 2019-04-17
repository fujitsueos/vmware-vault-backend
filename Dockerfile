FROM golang:1.10 AS compile

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN mkdir -p /go/src/github.com/fujitsueos/vmware-vault-backend
WORKDIR /go/src/github.com/fujitsueos/vmware-vault-backend

COPY . ./

RUN go build -o vmware-backend

FROM vault:1.0.2

RUN apk update \
  && apk add --no-cache ca-certificates=20171114-r3 \
  && apk add --no-cache wget=1.20.3-r0

RUN wget -q -O /etc/apk/keys/sgerrand.rsa.pub https://alpine-pkgs.sgerrand.com/sgerrand.rsa.pub

RUN wget https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.29-r0/glibc-2.29-r0.apk

# hadolint ignore=DL3018
RUN apk add --no-cache glibc-2.29-r0.apk

SHELL ["/bin/sh", "-o", "pipefail", "-c"]

RUN mkdir -p /tmp/plugins

COPY --from=compile /go/src/github.com/fujitsueos/vmware-vault-backend/vmware-backend ./tmp/plugins/

RUN ls /tmp/plugins

ENV VAULT_DEV_ROOT_TOKEN_ID="root" VAULT_ADDR="http://127.0.0.1:8200"

COPY scripts/docker_dev.sh ./tmp/scripts/

CMD ["/tmp/scripts/docker_dev.sh"]
