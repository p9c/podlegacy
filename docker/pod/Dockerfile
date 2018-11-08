# STEP 1 build executable binary

FROM golang:alpine as builder

# Install git
RUN apk update && apk add git 

COPY . $GOPATH/src/github.com/parallelcointeam/pod
WORKDIR $GOPATH/src/github.com/parallelcointeam/pod

RUN pwd;ls -l

#get dependancies
#you can also use dep
RUN go get -d -v /cmd/pod

#build the binary
RUN go build -o github.com/parallelcointeam/pod/cmd/pod/main.go

# STEP 2 build a small image

# start from scratch
FROM scratch

# Copy our static executable
COPY --from=builder /go/bin/hello /go/bin/hello
ENTRYPOINT ["/go/bin/hello"]
