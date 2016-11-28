# Web Publication Streamer

This project is a proof of concept that takes an EPUB as an input and provides the following resources in HTTP for it:

- a [Web Publication Manifest](https://github.com/HadrienGardeur/webpub-manifest) based on the OPF of the original EPUB
- resources from the EPUB (HTML, CSS, images etc.)

It is entirely written in Go using [Negroni](https://github.com/urfave/negroni). The server is automatically binded to HTTPS using an automatic Let's Encrypt certificate.

In addition to streaming an EPUB as a Web Publication, this project also embeds the [Web Publication Viewer](https://github.com/HadrienGardeur/webpub-viewer) in order to read such publications.

##Usage

By default the server will use EPUB files included in the books directory.

For local usage or dev use the parameters dev to bind the server in http mode on port 8080

##Live Demo

A live demo is available at: https://proto.myopds.com/
