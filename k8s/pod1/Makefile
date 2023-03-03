

all:

apply:
	kubectl apply -f pod.yaml

proxy:
	kubectl port-forward pod/nginx 8000:80

get:
	curl http://localhost:8000
