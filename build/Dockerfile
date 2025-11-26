FROM golang:1.25.3-alpine AS stub
# Copy go.mod and go.sum files (and only them)
# First - copy all the needed files
WORKDIR /project-stub-dir
ADD . .
# Here we delete all the files that are not "go.mod" or "go.sum"
# RUN find . ! -regex  '.*\(go.mod\|go.sum\)$' -type f | xargs rm
RUN find . -type f ! -name 'go.mod' ! -name 'go.sum' -print0 | xargs -0 rm -f

FROM golang:1.25.3-alpine AS build
# Cache dependencies and build in a clean image to make use of cache.
# We can do it because we move files to clean image. This way even when 
# you change one of non-go.mod/sum files the resulting layer is the same,
# as we deleted this file in the previous image.
RUN apk add build-base
COPY --from=stub /project-stub-dir /project-build-dir
WORKDIR /project-build-dir
# This command will use cache
RUN go mod download
ADD . .
# An this one will not, as we copied all the files again
# Next we build all the binaries we need in this docker image
RUN go build -o ./build/my-entrypoint-binary src/cmd/my-entrypoint/main.go

FROM alpine:latest AS runtime
# copy binary to the clean image
RUN apk add --no-cache tzdata
ENV TZ="America/Bogota"
WORKDIR /project-run-dir
COPY --from=build /project-build-dir/build/my-entrypoint-binary ./my-entrypoint-binary

# ENTRYPOINT [ "./my-entrypoint-binary" ]
# keeping this one for image debugging purposes
ENTRYPOINT ["sh"]
