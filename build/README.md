# `/build`

All generated binary, cmake, make files go there

## Build image
```bash
# build image
docker build -t this-project -f ./build/Dockerfile .

# run container (interactive)
docker run -it this-project

# enter a running container with sh
docker exec -it this-project_1 sh
```
