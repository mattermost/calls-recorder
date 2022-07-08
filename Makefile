.PHONY: build
build:
	CGO_ENABLED=0 go build cmd/recorder/main.go cmd/recorder/upload.go

.PHONY: build-service
build-service:
	CGO_ENABLE=0 go build cmd/service/service.go

.PHONY: docker
docker: build
	docker build -t streamer45/calls-recorder .

.PHONY: run
run:

ifeq ($(shell test -f .config.env && printf "yes"),yes)
	@docker run --name calls-recorder \
		--env-file .config.env \
		-v calls-recorder-volume:/recs streamer45/calls-recorder
else
	@docker run --name calls-recorder \
		-e MM_SITE_URL=${MM_SITE_URL} \
		-e MM_USERNAME=${MM_USERNAME} \
		-e MM_PASSWORD=$(value MM_PASSWORD) \
		-e MM_TEAM_NAME=${MM_TEAM_NAME} \
		-e MM_CHANNEL_ID=${MM_CHANNEL_ID} \
		-v calls-recorder-volume:/recs streamer45/calls-recorder
endif


.PHONY: stop
stop:
	docker stop -t 300 calls-recorder

.PHONY: clean
clean:
	docker kill calls-recorder; docker rm calls-recorder

.PHONY: lint
lint:
	golangci-lint run ./...

