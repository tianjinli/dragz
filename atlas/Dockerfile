FROM alpine:3.19 AS atlas
RUN wget -O /atlas https://release.ariga.io/atlas/atlas-linux-amd64-latest

FROM alpine:3.19
COPY --from=atlas /atlas /usr/local/bin/atlas
RUN chmod +x /usr/local/bin/atlas

WORKDIR /app
COPY migrations ./migrations

WORKDIR /app
COPY migrations ./migrations
COPY entrypoint.sh ./entrypoint.sh
RUN chmod +x /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]
