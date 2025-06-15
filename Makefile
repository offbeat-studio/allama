.PHONY: build up down clean

build:
	docker-compose -f Docker/docker-compose.yml build

up:
	docker-compose -f Docker/docker-compose.yml up -d

down:
	docker-compose -f Docker/docker-compose.yml down

clean:
	docker-compose -f Docker/docker-compose.yml down --volumes --rmi all
