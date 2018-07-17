# spark-ui-proxy

## build
go build -o spark-ui-proxy

## run
SPARK_MASTER_ADDR=127.0.0.1:8080; ./spark-ui-proxy $SPARK_MASTER_ADDR
