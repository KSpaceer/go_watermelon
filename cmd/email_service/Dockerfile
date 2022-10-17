FROM scratch

EXPOSE 587

COPY ./emailinfo.csv /
COPY ./email_service /

VOLUME /img

ENTRYPOINT ["/email_service"]
