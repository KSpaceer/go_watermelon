FROM scratch

EXPOSE 9090

ADD ./cert /cert

COPY ./pgsinfo.txt /
COPY ./user_handling_service /

ENTRYPOINT ["/user_handling_service", "-tls"]
