# Web Publication Viewer

This project is a proof of concept for handling Web Publications and the [Web Publication Manifest](https://github.com/HadrienGardeur/webpub-manifest) in a browser.

The viewer is a simple Web App that does the following things:

- display the publication in an iframe
- cache the resources listed in a Web Publication Manifest for offline viewing and serve them with a Service Worker
- provide the ability to navigate between resources in the publication

For the progressive enhancements use case, check [Web Publication JS](https://github.com/HadrienGardeur/webpub-js).

##Usage

By default the viewer will use the following Web Publication Manifest: https://hadriengardeur.github.io/webpub-manifest/examples/MobyDick/manifest.json

To override this behavior, use the following query parameters:

- "manifest" set to "true"
- "href" set to the location of the Web Publication Manifest that you'd like to display

Check the live demo for an example.

##Live Demo

A live demo of the viewer is available at: https://hadriengardeur.github.io/webpub-manifest/examples/viewer

To display another example, you can also use query parameters for example: https://hadriengardeur.github.io/webpub-manifest/examples/viewer/?manifest=true&href=https://hadriengardeur.github.io/webpub-manifest/examples/comics/manifest.json
