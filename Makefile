all: build push

build:
	docker build -t brandonlbarrow/jagger .

push:
	docker push brandonlbarrow/jagger