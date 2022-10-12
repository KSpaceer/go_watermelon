EMAILSERVICEPATH = ./cmd/email_service/
MAINSERVICEPATH = ./cmd/user_handling_service/
MAINSERVICEPROXYPATH = ./cmd/user_handling_service/proxy/
EMAILSERVICEEXEC = ./cmd/email_service/email_service
MAINSERVICEEXEC = ./cmd/user_handling_service/user_handling_service
MAINSERVICEPROXYEXEC = ./cmd/user_handling_service/proxy/user_handling_service_proxy

build: build_email_service build_main_service build_main_service_proxy

build_email_service:
	go build -o $(EMAILSERVICEEXEC) $(EMAILSERVICEPATH) 

build_main_service:
	go build -o $(MAINSERVICEEXEC) $(MAINSERVICEPATH) 

build_main_service_proxy:
	go build -o $(MAINSERVICEPROXYEXEC) $(MAINSERVICEPROXYPATH) 

test:
	go test -cpu 1,4 -race ./...

fmt:
	go fmt ./...

clean:
	rm -rf $(EMAILSERVICEEXEC) $(MAINSERVICEEXEC) $(MAINSERVICEPROXYEXEC)

rebuild: clean build
