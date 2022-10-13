FROM scratch

EXPOSE 587

COPY ../../img ./
COPY ./emailinfo.csv ./
COPY email_service ./

ENTRYPOINT ./email_service
