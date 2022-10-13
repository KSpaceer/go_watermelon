FROM scratch

EXPOSE 587

COPY ./emailinfo.csv ./
COPY ./email_service ./

VOLUME ../../img ./img

ENTRYPOINT ["./email_service"]
