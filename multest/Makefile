SHELL=/bin/bash

include .env

default:
	@echo


deploy:	
	aws s3 cp index.html $(S3_TARGET)
	aws cloudfront create-invalidation --distribution-id $(CDN_ID) --path "/mnozenie/*"