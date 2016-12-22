# Readium-2 Streamer in Go

This project is based on the [Readium-2 Streamer architecture](https://github.com/readium/readium-2/blob/master/streamer/README.md) that basically takes an EPUB as an input and exposes in HTTP:

- a [Web Publication Manifest](https://github.com/HadrienGardeur/webpub-manifest) based on the OPF of the original EPUB
- resources from the container (HTML, CSS, images etc.)

It is entirely written in Go using [Negroni](https://github.com/urfave/negroni). 

This project is broken down in multiple Go packages that can be used independently from the project:

- `models` is an in-memory model of a publication and its components
- `parser` is responsible for parsing an EPUB and converting that info to the in-memory model
- `fetcher` is meant to access resources contained in an EPUB and pre-process them (decryption, deobfuscation, content injection)

##Usage

The `server` binary can be called using a single argument: the location to an EPUB file.

The server will bind itself to an available port on `localhost` and return a URI pointing to the Web Publication Manifest.

##Need a Binary Version? 

Releases for stable versions are available at: https://github.com/Feedbooks/webpub-streamer/releases

##Live Demo

A live demo is available at: https://proto.myopds.com/
