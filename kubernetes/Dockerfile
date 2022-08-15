# SPDX-License-Identifier: MIT
FROM alpine:3.16.2

LABEL description="Kosmoo"

RUN apk add --no-cache ca-certificates

ADD kosmoo /bin/

ENTRYPOINT ["/bin/kosmoo"]
