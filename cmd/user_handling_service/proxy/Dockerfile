FROM scratch


EXPOSE 8081
ADD ./cert /cert

COPY ./user_handling_service_proxy /

ENTRYPOINT ["/user_handling_service_proxy", "-tls"]
