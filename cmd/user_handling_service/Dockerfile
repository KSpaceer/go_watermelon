FROM scratch

EXPOSE 9090

COPY ./pgsinfo.txt /
COPY ./user_handling_service /

ENTRYPOINT ["/user_handling_service"]
