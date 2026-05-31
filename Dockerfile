# Depends on Available px-containers base image: harbor.dx.proxywerx.io/library/alpine-base. Image is Pinned as of 04-2026. 
FROM harbor.dx.proxywerx.io/library/alpine-base@sha256:dff90e4bce42c0399af3700e741e4ea1e3dcca2f0198de892b3b0e9b8694840c

USER root

RUN apk add --no-cache ca-certificates openssl tzdata

WORKDIR /inframon

RUN mkdir -p /app/inframon/logs

COPY --from=builder /inframon/inframon /app/inframon/inframon

ENV CONFIG_PATH=/config/config.yaml

RUN chown -R linuxuser:linuxuser /app/inframon && \
    chmod -R 755 /app/inframon

USER linuxuser

STOPSIGNAL SIGTERM

CMD ["/app/inframon/inframon", "--config=/config/config.yaml"]