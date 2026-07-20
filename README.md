# The Chunk Explorer

The vision of this project is to create a space where anyone can host their Minecraft creations easily and make them discoverable for other people.
Currently, the focus is only on minigames, but future releases will extend the system for all types of experiences. Due to utilizing container checkpointing, 
which enables us to eliminate the heavy startup cost of Minecraft servers, the server bill is substantially minimized, because now it's possible to run servers
on limited hardware like small cloud VMs.

**Note**: This is very much still a work in progress project, so technical aspects will possibly change quite often and break things.

## What is the current state of the project?

The project is currently in an alpha state. The basic functions are implemented and working™. You can try it out by opening Minecraft (Java Edition) and joining chunks.space.
The next steps are fixing bugs, that come up and investing time into stability and observability. 

Here are some examples of things, that are still missing:
* Configurable user limits
* Feature flags
* Improvements on the platformd side (component responsible for actually running the containers)
  * Image GC
  * TBD
* TBD

If there is a concrete roadmap it will be posted on our [Discord](http://discord.chunks.space/).

**Requirements**
* Linux kernel >= 6.6, tcx not supported (caused by `link.AttachTCX`)

**Limitations**
* Only single-platform OCI images can be built, so setups containing both `linux/arm64` and `linux/amd64` hosts is not possible as of now.

## License
This project uses two different licenses: AGPLv3 and LGPLv3. Everything found under under the `api/` folder is licensed under LGPLv3 while everything else is covered by AGPLv3, if not stated otherwise.


