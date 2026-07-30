SHELL=/bin/bash

all: 
	echo


IMAGE=n1hello
VERSION=1

REGION=eu-central-1
ACCOUNT=184652322412

ECR_DOMAIN=$(ACCOUNT).dkr.ecr.$(REGION).amazonaws.com

ECR_TAG=$(ACCOUNT).dkr.ecr.$(REGION).amazonaws.com/$(IMAGE):$(VERSION)
ROLE_ARN=arn:aws:iam::184652322412:role/Server1StartStop

build:
	docker build --platform linux/amd64 -t $(IMAGE):$(VERSION) .

ecr-login:
	aws ecr get-login-password --region $(REGION)  | docker login --username AWS --password-stdin $(ECR_DOMAIN)

create-repository:
	aws ecr create-repository --repository-name $(IMAGE) --region $(REGION) --image-scanning-configuration scanOnPush=true --image-tag-mutability MUTABLE

tag:
	docker tag  $(IMAGE):$(VERSION)  $(ECR_TAG)

push: 
	docker push $(ECR_TAG)


create-function:
	aws lambda create-function \
  		--function-name $(IMAGE) \
  		--package-type Image \
  		--code ImageUri=$(ECR_TAG) \
		--region $(REGION) \
  		--role $(ROLE_ARN)



invoke-function:
	aws lambda invoke --region $(REGION) --function-name $(IMAGE) response.json

