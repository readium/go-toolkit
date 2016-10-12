/* Simple prototype for a Web App for Web Publications based on an iframe. */

(function() {

  if (navigator.serviceWorker) {
    //HINT: Make sure that the path to your Service Worker is correct
    navigator.serviceWorker.register('sw.js');
  
    navigator.serviceWorker.ready.then(function() {
      console.log('SW ready');
    }); 
  };
  
  var DEFAULT_MANIFEST = "https://hadriengardeur.github.io/webpub-manifest/examples/MobyDick/manifest.json";
  var current_url_params = new URLSearchParams(location.href);

  if (current_url_params.has("href")) {
    console.log("Found manifest in params")
    var manifest_url = current_url_params.get("href");
  } else {
    var manifest_url = DEFAULT_MANIFEST;
  };

  if (current_url_params.has("document")) {
    console.log("Found reference to a document in params")
    var document_url = current_url_params.get("href");
  } else {
    var document_url = undefined;
  };

  verifyAndCacheManifest(manifest_url).catch(function() {});
  initializeNavigation(manifest_url).catch(function() {});
  
  var iframe = document.querySelector("iframe");
  var next = document.querySelector("a[rel=next]");
  var previous = document.querySelector("a[rel=prev]");
  var navigation = document.querySelector("div[class=controls]");

  iframe.style.height = document.body.scrollHeight - navigation.scrollHeight - 5 + 'px';
  iframe.style.marginTop = navigation.scrollHeight + 'px';

  next.addEventListener("click", function(event) {
    if (next.hasAttribute("href")) {
      iframe.src = next.href;
      iframe.style.height = document.body.scrollHeight - navigation.scrollHeight - 5 + 'px';
      history.pushState(null, null, "./?manifest=true&href="+manifest_url+"&document="+next.href);
      updateNavigation(manifest_url).catch(function() {});
    };
    event.preventDefault();
  });

  previous.addEventListener("click", function(event) {
    if ( previous.hasAttribute("href")) {
      iframe.src = previous.href;
      iframe.style.height = document.body.scrollHeight - navigation.scrollHeight - 5 + 'px';
      history.pushState(null, null, "./?manifest=true&href="+manifest_url+"&document="+previous.href);
      updateNavigation(manifest_url).catch(function() {});
    };
    event.preventDefault();
  });

  function getManifest(url) {
    return fetch(url).then(function(response) {
      return response.json();
    })
  };

  function verifyAndCacheManifest(url) {
    return caches.open(url).then(function(cache) {
      return cache.match(url).then(function(response){
        if (!response) {
          console.log("No cache key found");
          console.log('Caching manifest at: '+url);
          return cacheManifest(url);
        } else {
          console.log("Found cache key");
        };
      })
    });
  };
  
  function cacheURL(data, manifest_url) {
    return caches.open(manifest_url).then(function(cache) {
      return cache.addAll(data.map(function(url) {
        console.log("Caching "+url);
        return new URL(url, manifest_url);
      }));
    });
  };

  function cacheManifest(url) {
    var manifestJSON = getManifest(url);
    return Promise.all([cacheSpine(manifestJSON, url), cacheResources(manifestJSON, url)])
  };

  function cacheSpine(manifestJSON, url) {
    return manifestJSON.then(function(manifest) {
      return manifest.spine.map(function(el) { return el.href});}).then(function(data) {
        data.push(url);
        return cacheURL(data, url);})
  };

  function cacheResources(manifestJSON, url) {
    return manifestJSON.then(function(manifest) {
      return manifest.resources.map(function(el) { return el.href});}).then(function(data) {return cacheURL(data, url);})
  };

  function initializeNavigation(url) {
    return getManifest(url).then(function(json) { 
      var title = json.metadata.title;
      console.log("Title of the publication: "+title);
      document.querySelector("title").textContent = title;
      return json.spine;
    }).then(function(spine) {
      
      //Find iframe and set start document
      var iframe = document.querySelector("iframe");
      var start_url = new URL(spine[0].href, url).href;
      console.log("Set iframe to: "+start_url)
      iframe.src = start_url;
      
      var start = document.querySelector("a[rel=start]");
      var next = document.querySelector("a[rel=next]");
      var previous = document.querySelector("a[rel=prev]");
      var navigation = document.querySelector("div[class=controls]");

      //Set start action
      start.href = start_url; 
      start.addEventListener("click", function(event) {
        iframe.src = start.href;
        iframe.style.height = document.body.scrollHeight - navigation.scrollHeight - 5 + 'px';
        next.href = new URL(spine[1].href, url).href;
        previous.removeAttribute("href");
        history.pushState(null, null, "./?manifest=true&href="+url+"&document="+start.href);
        event.preventDefault();
      });

      //Set next button
      console.log("Next document is: "+spine[1].href);
      next.href = new URL(spine[1].href, url).href;

    });
  };

  function updateNavigation(url) {
    console.log("Getting "+url)
    return getManifest(url).then(function(json) { return json.spine} ).then(function(spine) {
      var iframe = document.querySelector("iframe");
      var start = document.querySelector("a[rel=start]");
      var next = document.querySelector("a[rel=next]");
      
      var current_index = spine.findIndex(function(element) {
        var element_url = new URL(element.href, url);
        return element_url.href == iframe.src
      })
      
      if (current_index >= 0) {

        if (current_index > 0) {
          console.log("Previous document is: "+spine[current_index - 1].href);
          previous.href = new URL(spine[current_index - 1].href, url).href;
        } else {
          previous.removeAttribute("href");
        };
        
        if (current_index < (spine.length-1)) {
          console.log("Next document is: "+spine[current_index + 1].href);
          next.href = new URL(spine[current_index + 1].href, url).href;
        } else {
          next.removeAttribute("href");
        };
      }
    });
  };

}());