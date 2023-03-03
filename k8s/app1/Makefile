

all:
	echo 

build:
	GOOS=linux GOARCH=amd64 go build -o sampleapp1

docker:
	docker build -t sampleapp:1 . 

run:
	docker run -p 8080:8080 sampleapp:1

# https://kubernetes.github.io/ingress-nginx/deploy/
install-ingress-controller:
	kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.5.1/deploy/static/provider/cloud/deploy.yaml


deploy:
	kubectl apply -f deployment.yaml

	#https://kubernetes.io/docs/concepts/services-networking/service/
	kubectl apply -f service.yaml

	#https://kubernetes.io/docs/concepts/services-networking/ingress/
	kubectl apply -f ingress.yaml

