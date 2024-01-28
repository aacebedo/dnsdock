FROM scratch

COPY dnsdock /
ENTRYPOINT ["dnsdock"]
