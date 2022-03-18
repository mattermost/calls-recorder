.PHONY: build
build:
	CGO_ENABLED=0 go build main.go

.PHONY: docker
docker: build
	docker build -t streamer45/calls-recorder .

.PHONY: run
run:
	@docker run --name calls-recorder \
		-e SITE_URL=${SITE_URL} \
		-e USERNAME=${USERNAME} \
		-e PASSWORD=$(value PASSWORD) \
		-e TEAM_NAME=${TEAM_NAME} \
		-e CHANNEL_ID=${CHANNEL_ID} \
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

