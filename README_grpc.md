# GRPC

# Building the GRPC RIS Bindings for Python
```
# create a virtual env
....

# install dependencies
pip install grpcio-tools

# symlinks, so the imports inside the .proto files work
mkdir -p github.com/bio-routing/
ln -s ../../ github.com/bio-routing/bio-rd

# generate bindings (from git repo root)
mkdir -p pyproto/github/com/bio_routing/bio_rd/net/api pyproto/github/com/bio_routing/bio_rd/route/api
python -m grpc_tools.protoc -I. -I net/api/ --python_out pyproto/github/com/bio_routing/bio_rd/net/api/ net.proto
python -m grpc_tools.protoc -I. -I route/api/ --python_out pyproto/github/com/bio_routing/bio_rd/route/api/ route.proto
python -m grpc_tools.protoc -I. -I cmd/ris/api/ --python_out pyproto/ --grpc_python_out pyproto/ ris.proto
```

In some cases the bindings might error out on import with something like this:
```
TypeError: Couldn't build proto file into descriptor pool!
```
Then this might help:
```
pip uninstall protobuf
pip install --no-binary=protobuf protobuf
```

