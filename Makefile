GOENVS = CGO_ENABLED=0 GOOS=linux GOARCH=386
GOFLAGS = -a -installsuffix cgo -ldflags '-s'

EMAILSERVICEPATH = ./cmd/email_service/
MAINSERVICEPATH = ./cmd/user_handling_service/
MAINSERVICEPROXYPATH = ./cmd/user_handling_service/proxy/
CLICKHOUSEPATH = ./dbdata/clickhouse

EMAILSERVICEEXEC = ./cmd/email_service/email_service
MAINSERVICEEXEC = ./cmd/user_handling_service/user_handling_service
MAINSERVICEPROXYEXEC = ./cmd/user_handling_service/proxy/user_handling_service_proxy

EMAILSERVICEIMAGE = watermelon-emailservice
MAINSERVICEIMAGE = watermelon-mainservice
MAINSERVICEPROXYIMAGE = watermelon-mainserviceproxy
CLICKHOUSEIMAGE = clickhouse-exposed

CLIENTPATH = ./cmd/client
CLIENTEXEC = ./cmd/client/client

build: build_email_service build_main_service build_main_service_proxy

build_email_service:
	go build -o $(EMAILSERVICEEXEC) $(EMAILSERVICEPATH) 

build_main_service:
	go build -o $(MAINSERVICEEXEC) $(MAINSERVICEPATH) 

build_main_service_proxy:
	go build -o $(MAINSERVICEPROXYEXEC) $(MAINSERVICEPROXYPATH) 

container_build: container_build_email_service container_build_main_service container_build_main_service_proxy

container_build_email_service:
	$(GOENVS) go build $(GOFLAGS) -o $(EMAILSERVICEEXEC) $(EMAILSERVICEPATH)

container_build_main_service:
	$(GOENVS) go build $(GOFLAGS) -o $(MAINSERVICEEXEC) $(MAINSERVICEPATH)

container_build_main_service_proxy:
	$(GOENVS) go build $(GOFLAGS) -o $(MAINSERVICEPROXYEXEC) $(MAINSERVICEPROXYPATH) 

create_images: container_build
	docker build --rm -t $(EMAILSERVICEIMAGE) $(EMAILSERVICEPATH)
	docker build --rm -t $(MAINSERVICEIMAGE) $(MAINSERVICEPATH)
	docker build --rm -t $(MAINSERVICEPROXYIMAGE) $(MAINSERVICEPROXYPATH)	
	docker build --rm -t $(CLICKHOUSEIMAGE) $(CLICKHOUSEPATH)

test:
	go test -cpu 1,4 -race ./cmd/... ./internal/...

fmt:
	go fmt ./cmd/... ./internal/...

clean_images:
	docker rmi $$(docker images --filter "dangling=true" -q --no-trunc)

clean_executables:
	rm -rf $(EMAILSERVICEEXEC) $(MAINSERVICEEXEC) $(MAINSERVICEPROXYEXEC) $(CLIENTEXEC)

clean: clean_executables clean_images

containers_up:
	GWM_DELIVERY_TIME="" GWM_DELIVERY_INTERVAL="" GWM_HOST_EXTERNAL_IP=$$(curl ifconfig.me) docker-compose up

containers_down:
	docker-compose down

build_client:
	go build -o $(CLIENTEXEC) $(CLIENTPATH) 

rebuild: clean build
