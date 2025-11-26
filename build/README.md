# `/build`

All generated binary, cmake, make files go there

## Build image
```bash
# build image
docker build -t expense-tracker -f ./build/Dockerfile .

# run container (interactive)
docker run -it expense-tracker

# enter a running container with sh
docker exec -it expense-tracker_1 sh
```
