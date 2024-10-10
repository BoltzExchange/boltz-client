# GDK

The gdk build process is split in two parts: the builder image inside the gdk repo and the `gdk.Dockerfile` located in this repo.
The builder image contains all the prebuilt dependencies and the `gdk.Dockerfile` uses it to build the static and dynamic version of the library
and then publish just those 2 artifacts, allowing for fast downloads.

To update GDK, bump the version in the `Makefile` and run `make build-gdk-builder`.
This will build and push the builder image to the registry.
Then run `make build-gdk` to build the actual artifacts.
