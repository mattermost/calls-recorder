.PHONY: build
build:
	CGO_ENABLED=0 go build main.go

.PHONY: docker
docker: build
	docker build -t streamer45/calls-recorder .

.PHONY: run
run:
	@docker run --name calls-recorder \
		-e MM_SITE_URL=${MM_SITE_URL} \
		-e MM_USERNAME=${MM_USERNAME} \
		-e MM_PASSWORD=$(value MM_PASSWORD) \
		-e MM_TEAM_NAME=${MM_TEAM_NAME} \
		-e MM_CHANNEL_ID=${MM_CHANNEL_ID} \
		-v calls-recorder-volume:/recs streamer45/calls-recorder

.PHONY: stop
stop:
	docker stop calls-recorder

.PHONY: clean
clean:
	docker kill calls-recorder; docker rm calls-recorder

.PHONY: lint
lint:
	golangci-lint run ./...

